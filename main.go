package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const appName = "Niconico Desktop MyGo"

// Set by the release workflow. It does not change the WebView2 profile name.
var version = "dev"

type browser struct {
	win                          *mygo.Window
	web                          *mygo.WebView
	field, source, title, status string
	libraryPath, storeWarning    string
	commentLanguageWarning       bool
	data                         library
	history, loaded, closed      bool
	noHistory                    string
	session                      *mygo.Session
	official, openingOfficial    bool
	officialTarget               string
	catalog                      catalog
	navigation                   []screenRoute
	index                        int
	result                       catalogPage
	resultErr                    error
	busy                         bool
	cancel                       context.CancelFunc
	epoch                        uint64
	player                       *playerWindow
}

func main() {
	mygo.App.SetName(appName)
	b := &browser{index: -1}
	initial := home
	if len(os.Args) > 1 && allowedURL(os.Args[1]) != nil {
		initial = os.Args[1]
	}
	mygo.App.WhenReady(func() { b.start(initial) })
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (b *browser) start(initial string) {
	if dir, err := os.UserConfigDir(); err == nil {
		b.libraryPath = filepath.Join(dir, "NiconicoDesktopMyGo", "library.json")
		b.data, err = loadLibrary(b.libraryPath)
		if err != nil {
			b.storeWarning = "既存の保存ファイルを読み込めません。上書きは行いません。"
		}
	} else {
		b.storeWarning = "保存先を取得できません。"
	}
	b.win = mygo.NewWindow(mygo.WindowOptions{
		Title: appName + " " + version, Width: 1260, Height: 850, MinWidth: 1000, MinHeight: 600,
		Content: ui.View(b.draw), Page: mygo.PageOptions{DevTools: mygo.DevToolsDisabled},
	})
	b.session = mygo.NewSession("niconico-desktop-mygo")
	b.catalog = newCatalog(func(target string) (*http.Cookie, error) {
		return readSessionCookie(b.session.Cookies.List, target, time.Now())
	})
	b.win.OnClosed(func() {
		b.closed = true
		b.cancelLoad()
		if play := b.player; play != nil {
			play.win.Close()
		}
	})
	b.openNative(routeFor(home), true)
	if initial != home {
		b.navigate(initial)
	}
}

func (b *browser) attachOfficial() {
	if b.closed || !b.official || b.web != nil || b.openingOfficial {
		return
	}
	b.openingOfficial = true
	defer func() { b.openingOfficial = false }()
	b.commentLanguageWarning = setJapaneseComments(b.session.Cookies.Set, time.Now()) != nil
	if b.closed || !b.official {
		return
	}
	view, err := b.win.NewWebView(mygo.WebViewOptions{
		Page: mygo.PageOptions{Untrusted: true, DevTools: mygo.DevToolsDisabled, Session: b.session},
	})
	if err != nil {
		b.status = "WebView2を起動できません。Microsoft Edge WebView2 Runtimeを確認してください。"
		b.win.Invalidate()
		return
	}
	if b.closed || !b.official {
		view.Destroy()
		return
	}
	b.web = view
	p := view.Page()
	active := func() bool { return !b.closed && b.official && b.web == view }
	p.OnWillNavigate(func(e *mygo.NavigateEvent) {
		if !active() {
			e.PreventDefault()
			return
		}
		if allowedURL(e.URL) == nil {
			e.PreventDefault()
			b.status = "ニコニコ以外への移動をブロックしました。"
		} else if e.UserInitiated && b.redirectFromPlayer(e.URL) {
			e.PreventDefault()
		} else {
			b.loading()
		}
		b.win.Invalidate()
	})
	p.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		if !active() {
			return nil
		}
		u := allowedURL(req.URL)
		if u != nil && (req.UserInitiated || strings.EqualFold(u.Hostname(), "account.nicovideo.jp")) {
			target := u.String()
			if req.UserInitiated && b.redirectFromPlayer(target) {
				return nil
			}
			// Do not re-enter the WebView2 callback to navigate.
			go mygo.RunOnMain(func() {
				if active() {
					b.loadOfficial(target)
				}
			})
		} else {
			b.status = "アプリ外の新規ウィンドウは開きません。"
			b.win.Invalidate()
		}
		return nil
	})
	p.OnDidNavigate(func(_ string) {
		if !active() {
			return
		}
		b.loading()
		b.win.Invalidate()
	})
	p.OnPageTitleUpdated(func(e *mygo.TitleEvent) {
		e.PreventDefault()
		if !active() {
			return
		}
		b.title = e.Title
		if b.loaded {
			b.recordVideo()
		}
		b.win.Invalidate()
	})
	p.OnDidFinishLoad(func() {
		if !active() {
			return
		}
		raw := p.URL()
		b.source, b.loaded = raw, allowedURL(raw) != nil
		b.status = "ニコニコ公式ページ"
		if !b.loaded {
			b.status = "対応するページを開いてください。"
		}
		b.recordVideo()
		b.win.Invalidate()
	})
	p.OnDidFailLoad(func(_ *mygo.LoadError) {
		if !active() {
			return
		}
		b.loaded = false
		b.status = "ページを読み込めませんでした。接続を確認してください。"
		b.win.Invalidate()
	})
	p.OnRenderProcessGone(func(_ string) {
		if !active() {
			return
		}
		b.loaded = false
		b.status = "表示が終了しました。「更新」で再読み込みできます。"
		b.win.Invalidate()
	})
	b.loadOfficial(b.officialTarget)
	// pushState and hash changes do not always emit a new-document event.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			keep := false
			mygo.RunOnMain(func() {
				keep = active()
				if !keep || !b.loaded {
					return
				}
				if raw := p.URL(); raw != b.source {
					b.source = raw
					b.recordVideo()
					b.win.Invalidate()
				}
			})
			if !keep {
				return
			}
		}
	}()
}

