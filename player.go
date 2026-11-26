package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const webSourceInterval = 500 * time.Millisecond

type playerWindow struct {
	win                    *mygo.Window
	view                   *mygo.WebView
	mode                   playbackMode
	native                 *nativePlayer
	target, title, status  string
	commentLanguageWarning bool
	historySuppressed      map[string]bool
	document               playerDocument
}

func (p *playerWindow) resumeHistory(video string) {
	delete(p.historySuppressed, video)
}

type playerDocument struct {
	loaded               bool
	revision             uint64
	pending              string
	source, video, title string
	previousTitle        string
}

func (d *playerDocument) observe(raw, title string) bool {
	changed := d.source != raw
	d.source = raw
	r := routeFor(raw)
	video := ""
	if r.kind == screenVideo {
		video = r.link
	}
	if d.video != video {
		if d.title != "" {
			d.previousTitle = d.title
		}
		d.video, d.title = video, ""
	}
	// A SPA can update the title and the URL in either order. Keep the first
	// title for this video, and wait past the previous video's title on a new URL.
	if title = strings.TrimSpace(title); d.video != "" && d.title == "" && title != "" && title != d.previousTitle {
		d.title = title
		changed = true
	}
	return changed
}

func (b *browser) openPlayer(raw string) { b.openPlayerMode(raw, b.playMode) }

func (b *browser) openPlayerMode(raw string, mode playbackMode) {
	if mode != nativePlayback {
		mode = webPlayback
	}
	route := routeFor(raw)
	if route.kind != screenVideo {
		return
	}
	b.playbackRequest++
	request := b.playbackRequest
	// UI callbacks run while the frame is being built; create and navigate
	// the native window after that callback finishes.
	go mygo.RunOnMain(func() {
		if b.closed || b.playbackRequest != request {
			return
		}
		if p := b.player; p != nil && p.mode != mode {
			b.player = nil
			p.win.Close()
		}
		if p := b.player; p != nil {
			p.resumeHistory(route.link)
			if p.mode == nativePlayback {
				b.loadNativePlayer(p, route)
				p.win.Show()
				p.win.Focus()
				return
			}
			p.target = route.target
			if p.view != nil {
				p.load(route.target)
			}
			p.win.Show()
			p.win.Focus()
			return
		}
		p := &playerWindow{mode: mode, target: route.target, status: "再生を準備しています…"}
		p.win = mygo.NewWindow(mygo.WindowOptions{
			Title: appName + " · プレーヤー",
			Width: 1060, Height: 700, MinWidth: 560, MinHeight: 420,
			Content: ui.View(func(c *ui.Context) { b.drawPlayer(p, c) }),
			Page:    mygo.PageOptions{DevTools: mygo.DevToolsDisabled},
		})
		b.player = p
		p.win.OnClosed(func() {
			// MyGo destroys WebViews owned by a closed window.
			if b.player == p {
				b.player = nil
			}
			p.view = nil
			p.stopNative()
		})
		if mode == nativePlayback {
			b.loadNativePlayer(p, route)
		} else {
			b.attachPlayer(p)
		}
	})
}

