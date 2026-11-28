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
	source, title, status        string
	searchInput                  searchInput
	libraryPath, storeWarning    string
	commentLanguageWarning       bool
	data                         library
	pendingLibraryEdits          []func(*library) bool
	history, loaded, closed      bool
	showLibrary                  bool
	layoutPath                   string
	layoutLoadErr                error
	focusSearch                  bool
	closeHelpPath                string
	closeHelpAcknowledged        bool
	session                      *mygo.Session
	loginState                   loginState
	loginBusy                    bool
	loginCheck                   uint64
	loginCancel, stopLoginWatch  context.CancelFunc
	official, openingOfficial    bool
	loginHelp                    bool
	officialTarget               string
	catalog                      catalog
	thumbnails                   *thumbnails
	navigation                   []screenRoute
	nativeVisits                 []*catalogVisit
	index                        int
	userNames                    map[string]string
	result                       catalogPage
	resultErr                    error
	busy                         bool
	cancel                       context.CancelFunc
	epoch                        uint64
	player                       *playerWindow
	playbackRequest              uint64
	playMode                     playbackMode
	showPlaybackSettings         bool
	playbackPath, playbackNotice string
	commentNG                    commentNG
	nativeVolume                 float64
	nativeDisplay                nativeDisplayPreferences
	nativeDisplayNotice          string
	nativeMuted                  bool
	nativeAudioNotice            string

	searchFocus, libraryFocus, contentFocus ui.Handle
	libraryRegion, contentRegion            ui.Handle

	searchSortPath, searchSortNotice string
	searchDefault                    searchSortPreference
}