func (b *browser) loading() {
	b.loaded = false
	b.status = "読み込み中…"
	b.title = ""
	b.noHistory = ""
}

func (b *browser) loadOfficial(target string) {
	if b.web == nil || allowedURL(target) == nil {
		return
	}
	b.loading()
	if err := b.web.Page().LoadURL(target); err != nil {
		b.status = "ページを開けませんでした。"
	}
	b.win.Invalidate()
}

func (b *browser) search(byTag bool) {
	if target := resolve(b.field, byTag); target != "" {
		b.navigate(target)
	} else {
		b.status = "検索語・タグ・動画ID、またはニコニコ公式のHTTPS URLを入力してください。"
	}
}

func (b *browser) current() string {
	if b.official {
		if b.loaded && b.web != nil {
			return b.source
		}
		return ""
	}
	if b.index < 0 {
		return ""
	}
	return b.navigation[b.index].link
}

func (b *browser) currentTitle() string {
	if b.official {
		return b.title
	}
	return b.result.title
}

func (b *browser) persist() bool {
	if b.storeWarning != "" {
		return false
	}
	if err := saveLibrary(b.libraryPath, b.data); err != nil {
		b.status = "保存に失敗しました。操作はこのセッションにのみ反映されます。"
		return false
	}
	return true
}

func (b *browser) recordVideo() {
	b.recordVideoURL(b.current(), b.currentTitle())
}

func (b *browser) recordVideoURL(raw, title string) {
	e, ok := makeEntry(raw, title)
	if !ok || e.Kind != "動画" || e.URL == b.noHistory || len(b.data.History) > 0 && b.data.History[0] == e {
		return
	}
	b.data.History = putFirst(b.data.History, e, 100)
	b.persist()
}

func (b *browser) pinCurrent() {
	e, ok := makeEntry(b.current(), b.currentTitle())
	if !ok {
		return
	}
	if b.storeWarning != "" {
		b.status = b.storeWarning
		return
	}
	if len(b.data.Pins) >= 500 || slices.ContainsFunc(b.data.Pins, func(item entry) bool { return item.URL == e.URL }) {
		b.status = "登録済み、または登録件数が上限に達しています。"
		return
	}
	b.data.Pins = putFirst(b.data.Pins, e, 500)
	b.history = false
	if b.persist() {
		b.status = e.Kind + "を登録しました。"
	}
}

func (b *browser) remove(e entry) {
	if b.storeWarning != "" {
		b.status = b.storeWarning
		return
	}
	items := &b.data.Pins
	if b.history {
		items = &b.data.History
		if u, _ := canonical(b.current()); u == e.URL {
			b.noHistory = u
		}
	}
	*items = slices.DeleteFunc(*items, func(x entry) bool { return x.URL == e.URL })
	if b.persist() {
		b.status = "選択した項目を削除しました。"
	}
}

func (b *browser) copyCurrent() {
	target := goNicoListURL(b.current())
	if target == "" {
		return
	}
	if err := mygo.Clipboard.Write(transfer.TextData(target)); err != nil {
		b.status = "クリップボードにコピーできませんでした。"
	} else {
		b.status = "go-nico-list用のURLをコピーしました。"
	}
}

