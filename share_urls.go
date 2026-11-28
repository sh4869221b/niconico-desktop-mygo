package main

import (
	"math"

	"github.com/egoist/mygo/ui"
)

func ordinaryShareURL(raw string) (string, string) {
	canonicalURL, kind := canonical(raw)
	switch kind {
	case "動画":
		return canonicalURL, "動画URLをコピー"
	case "キーワード検索", "タグ検索":
		if allowedURL(raw) != nil {
			return raw, "現在の検索条件をコピー"
		}
	}
	if route := routeFor(raw); route.kind == screenList || route.kind == screenUser || route.kind == screenUserSeries || route.kind == screenSeries {
		return canonicalURL, "ページURLをコピー"
	}
	return "", "ページURLをコピー"
}
func copyURL(c *ui.Context, target string, notice func(string)) {
	if target == "" {
		return
	}
	c.WriteClipboardThen(target, func(err error) {
		if err != nil {
			notice("クリップボードにコピーできませんでした。")
		} else {
			notice("URLをコピーしました。")
		}
		c.Invalidate()
	})
}

func playerShareURLs(p *playerWindow, live playbackPage) (string, string) {
	raw := p.target
	webWatch := screenRoute{}
	if p.native == nil && live != nil && p.document.loaded && !p.document.loading && p.failedWebTarget == "" && !p.replaceFailedWebView && !live.IsLoading() {
		shown := routeFor(live.URL())
		// The requested video can already have changed before WebView signals loading.
		if shown.kind == screenVideo && shown.link == p.webVideo {
			webWatch, raw = shown, shown.link
		}
	}
	base, _ := canonical(raw)
	if routeFor(base).kind != screenVideo {
		return "", ""
	}
	seconds, verified := float64(0), false
	if n := p.native; n != nil {
		if n.stream != nil && n.stream.Ready() && !n.stream.Preparing() {
			seconds = n.stream.Position().Seconds()
			verified = true
		}
	} else if webWatch.kind == screenVideo && !p.returningToVideo && p.restoring == nil && p.playedPosition != nil && p.playedVideo != "" &&
		p.document.video == webWatch.link && p.playedWatch.link == webWatch.link {
		seconds = p.playedPosition.seconds
		verified = true
	}
	if !verified || math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || seconds >= 1<<33 {
		return base, ""
	}
	return base, playbackRestoreURL(base, seconds)
}
func (p *playerWindow) drawSharing(c *ui.Context, live playbackPage) {
	base, timed := playerShareURLs(p, live)
	ui.Row(c).Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
		notice := func(s string) { p.shareNotice = s }
		if ui.Button(c, "動画URLをコピー").Disabled(base == "").Clicked() {
			copyURL(c, base, notice)
		}
		if ui.Button(c, "この時点のURLをコピー").Disabled(timed == "").Clicked() {
			copyURL(c, timed, notice)
		}
		if p.shareNotice != "" {
			ui.Text(c, p.shareNotice).Wrap().Role(ui.RoleStatus)
		}
	})
}