func main() {
	mygo.App.SetName(appName)
	b := &browser{index: -1, showLibrary: true}
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
	b.playMode = webPlayback
	b.nativeVolume = defaultNativeVolume
	if dir, err := os.UserConfigDir(); err == nil {
		b.playbackPath = filepath.Join(dir, "NiconicoDesktopMyGo", "playback.json")
		if prefs, err := loadNativeDisplay(b.nativeDisplayPath()); err == nil {
			b.nativeDisplay = prefs
		} else {
			b.nativeDisplayNotice = "表示設定を読み込めませんでした。"
		}
		b.layoutPath = filepath.Join(dir, "NiconicoDesktopMyGo", "layout.json")
		b.showLibrary, b.layoutLoadErr = loadSidebarVisibility(b.layoutPath)
		if audio, err := loadNativeAudio(b.nativeAudioPath()); err == nil {
			b.nativeVolume, b.nativeMuted = audio.Volume, audio.Muted
		} else {
			b.nativeAudioNotice = "音量・ミュート設定を読み込めませんでした。"
		}
		b.closeHelpPath = filepath.Join(dir, "NiconicoDesktopMyGo", "close-help.json")
		b.closeHelpAcknowledged = loadCloseHelp(b.closeHelpPath)
		b.searchSortPath = filepath.Join(dir, "NiconicoDesktopMyGo", "search-sort.json")
		b.searchDefault = loadSearchSort(b.searchSortPath)
		if mode, err := loadPlayback(b.playbackPath); err == nil {
			b.playMode = mode
		} else {
			b.playbackNotice = "再生設定を読み込めませんでした。既定の公式プレーヤーを使います。"
		}
		b.libraryPath = filepath.Join(dir, "NiconicoDesktopMyGo", "library.json")
		b.commentNG, _ = loadCommentNG(filepath.Join(dir, "NiconicoDesktopMyGo", "comment-ng.json"))
		b.data, err = loadLibrary(b.libraryPath)
		if err != nil {
			b.storeWarning = "既存の保存ファイルを読み込めません。上書きは行いません。"
		}
	} else {
		b.storeWarning = "保存先を取得できません。"
	}
	b.win = mygo.NewWindow(mygo.WindowOptions{
		StateKey: "main", Title: appName + " " + version, Width: 1260, Height: 850, MinWidth: 640, MinHeight: 520,
		Content: ui.View(b.draw), Page: mygo.PageOptions{DevTools: mygo.DevToolsDisabled},
	})
	_ = b.win.SetIcon(appIconPNG)
	b.session = mygo.NewSession("niconico-desktop-mygo")
	b.catalog = newCatalog(func(target string) (*http.Cookie, error) {
		return readSessionCookie(b.session.Cookies.List, target, time.Now())
	})
	b.thumbnails = newThumbnails(func() {
		if !b.closed {
			b.win.Invalidate()
		}
	})
	b.win.OnClosed(func() {
		b.closed = true
		if b.stopLoginWatch != nil {
			b.stopLoginWatch()
		}
		if b.loginCancel != nil {
			b.loginCancel()
		}
		b.cancelLoad()
		if play := b.player; play != nil {
			play.win.Close()
		}
	})
	b.openNative(routeFor(home), true)
	b.watchLogin()
	b.checkLogin()
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
			if b.officialTarget == login {
				b.status += "外部事業者のログインは利用できません。"
				b.loginHelp = true
			}
		} else if b.playOfficialLink(view, e.URL, false) {
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
			if b.playOfficialLink(view, target, false) {
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
			if b.officialTarget == login {
				b.status += "ニコニコのログイン方法を選んでください。"
				b.loginHelp = true
			}
			b.win.Invalidate()
		}
		return nil
	})
	p.OnDidNavigate(func(raw string) {
		if !active() {
			return
		}
		b.loading()
		b.playOfficialLink(view, raw, true)
		b.win.Invalidate()
	})
	p.OnPageTitleUpdated(func(e *mygo.TitleEvent) {
		e.PreventDefault()
		if !active() {
			return
		}
		b.title = e.Title
		b.win.Invalidate()
	})
	p.OnDidFinishLoad(func() {
		if !active() {
			return
		}
		raw := p.URL()
		if b.playOfficialLink(view, raw, true) {
			return
		}
		b.source, b.loaded = raw, allowedURL(raw) != nil
		b.showSearchRoute(routeFor(raw))
		b.checkLogin()
		b.status = "ニコニコ公式ページ"
		if !b.loaded {
			b.status = "対応するページを開いてください。"
		}
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
	watchWebSource(active, func() {
		raw := p.URL()
		if !b.playOfficialLink(view, raw, true) && b.loaded && raw != b.source {
			b.source = raw
			b.showSearchRoute(routeFor(raw))
			b.win.Invalidate()
		}
	})
}

func (b *browser) loading() {
	b.loaded = false
	b.status = "読み込み中…"
	b.title = ""
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
	return b.navigation[b.index].target
}

func (b *browser) currentTitle() string {
	if b.official {
		return b.title
	}
	title := b.result.title
	if b.index >= 0 && b.navigation[b.index].kind == screenSearch && title != "" {
		r := b.navigation[b.index]
		sort, ascending := r.searchSort()
		order := "降順"
		if ascending {
			order = "昇順"
		}
		if sort.snapshotKey == "" {
			order = "指定順"
		}
		title += fmt.Sprintf("（%s・%s / %dページ）", sort.label, order, r.page+1)
	}
	return title
}

func (b *browser) changeLibrary(edit func(*library) bool) (changed, saved bool) {
	if b.storeWarning != "" {
		return edit(&b.data), false
	}
	var latest library
	err := withSettingsLock(b.libraryPath, func() error {
		var err error
		latest, err = loadLibrary(b.libraryPath)
		if err != nil {
			return err
		}
		dirty := false
		for _, pending := range b.pendingLibraryEdits {
			dirty = pending(&latest) || dirty
		}
		changed = edit(&latest)
		if dirty || changed {
			return saveLibrary(b.libraryPath, latest)
		}
		return nil
	})
	if err != nil {
		changed = edit(&b.data)
		if changed {
			b.pendingLibraryEdits = append(b.pendingLibraryEdits, edit)
		}
		b.status = "保存に失敗しました。次に登録や履歴を操作したときに再試行します。"
		return changed, false
	}
	b.pendingLibraryEdits = nil
	b.data = latest
	return changed, true
}

func (b *browser) recordVideo(p *playerWindow, raw, title string) {
	e, ok := makeEntry(raw, title)
	if b.player != p || !ok || e.Kind != "動画" || p.historySuppressed[e.URL] {
		return
	}
	b.changeLibrary(func(data *library) bool {
		next := e
		if next.Title == next.URL {
			if i := slices.IndexFunc(data.History, func(x entry) bool { return x.URL == e.URL }); i >= 0 {
				next.Title = data.History[i].Title
			}
		}
		if len(data.History) > 0 && data.History[0] == next {
			return false
		}
		data.History = putFirst(data.History, next, 100)
		return true
	})
	if b.win != nil {
		b.win.Invalidate()
	}
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
	changed, saved := b.changeLibrary(func(data *library) bool {
		if len(data.Pins) >= 500 || slices.ContainsFunc(data.Pins, func(item entry) bool { return item.URL == e.URL }) {
			return false
		}
		data.Pins = putFirst(data.Pins, e, 500)
		return true
	})
	if changed {
		b.history, b.showLibrary = false, true
	}
	if saved {
		if changed {
			b.status = e.Kind + "を登録しました。"
		} else {
			b.status = "登録済み、または登録件数が上限に達しています。"
		}
	}
}

func (b *browser) remove(e entry) {
	if b.storeWarning != "" {
		b.status = b.storeWarning
		return
	}
	history := b.history
	if history {
		if p := b.player; p != nil {
			if p.historySuppressed == nil {
				p.historySuppressed = make(map[string]bool)
			}
			p.historySuppressed[e.URL] = true
		}
	}
	_, saved := b.changeLibrary(func(data *library) bool {
		items := &data.Pins
		if history {
			items = &data.History
		}
		before := len(*items)
		*items = slices.DeleteFunc(*items, func(x entry) bool { return x.URL == e.URL })
		return len(*items) != before
	})
	if saved {
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
		if mylistCopyCondition(b.current()) != "" {
			b.status += " " + mylistCopyCondition(b.current())
		}
	}
}

func (b *browser) draw(c *ui.Context) {
	c.SetTheme(readableAppTheme(c.Theme()))
	b.focusRegionShortcut(c)
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		b.toolbar(c)
		if !b.closeHelpAcknowledged {
			b.drawCloseHelp(c)
		}
		if b.showPlaybackSettings {
			b.drawPlaybackSettings(c)
		}
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			if b.showLibrary {
				b.sidebar(c)
			}
			if b.loginHelp {
				b.drawLoginHelp(c, b.official && b.officialTarget == login)
			} else if !b.official {
				b.drawNative(c)
			} else if b.web != nil {
				ui.WebView(c, b.web).Grow(1).MinWidth(0)
			} else {
				ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, b.status).Wrap()
					if b.officialTarget == login {
						b.drawLoginHelp(c, true)
					}
				})
			}
		})
		if b.status != "" || b.storeWarning != "" || b.official && b.commentLanguageWarning {
			ui.Row(c).FillWidth().Wrap().Padding(5, 12).Gap(8).Background(t.Surface).Children(func() {
				if b.status != "" {
					ui.Text(c, b.status).TextColor(t.TextMuted).Wrap()
				}
				if b.official && b.commentLanguageWarning {
					ui.Text(c, commentLanguageWarning).TextColor(t.TextMuted).Wrap()
				}
				if b.storeWarning != "" {
					ui.Text(c, b.storeWarning).TextColor(t.TextMuted).Wrap()
				}
			})
		}
	})
}

