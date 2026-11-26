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
	webVideo               string // the video currently allowed in this WebView
	document               playerDocument
}

func (p *playerWindow) resumeHistory(video string) {
	delete(p.historySuppressed, video)
}

func (p *playerWindow) visitWebVideo(video string) {
	if p.webVideo != video {
		p.resumeHistory(video)
		p.webVideo = video
	}
}

func (p *playerWindow) needsNativePlayer(route screenRoute, mode playbackMode) bool {
	return mode == nativePlayback && route.kind == screenVideo && route.link != p.webVideo
}

type playerDocument struct {
	loaded               bool
	committed            bool
	revision             uint64
	pending              string
	source, video, title string
	lookingUpTitle       bool
}

func (d *playerDocument) begin(raw string) {
	d.pending, d.loaded, d.committed = raw, false, false
	d.revision++
	d.title = ""
	d.lookingUpTitle = false
}

func (d *playerDocument) commit(raw string) {
	d.begin(raw)
	d.committed = true
}

func (d *playerDocument) matchesSource(raw string) bool {
	if d.pending == "" || samePageURL(d.pending, raw) {
		return true
	}
	// pushState can change the URL of the committed document before its
	// finish notification. A pending request has no committed document yet.
	return d.committed && samePageOrigin(d.pending, raw)
}

func (d *playerDocument) finish(raw string) bool {
	if d.pending == "" || !d.matchesSource(raw) {
		return false
	}
	d.pending, d.loaded, d.source = "", true, ""
	return true
}

func (d *playerDocument) failed(raw, current string) bool {
	// WebView2 can report the live source URL on a previous load's failure.
	// There must still be a pending load, and both URLs must match it.
	return d.pending != "" && samePageURL(d.pending, current) && samePageURL(d.pending, raw)
}

func samePageOrigin(a, b string) bool {
	u, v := allowedURL(a), allowedURL(b)
	return u != nil && v != nil && strings.EqualFold(u.Hostname(), v.Hostname())
}

func samePageURL(a, b string) bool {
	if a == b {
		return a != ""
	}
	u, v := allowedURL(a), allowedURL(b)
	return u != nil && v != nil && strings.EqualFold(u.Hostname(), v.Hostname()) &&
		u.Path == v.Path && u.RawQuery == v.RawQuery && u.Fragment == v.Fragment
}

func (d *playerDocument) observe(raw string, route screenRoute) bool {
	changed := d.source != raw
	d.source = raw
	video := ""
	if route.kind == screenVideo {
		video = route.link
	}
	if d.video != video {
		d.video, d.title = video, ""
		d.lookingUpTitle = false
		d.revision++
	}
	return changed
}

func (d *playerDocument) acceptTitle(revision uint64, video, title string) bool {
	if title = strings.TrimSpace(title); !d.loaded || d.revision != revision || video == "" || d.video != video || title == "" || d.title != "" {
		return false
	}
	d.title = title
	return true
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
			p.visitWebVideo(route.link)
			p.target = route.target
			if p.view != nil {
				p.load(route.target)
			}
			p.win.Show()
			p.win.Focus()
			return
		}
		p := &playerWindow{mode: mode, target: route.target, webVideo: route.link, status: "再生を準備しています…"}
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
			if dest.link == p.webVideo {
				p.resumeHistory(dest.link)
			}
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
		if !active() || allowedURL(e.URL) == nil || b.handoffWatch(p, routeFor(e.URL)) || e.UserInitiated && routeLink(e.URL) {
			e.PreventDefault()
			return
		}
		p.document.begin(e.URL)
	})
	page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		u := allowedURL(req.URL)
		if !active() || u == nil || !req.UserInitiated && !strings.EqualFold(u.Hostname(), "account.nicovideo.jp") {
			return nil
		}
		if b.handoffWatch(p, routeFor(u.String())) || req.UserInitiated && routeLink(u.String()) {
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
	page.OnDidNavigate(func(raw string) {
		if active() && allowedURL(raw) != nil && !b.handoffWatch(p, routeFor(raw)) {
			p.document.commit(raw)
			p.setWebTitle()
		}
	})
	page.OnDidFinishLoad(func() {
		if !active() || !p.document.finish(page.URL()) {
			return
		}
		p.status = ""
		p.win.Invalidate()
	})
	page.OnDidFailLoad(func(err *mygo.LoadError) {
		if !active() || !p.document.failed(err.URL, page.URL()) {
			return
		}
		p.status = "動画ページを開けませんでした。"
		p.win.Invalidate()
	})
	page.OnRenderProcessGone(func(_ string) {
		if !active() {
			return
		}
		p.document.loaded = false
		p.document.pending = ""
		p.document.revision++
		p.status = "プレーヤーが終了しました。ウィンドウを閉じて再度開いてください。"
		p.win.Invalidate()
	})
	p.load(p.target)
	watchWebSource(active, func() { b.observePlayerPage(p, page, active) })
}

