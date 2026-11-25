package main

import (
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type playerWindow struct {
	win                              *mygo.Window
	view                             *mygo.WebView
	mode                             playbackMode
	native                           *nativePlayer
	target, loadedURL, title, status string
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
	// UI callbacks run while the frame is being built; create and navigate
	// the native window after that callback finishes.
	go mygo.RunOnMain(func() {
		if b.closed {
			return
		}
		if p := b.player; p != nil && p.mode != mode {
			b.player = nil
			p.win.Close()
		}
		if p := b.player; p != nil {
			if p.mode == nativePlayback {
				b.loadNativePlayer(p, route)
				p.win.Show()
				p.win.Focus()
				return
			}
			p.target = route.link
			if p.view != nil {
				p.load(route.link)
			}
			p.win.Show()
			p.win.Focus()
			return
		}
		p := &playerWindow{mode: mode, target: route.link, status: "再生を準備しています…"}
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
	_ = setJapaneseComments(b.session.Cookies.Set, time.Now())
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
	toLibrary := func(raw string) bool {
		dest := routeFor(raw)
		if dest.kind == screenOfficial || dest.link == routeFor(p.target).link {
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
		if !active() || allowedURL(e.URL) == nil || e.UserInitiated && toLibrary(e.URL) {
			e.PreventDefault()
		}
	})
	page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		u := allowedURL(req.URL)
		if !active() || u == nil || !req.UserInitiated && !strings.EqualFold(u.Hostname(), "account.nicovideo.jp") {
			return nil
		}
		if req.UserInitiated && toLibrary(u.String()) {
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
		if !active() {
			return
		}
		if title := strings.TrimSpace(e.Title); title != "" {
			p.title = title
			p.win.SetTitle(title + " · " + appName)
			if p.loadedURL != "" {
				b.recordVideoURL(p.loadedURL, title)
			}
		}
	})
	page.OnDidFinishLoad(func() {
		if !active() {
			return
		}
		raw := page.URL()
		if allowedURL(raw) != nil {
			p.loadedURL = raw
			p.status = ""
			b.recordVideoURL(raw, p.title)
		}
		p.win.Invalidate()
	})
	page.OnDidFailLoad(func(_ *mygo.LoadError) {
		if !active() {
			return
		}
		p.status = "動画ページを開けませんでした。"
		p.win.Invalidate()
	})
	page.OnRenderProcessGone(func(_ string) {
		if !active() {
			return
		}
		p.status = "プレーヤーが終了しました。ウィンドウを閉じて再度開いてください。"
		p.win.Invalidate()
	})
	p.load(p.target)
}

func (p *playerWindow) load(target string) {
	if p.view == nil || allowedURL(target) == nil {
		return
	}
	p.target, p.loadedURL, p.title, p.status = target, "", "", ""
	if err := p.view.Page().LoadURL(target); err != nil {
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
	})
}