func (b *browser) toolbar(c *ui.Context) {
	var p *mygo.Page
	if b.official && b.web != nil {
		p = b.web.Page()
	}
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(6).Padding(8, 10).Background(t.Surface).Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			back := b.loginHelp || b.official || b.index > 0
			forward := p != nil && p.CanGoForward() || !b.official && b.index+1 < len(b.navigation)
			if ui.Button(c, "‹").Label("戻る").Disabled(!back).Clicked() || back && c.Shortcut(ui.Alt, ui.KeyLeft) {
				b.loginHelp = false
				if p != nil && p.CanGoBack() {
					p.GoBack()
				} else if b.official {
					b.leaveOfficial()
				} else {
					b.moveNative(-1)
				}
			}
			if ui.Button(c, "›").Label("進む").Disabled(!forward).Clicked() || forward && c.Shortcut(ui.Alt, ui.KeyRight) {
				b.loginHelp = false
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
			input := ui.TextInput(c, &b.searchInput.field).Bind(&b.searchFocus).Label("キーワード、タグ、動画IDまたはURL").Placeholder("キーワード・タグ・動画ID・URL").Grow(1).MinWidth(0)
			if b.focusSearch || c.Shortcut(ui.Ctrl, ui.KeyL) {
				b.focusSearch = false
				input.Focus()
			}
			keyword := ui.PrimaryButton(c, "検索").Tooltip("キーワード検索").Checked(b.searchInput.active && !b.searchInput.byTag)
			tag := ui.Button(c, "タグ検索").Checked(b.searchInput.active && b.searchInput.byTag)
			switch {
			case keyword.Clicked():
				b.search(false)
			case tag.Clicked():
				b.search(true)
			case input.Submitted():
				b.search(b.searchInput.byTag)
			}
		})
		ui.Row(c).FillWidth().Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
			if b.official && ui.Button(c, "独自UIに戻る").Clicked() {
				b.leaveOfficial()
			}
			if ui.Button(c, "登録・履歴").Checked(b.showLibrary).Clicked() {
				b.toggleLibrary()
			}
			if ui.Button(c, "結果へ移動").Disabled(b.official).Tooltip("本文へ移動（F6 で領域間を移動、Shift+F6 で逆順）").Clicked() {
				b.contentFocus.Focus(c)
			}
			if ui.Button(c, "マイリスト").Clicked() {
				b.navigate(mylists)
			}
			saved, _ := canonical(b.current())
			if ui.Button(c, "☆ 登録").Tooltip("表示中のページを登録").Disabled(saved == "").Clicked() {
				b.pinCurrent()
			}
			if ui.Button(c, "URLをコピー").Tooltip("go-nico-list用のURLをコピー").Disabled(goNicoListURL(b.current()) == "").Clicked() {
				b.copyCurrent()
			}
			if notice := mylistCopyCondition(b.current()); notice != "" {
				ui.Text(c, notice).Wrap().TextColor(t.TextMuted)
			}
			if ui.Button(c, "設定").Clicked() {
				b.showPlaybackSettings = !b.showPlaybackSettings
			}
			loginLabel := "ログイン"
			loginTarget := login
			if b.loginState == loginAuthenticated {
				loginLabel = "アカウント"
				loginTarget = "https://account.nicovideo.jp/my/account"
			}
			if ui.Button(c, loginLabel).Clicked() {
				b.navigate(loginTarget)
			}
			ui.Text(c, b.loginState.label()).Role(ui.RoleStatus).TextColor(t.TextMuted)
			if ui.Button(c, "再確認").Label("ログイン状態を再確認").Disabled(b.loginBusy).Clicked() {
				b.checkLogin()
			}
		})
	})
}