func (b *browser) handoffWatch(p *playerWindow, route screenRoute) bool {
	if !p.needsNativePlayer(route, b.playMode) {
		return false
	}
	b.openPlayer(route.target)
	return true
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
	raw := page.URL()
	if !d.matchesSource(raw) {
		return
	}
	route := routeFor(raw)
	if b.handoffWatch(p, route) || !d.loaded || page.IsLoading() {
		return
	}
	if d.observe(raw, route) {
		if d.video != "" {
			p.visitWebVideo(d.video)
			b.recordVideo(p, d.video, d.title)
		}
		p.setWebTitle()
	}
	if d.video == "" || d.title != "" {
		return
	}

	revision, video := d.revision, d.video
	applyTitle := func(title string) {
		if active() && routeFor(page.URL()).link == video && !page.IsLoading() && d.acceptTitle(revision, video, title) {
			b.recordVideo(p, video, d.title)
			p.setWebTitle()
		}
	}
	// Read the ID and title from the same watch response. The document title
	// and URL can change independently during a same-document navigation.
	ctx, cancel := context.WithTimeout(context.Background(), webSourceInterval)
	value, err := page.EvalContext(ctx, `(() => {
		try {
			const data = JSON.parse(document.querySelector('meta[name="server-response"]').content);
			const video = data.data.response.$watchV4.data.video;
			return [location.href, video.id, video.title.slice(0, 1024)];
		} catch { return null; }
	})()`)
	cancel()
	if err == nil {
		applyTitle(watchResponseTitle(value, raw, route.key))
	}
	if !active() || d.revision != revision || routeFor(page.URL()).link != video || d.title != "" || d.lookingUpTitle {
		return
	}
	// The server response in the DOM may still belong to the first document.
	// Look up metadata for this video ID without using the WebView's cookies.
	d.lookingUpTitle = true
	go func() {
		result, err := b.catalog.video(context.Background(), route)
		if err == nil {
			mygo.RunOnMain(func() { applyTitle(result.title) })
		}
	}()
}

func watchResponseTitle(value any, raw, videoID string) string {
	if pair, ok := value.([]any); ok && len(pair) == 3 && pair[0] == raw && pair[1] == videoID {
		title, _ := pair[2].(string)
		return title
	}
	return ""
}

func (p *playerWindow) setWebTitle() {
	windowTitle := appName + " · プレーヤー"
	if p.document.title != "" {
		windowTitle = p.document.title + " · " + appName
	}
	p.win.SetTitle(windowTitle)
}

func (p *playerWindow) load(target string) {
	if p.view == nil || allowedURL(target) == nil {
		return
	}
	p.target, p.status = target, ""
	page := p.view.Page()
	current := page.URL()
	currentBase, _, _ := strings.Cut(current, "#")
	targetBase, _, _ := strings.Cut(target, "#")
	if !p.document.loaded || current == target || !samePageURL(currentBase, targetBase) {
		p.document.begin(target)
	}
	if err := page.LoadURL(target); err != nil {
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