func (b *browser) attachPlayer(p *playerWindow) {
	active := func() bool { return !b.closed && b.player == p }
	if !active() {
		return
	}
	p.commentLanguageWarning = setJapaneseComments(b.session.Cookies.Set, time.Now()) != nil
	if !active() {
		return
	}
	view, err := p.win.NewWebView(mygo.WebViewOptions{Page: mygo.PageOptions{
		Untrusted: true, DevTools: mygo.DevToolsDisabled, Session: b.session,
	}})
	if !active() {
		if view != nil {
			view.Destroy()
		}
		return
	}
	if err != nil {
		p.status = "公式プレーヤーを起動できません。WebView2 Runtimeを確認してください。"
		p.win.Invalidate()
		return
	}
	p.view = view
	page := view.Page()
	routeLink := func(raw string) bool {
		dest := routeFor(raw)
		switch dest.kind {
		case screenOfficial:
			return false
		case screenVideo:
			if b.playMode == nativePlayback && dest.link != routeFor(page.URL()).link {
				b.openPlayer(dest.target)
				return true
			}
			p.resumeHistory(dest.link)
			return false
		}
		go mygo.RunOnMain(func() {
			if active() {
				b.openNative(dest, true)
				b.win.Show()
				b.win.Focus()
			}
		})
		return true
	}
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		if !active() || allowedURL(e.URL) == nil || e.UserInitiated && routeLink(e.URL) {
			e.PreventDefault()
		}
	})
	page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		u := allowedURL(req.URL)
		if !active() || u == nil || !req.UserInitiated && !strings.EqualFold(u.Hostname(), "account.nicovideo.jp") {
			return nil
		}
		if req.UserInitiated && routeLink(u.String()) {
			return nil
		}
		go mygo.RunOnMain(func() {
			if active() {
				p.load(u.String())
			}
		})
		return nil
	})
	page.OnPageTitleUpdated(func(e *mygo.TitleEvent) {
		e.PreventDefault()
	})
	page.OnDidNavigate(func(_ string) {
		if active() {
			p.document = playerDocument{revision: p.document.revision + 1}
			p.win.SetTitle(appName + " · プレーヤー")
		}
	})
	page.OnDidFinishLoad(func() {
		if !active() {
			return
		}
		p.document.pending = ""
		p.document.loaded = allowedURL(page.URL()) != nil
		if p.document.loaded {
			p.status = ""
		}
		p.win.Invalidate()
	})
	page.OnDidFailLoad(func(_ *mygo.LoadError) {
		if !active() {
			return
		}
		p.document.loaded = false
		p.status = "動画ページを開けませんでした。"
		p.win.Invalidate()
	})
	page.OnRenderProcessGone(func(_ string) {
		if !active() {
			return
		}
		p.document.loaded = false
		p.status = "プレーヤーが終了しました。ウィンドウを閉じて再度開いてください。"
		p.win.Invalidate()
	})
	p.load(p.target)
	watchWebSource(active, func() { b.observePlayerPage(p, page, active) })
}

func watchWebSource(active func() bool, observe func()) {
	go func() {
		ticker := time.NewTicker(webSourceInterval)
		defer ticker.Stop()
		for range ticker.C {
			keep := false
			mygo.RunOnMain(func() {
				keep = active()
				if keep {
					observe()
				}
			})
			if !keep {
				return
			}
		}
	}()
}

func (b *browser) observePlayerPage(p *playerWindow, page *mygo.Page, active func() bool) {
	d := &p.document
	if !d.loaded || page.IsLoading() {
		return
	}
	raw := page.URL()
	replay := d.pending != ""
	if replay && raw != d.pending || !replay && raw == d.source && (d.title != "" || d.video == "") {
		return
	}
	revision := d.revision
	title := ""
	if routeFor(raw).kind == screenVideo {
		// Sample URL and title in one renderer call. Eval can pump the UI loop,
		// so validate the load and source again before using its result.
		ctx, cancel := context.WithTimeout(context.Background(), webSourceInterval)
		value, err := page.EvalContext(ctx, `[location.href, document.title.slice(0, 1024)]`)
		cancel()
		if pair, ok := value.([]any); err == nil && ok && len(pair) == 2 && pair[0] == raw {
			title, _ = pair[1].(string)
		}
	}
	if !active() || !d.loaded || revision != d.revision || page.IsLoading() || page.URL() != raw {
		return
	}
	d.pending = ""
	if !d.observe(raw, title) && !replay {
		return
	}
	if d.video != "" {
		b.recordVideo(p, d.video, d.title)
	}
	windowTitle := appName + " · プレーヤー"
	if d.title != "" {
		windowTitle = d.title + " · " + appName
	}
	p.win.SetTitle(windowTitle)
}

func (p *playerWindow) load(target string) {
	if p.view == nil || allowedURL(target) == nil {
		return
	}
	p.target, p.status = target, ""
	p.document.pending = target
	p.document.revision++
	if err := p.view.Page().LoadURL(target); err != nil {
		p.document.pending = ""
		p.status = "動画ページを開けませんでした。"
	}
	p.win.Invalidate()
}

func (p *playerWindow) draw(c *ui.Context) {
	ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() {
		if p.view != nil {
			ui.WebView(c, p.view).Grow(1).FillWidth().MinHeight(0)
		}
		if p.status != "" {
			ui.Text(c, p.status).Wrap()
		}
		if p.commentLanguageWarning {
			ui.Text(c, commentLanguageWarning).TextColor(c.Theme().TextMuted).Padding(5, 12).Wrap()
		}
	})
}