func (b *browser) sidebar(c *ui.Context) {
	width, _ := c.Size()
	sidebarWidth := float32(218)
	if width < 900 {
		sidebarWidth = 174
	}
	ui.Column(c).Bind(&b.libraryRegion).Width(sidebarWidth).Background(c.Theme().Surface).Padding(10).Gap(8).Children(func() {
		ui.Text(c, "登録・履歴").Bold().Focusable().Label("登録・履歴領域").Bind(&b.libraryFocus)
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
			notice := "視聴履歴はありません。"
			if !b.history {
				notice = "動画や検索結果を開き、上の「☆ 登録」でここに保存できます。"
			}
			ui.Text(c, notice).TextColor(c.Theme().TextMuted).Wrap()
			return
		}
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, item := range items {
					row := c.Key(item.URL)
					ui.Row(row).Gap(4).AlignItems(ui.Center).Children(func() {
						if ui.Button(row, "["+item.Kind+"] "+item.Title).Tooltip(item.Title).Grow(1).MinWidth(0).Clicked() {
							b.navigate(item.openURL())
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

func (b *browser) drawLoginHelp(c *ui.Context, retry bool) {
	ui.Column(c).Grow(1).MinWidth(0).Padding(20).Gap(12).Children(func() {
		ui.Text(c, "公式ログインを開く前に").FontSize(24).Bold()
		ui.Text(c, "Windowsでは Microsoft Edge WebView2 Runtime が必要です。ニコニコの公式ログインを同じアプリ内で開きます。").Wrap()
		ui.Text(c, "ニコニコのアカウントでログインしてください。外部事業者の認証先とアプリ外の別窓への移動には対応していません。").Wrap()
		label := "公式ログインを開く"
		if retry {
			label = "ログイン画面を再試行"
		}
		if ui.PrimaryButton(c, label).Clicked() {
			b.loginHelp = false
			b.openOfficial(login)
		}
		if ui.Button(c, "検索へ戻る").Clicked() {
			b.loginHelp = false
			b.navigate(home)
		}
		if ui.Button(c, "WebView2 Runtime の公式案内（外部ブラウザー）").Clicked() {
			if err := mygo.Shell.OpenExternal("https://developer.microsoft.com/microsoft-edge/webview2/"); err != nil {
				b.status = "Runtimeの案内を開けませんでした。"
			}
		}
	})
}

func mylistCopyCondition(raw string) string {
	if _, kind := canonical(raw); kind == "マイリスト" && goNicoListURL(raw) != "" {
		return "非公開のリストは外部ツールでは取得できません。"
	}
	return ""
}