func (b *browser) draw(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		b.toolbar(c)
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			b.sidebar(c)
			if !b.official {
				b.drawNative(c)
			} else if b.web != nil {
				ui.WebView(c, b.web).Grow(1).MinWidth(0)
			} else {
				ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, b.status).Wrap()
				})
			}
		})
		ui.Row(c).FillWidth().Padding(5, 12).Gap(12).Children(func() {
			ui.Text(c, b.status).TextColor(t.TextMuted)
			if b.official && b.commentLanguageWarning {
				ui.Text(c, "日本語コメントの設定に失敗しました。アプリを再起動してください。").TextColor(t.TextMuted)
			}
			if b.storeWarning != "" {
				ui.Text(c, b.storeWarning).TextColor(t.TextMuted)
			}
		})
	})
}

func (b *browser) toolbar(c *ui.Context) {
	var p *mygo.Page
	if b.official && b.web != nil {
		p = b.web.Page()
	}
	ui.Row(c).FillWidth().Gap(6).Padding(8, 10).AlignItems(ui.Center).Children(func() {
		back := b.official || b.index > 0
		forward := p != nil && p.CanGoForward() || !b.official && b.index+1 < len(b.navigation)
		if ui.Button(c, "‹").Label("戻る").Disabled(!back).Clicked() || back && c.Shortcut(ui.Alt, ui.KeyLeft) {
			if p != nil && p.CanGoBack() {
				p.GoBack()
			} else if b.official {
				b.leaveOfficial()
			} else {
				b.moveNative(-1)
			}
		}
		if ui.Button(c, "›").Label("進む").Disabled(!forward).Clicked() || forward && c.Shortcut(ui.Alt, ui.KeyRight) {
			if p != nil {
				p.GoForward()
			} else {
				b.moveNative(1)
			}
		}
		if ui.Button(c, "更新").Clicked() || c.Shortcut(0, ui.KeyF5) {
			if p != nil {
				b.loading()
				p.Reload()
			} else if b.official {
				go mygo.RunOnMain(b.attachOfficial)
			} else {
				b.refreshNative()
			}
		}
		if ui.Button(c, "ホーム").Clicked() {
			b.navigate(home)
		}
		if b.official && ui.Button(c, "独自UIに戻る").Clicked() {
			b.leaveOfficial()
		}
		input := ui.TextInput(c, &b.field).Label("キーワード、タグ、動画IDまたはURL").Placeholder("キーワード・タグ・URL").Grow(1).MinWidth(120)
		if c.Shortcut(ui.Ctrl, ui.KeyL) {
			input.Focus()
		}
		if ui.PrimaryButton(c, "検索").Clicked() || input.Submitted() {
			b.search(false)
		}
		if ui.Button(c, "タグ検索").Clicked() {
			b.search(true)
		}
		saved, _ := canonical(b.current())
		if ui.Button(c, "☆ 登録").Disabled(saved == "").Clicked() {
			b.pinCurrent()
		}
		if ui.Button(c, "URLをコピー").Tooltip("go-nico-list用のURLをコピー").Disabled(goNicoListURL(b.current()) == "").Clicked() {
			b.copyCurrent()
		}
		if ui.Button(c, "マイリスト").Clicked() {
			b.navigate(mylists)
		}
		if ui.Button(c, "ログイン").Clicked() {
			b.navigate(login)
		}
	})
}

func (b *browser) sidebar(c *ui.Context) {
	ui.Column(c).Width(218).Padding(10).Gap(8).Children(func() {
		ui.Text(c, "NICONICO DESKTOP").Bold()
		ui.Row(c).Gap(5).Children(func() {
			if ui.Button(c, "登録済み").Disabled(!b.history).Clicked() {
				b.history = false
			}
			if ui.Button(c, "視聴履歴").Disabled(b.history).Clicked() {
				b.history = true
			}
		})
		items := b.data.Pins
		if b.history {
			items = b.data.History
		}
		if len(items) == 0 {
			ui.Text(c, "表示する項目はありません。").Wrap()
			return
		}
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, item := range items {
					row := c.Key(item.URL)
					ui.Row(row).Gap(4).AlignItems(ui.Center).Children(func() {
						if ui.Button(row, "["+item.Kind+"] "+item.Title).Tooltip(item.Title).Grow(1).MinWidth(0).Clicked() {
							b.navigate(item.URL)
						}
						if ui.Button(row, "×").Label("削除").Clicked() {
							b.remove(item)
							return
						}
					})
				}
			})
		})
	})
}
