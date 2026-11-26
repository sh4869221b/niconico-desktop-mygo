package main

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const webSourceInterval = 500 * time.Millisecond
const maxTitleLookups = 3

type playerWindow struct {
	win                    *mygo.Window
	view                   *mygo.WebView
	mode                   playbackMode
	native                 *nativePlayer
	target, title, status  string
	commentLanguageWarning bool
	historySuppressed      map[string]bool
	webVideo               string // the video currently allowed in this WebView
	returningToVideo       bool
	playedWatch            screenRoute
	playedPosition         *webPlaybackPosition
	restoring              *webPlaybackRestore
	restoreToken           uint64
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
	loading              bool
	loaded               bool
	revision             uint64
	committedURL         string
	source, video, title string
	titleAttempts        int
	titleRetryAt         time.Time
	cancelTitleLookup    context.CancelFunc
}

func (d *playerDocument) begin() {
	d.loading = true
	d.committedURL = ""
	d.resetTitleLookup()
}

func (d *playerDocument) commit(raw string) {
	if !d.loading {
		d.begin()
	}
	d.committedURL = raw
	d.loaded = false
	d.title = ""
}

func (d *playerDocument) finish(raw string) {
	d.loading, d.loaded, d.source = false, allowedURL(raw) != nil, ""
	d.committedURL = ""
}

func (d *playerDocument) stopLoad() {
	// A failure before commit can leave the previously loaded page in place.
	d.loading = false
	d.committedURL = ""
}

func (d *playerDocument) resetTitleLookup() {
	d.stopTitleLookup()
	d.revision++
	d.titleAttempts, d.titleRetryAt = 0, time.Time{}
}

func (d *playerDocument) stopTitleLookup() {
	if d.cancelTitleLookup != nil {
		d.cancelTitleLookup()
		d.cancelTitleLookup = nil
	}
}

func (d *playerDocument) startTitleLookup(now time.Time) (context.Context, bool) {
	if !d.loaded || d.loading || d.video == "" || d.title != "" || d.cancelTitleLookup != nil ||
		d.titleAttempts >= maxTitleLookups || now.Before(d.titleRetryAt) {
		return nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.cancelTitleLookup = cancel
	d.titleAttempts++
	return ctx, true
}

func (d *playerDocument) finishTitleLookup(revision uint64, video string, now time.Time) bool {
	if d.revision != revision || d.video != video || d.cancelTitleLookup == nil {
		return false
	}
	d.stopTitleLookup()
	d.titleRetryAt = now.Add(time.Duration(d.titleAttempts) * time.Second)
	return true
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
		d.resetTitleLookup()
	}
	return changed
}

func (d *playerDocument) acceptTitle(revision uint64, video, title string) bool {
	if title = strings.TrimSpace(title); !d.loaded || d.loading || d.revision != revision || video == "" || d.video != video || title == "" || d.title != "" {
		return false
	}
	d.title = title
	d.stopTitleLookup()
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
			p.restoring = nil
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
			p.document.resetTitleLookup()
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
		Untrusted: true, DevTools: mygo.DevToolsDisabled, Session: b.session, PreloadScript: webPlayerPreload,
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
	routeLink := func(dest screenRoute) bool {
		if dest.kind == screenVideo && dest.link == p.webVideo {
			p.resumeHistory(dest.link)
		}
		return b.browseFromPlayer(p, dest, false)
	}
	page.OnWillNavigate(func(e *mygo.NavigateEvent) {
		dest := routeFor(e.URL)
		if !active() || allowedURL(e.URL) == nil || b.handoffWatch(p, dest) ||
			(e.UserInitiated || dest.isSearchPage()) && routeLink(dest) {
			e.PreventDefault()
		}
	})
	page.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		u := allowedURL(req.URL)
		if !active() || u == nil || !req.UserInitiated && !strings.EqualFold(u.Hostname(), "account.nicovideo.jp") {
			return nil
		}
		dest := routeFor(u.String())
		if b.handoffWatch(p, dest) || req.UserInitiated && routeLink(dest) {
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
	page.OnDidStartLoad(func(raw string) {
		if active() {
			if p.restoring != nil && routeFor(raw).target != p.restoring.target {
				p.restoring = nil
			}
			p.document.begin()
			p.status = ""
			p.win.Invalidate()
		}
	})
	page.OnDidNavigate(func(raw string) {
		if !active() {
			return
		}
		p.document.commit(raw)
		dest := routeFor(raw)
		if !b.handoffWatch(p, dest) && !b.browseFromPlayer(p, dest, true) {
			p.setWebTitle()
		}
	})
	page.OnDidFinishLoad(func() {
		if !active() {
			return
		}
		p.finishWebLoad(page.URL())
		p.status = ""
		p.win.Invalidate()
	})
	page.OnDidFailLoad(func(_ *mygo.LoadError) {
		if !active() {
			return
		}
		p.document.stopLoad()
		p.restoring = nil
		p.status = "動画ページを開けませんでした。"
		p.win.Invalidate()
	})
	page.OnDidCancelLoad(func() {
		if active() {
			p.document.stopLoad()
			p.restoring = nil
		}
	})
	page.OnRenderProcessGone(func(_ string) {
		if !active() {
			return
		}
		p.document.loaded = false
		p.document.stopLoad()
		p.restoring = nil
		p.document.resetTitleLookup()
		p.status = "プレーヤーが終了しました。ウィンドウを閉じて再度開いてください。"
		p.win.Invalidate()
	})
	p.load(p.target)
	watchWebSource(active, func() { b.observePlayerPage(p, page, active) })
}

func (r screenRoute) browsesInMain() bool {
	return r.officialBrowse || r.kind != screenVideo && r.kind != screenOfficial
}

func (r screenRoute) isSearchPage() bool {
	return r.kind == screenSearch || r.officialBrowse &&
		(strings.HasPrefix(r.link, home+"search/") || strings.HasPrefix(r.link, home+"tag/"))
}

func (b *browser) browseFromPlayer(p *playerWindow, dest screenRoute, alreadyNavigated bool) bool {
	if !dest.browsesInMain() {
		return false
	}
	if alreadyNavigated {
		if p.returningToVideo {
			return true
		}
		p.returningToVideo = true
	}
	revision := p.document.revision
	go mygo.RunOnMain(func() {
		if alreadyNavigated {
			defer func() { p.returningToVideo = false }()
		}
		if b.closed || b.player != p || p.view == nil {
			return
		}
		if alreadyNavigated {
			page := p.view.Page()
			if p.document.revision != revision || page.URL() != dest.target {
				return
			}
			// A history or same-document navigation may have already left the video.
			p.expireWebRestore(time.Now())
			p.captureWebPosition(page)
			if b.closed || b.player != p || p.view == nil || p.document.revision != revision || page.URL() != dest.target {
				return
			}
			p.returnToPlayedWatch()
		}
		b.navigate(dest.target)
		b.win.Show()
		b.win.Focus()
	})
	return true
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
	if p.expireWebRestore(time.Now()) {
		p.showWebRestoreFailure()
	}
	if d.loading || !d.loaded || page.IsLoading() {
		return
	}
	raw := page.URL()
	route := routeFor(raw)
	if b.handoffWatch(p, route) || b.browseFromPlayer(p, route, true) {
		return
	}
	if d.observe(raw, route) {
		if d.video != "" {
			p.visitWebVideo(d.video)
			b.recordVideo(p, d.video, d.title)
		}
		p.setWebTitle()
	}
	if route.kind == screenVideo {
		p.applyWebRestore(page, route)
		p.captureWebPosition(page)
		if !active() || d.loading || page.URL() != raw {
			return
		}
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
	if !active() || d.revision != revision || routeFor(page.URL()).link != video {
		return
	}
	// The server response in the DOM may still belong to the first document.
	// Look up metadata for this video ID without using the WebView's cookies.
	lookupCtx, ok := d.startTitleLookup(time.Now())
	if !ok {
		return
	}
	go func() {
		result, err := b.catalog.video(lookupCtx, route)
		mygo.RunOnMain(func() {
			if active() && d.finishTitleLookup(revision, video, time.Now()) && err == nil {
				applyTitle(result.title)
			}
		})
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
	p.status = ""
	if err := p.view.Page().LoadURL(target); err != nil {
		p.restoring = nil
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
