package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func readSessionCookie(list func(mygo.CookieFilter) ([]mygo.Cookie, error), target string, now time.Time) (*http.Cookie, error) {
	u, err := url.Parse(target)
	if err != nil || !sessionEndpoint(u) {
		return nil, errSignIn
	}
	values, err := list(mygo.CookieFilter{URL: target, Name: "user_session"})
	if err != nil {
		return nil, errSignIn
	}
	for _, value := range values {
		if value.Name != "user_session" || !value.Secure ||
			value.Domain != ".nicovideo.jp" && value.Domain != apiHost && value.Domain != "."+apiHost ||
			!value.Expires.IsZero() && !value.Expires.After(now) {
			continue
		}
		c := &http.Cookie{Name: value.Name, Value: value.Value, Secure: true, HttpOnly: true}
		if c.Value != "" && c.Valid() == nil {
			return c, nil
		}
	}
	return nil, errSignIn
}

func (b *browser) navigate(link string) {
	if allowedURL(link) == nil {
		return
	}
	r := routeFor(link)
	if r.kind == screenOfficial {
		b.openOfficial(link)
		return
	}
	b.openNative(r, true)
}

func (b *browser) openNative(r screenRoute, add bool) {
	if b.official {
		b.closeOfficial()
	}
	if add && (b.index == -1 || b.navigation[b.index] != r) {
		b.navigation = append(b.navigation[:b.index+1], r)
		b.index++
		if len(b.navigation) > 100 {
			b.navigation = b.navigation[1:]
			b.index--
		}
	}
	b.refreshNative()
}

func (b *browser) moveNative(delta int) {
	next := b.index + delta
	if next >= 0 && next < len(b.navigation) {
		b.index = next
		b.refreshNative()
	}
}

func (b *browser) pageNative(delta int) {
	if b.index < 0 {
		return
	}
	r := b.navigation[b.index]
	r.page += delta
	if r.page >= 0 && r.page <= 4166 {
		b.openNative(r, true)
	}
}

func (b *browser) cancelLoad() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
	b.epoch++
}

func (b *browser) refreshNative() {
	b.cancelLoad()
	if b.index < 0 || b.closed || b.official {
		return
	}
	r, epoch := b.navigation[b.index], b.epoch
	b.result, b.resultErr, b.busy, b.status = catalogPage{}, nil, r.kind != screenHome, ""
	if !b.busy {
		b.win.Invalidate()
		return
	}
	b.status = "読み込み中…"
	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	go func() {
		defer cancel()
		p, err := b.catalog.load(ctx, r)
		mygo.RunOnMain(func() {
			if b.closed || b.official || b.epoch != epoch {
				return
			}
			b.result, b.resultErr, b.busy = p, err, false
			b.status = p.searchMode
			b.win.Invalidate()
		})
	}()
	b.win.Invalidate()
}

// Only redirect explicit first-party links from a loaded player; don't
// interfere with redirects and callbacks used by sign-in or playback.
func (b *browser) redirectFromPlayer(target string) bool {
	r := routeFor(target)
	current := routeFor(b.source)
	if !b.loaded || current.kind != screenVideo || r.kind == screenOfficial || current.link == r.link {
		return false
	}
	view := b.web
	go mygo.RunOnMain(func() {
		if !b.closed && b.official && b.web == view {
			b.openNative(r, true)
		}
	})
	return true
}

func (b *browser) openOfficial(target string) {
	if allowedURL(target) == nil || b.closed {
		return
	}
	b.cancelLoad()
	b.official, b.officialTarget = true, target
	b.loading()
	b.win.Invalidate()
	go mygo.RunOnMain(func() {
		if b.closed || !b.official || b.officialTarget != target {
			return
		}
		if b.web == nil {
			b.attachOfficial()
		} else {
			b.loadOfficial(target)
		}
	})
}

func (b *browser) closeOfficial() {
	view := b.web
	b.web, b.official, b.loaded = nil, false, false
	b.source, b.status = "", ""
	if view != nil {
		// Destroy the previous view rather than leaving hidden playback running.
		go mygo.RunOnMain(view.Destroy)
	}
}

func (b *browser) leaveOfficial() {
	b.closeOfficial()
	b.refreshNative()
}

func (b *browser) drawNative(c *ui.Context) {
	t := c.Theme()
	if b.index < 0 {
		return
	}
	r, p := b.navigation[b.index], b.result
	scope := c.Key(fmt.Sprintf("native-%s-%d", r.link, r.page))
	ui.Column(scope).Grow(1).MinWidth(0).Padding(18, 22).Gap(12).Children(func() {
		if r.kind == screenHome {
			ui.Text(scope, "ニコニコ動画").FontSize(29).Bold()
			ui.Row(scope).Gap(8).Children(func() {
				if ui.Button(scope, "自分のマイリスト").Clicked() {
					b.navigate(mylists)
				}
				if ui.Button(scope, "ニコニコ公式トップ").Clicked() {
					b.openOfficial(home)
				}
			})
			return
		}
		title := p.title
		if title == "" {
			title = r.key
		}
		if title == "" {
			title = "マイリスト"
		}
		ui.Text(scope, title).FontSize(24).Bold().Wrap()
		ui.Row(scope).Gap(8).Children(func() {
			switch r.kind {
			case screenVideo:
				if ui.PrimaryButton(scope, "再生").Clicked() {
					b.openPlayer(r.link)
				}
			case screenUser:
				if ui.Button(scope, "公開マイリスト").Clicked() {
					b.navigate(home + "user/" + r.key + "/mylist")
				}
			case screenUserLists:
				if ui.Button(scope, "投稿動画").Clicked() {
					b.navigate(home + "user/" + r.key + "/video")
				}
			case screenOwnLists, screenOwnList:
				if errors.Is(b.resultErr, errSignIn) && ui.PrimaryButton(scope, "ニコニコにログイン").Clicked() {
					b.openOfficial(login)
				}
				if r.kind == screenOwnList && ui.Button(scope, "自分のマイリスト").Clicked() {
					b.navigate(mylists)
				}
			}
			if r.kind != screenVideo && ui.Button(scope, "公式ページ").Clicked() {
				b.openOfficial(r.link)
			}
		})
		if b.busy {
			return
		}
		if b.resultErr != nil {
			ui.Text(scope, b.resultErr.Error()).TextColor(t.TextMuted).Wrap()
			return
		}
		if r.page > 0 || p.more {
			ui.Row(scope).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.Button(scope, "前へ").Disabled(r.page == 0).Clicked() {
					b.pageNative(-1)
				}
				ui.Text(scope, fmt.Sprintf("%d ページ", r.page+1))
				if ui.Button(scope, "次へ").Disabled(!p.more).Clicked() {
					b.pageNative(1)
				}
			})
		}
		ui.Scroll(scope).Grow(1).MinHeight(0).Children(func() {
			ui.Column(scope).FillWidth().Gap(8).Children(func() {
				if p.author != "" {
					ui.Text(scope, p.author).TextColor(t.TextMuted).Wrap()
				}
				if p.summary != "" {
					ui.Text(scope, p.summary).Wrap()
				}
				for i, link := range p.links {
					row := scope.Key(fmt.Sprintf("link-%d", i))
					if ui.Button(row, link.title).Clicked() {
						b.navigate(link.link)
					}
				}
				if len(p.items) == 0 && r.kind != screenVideo {
					ui.Text(scope, "表示する項目はありません。").TextColor(t.TextMuted)
				}
				for i, item := range p.items {
					row := scope.Key(fmt.Sprintf("item-%d", i))
					ui.Row(row).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
						if ui.Button(row, item.title).Tooltip(item.title).Grow(1).MinWidth(0).Clicked() {
							b.navigate(item.link)
						}
						if item.owner != "" && r.kind != screenUser && ui.Button(row, "投稿者").Clicked() {
							b.navigate(item.owner)
						}
						if item.summary != "" {
							ui.Text(row, strings.TrimSpace(item.summary)).TextColor(t.TextMuted)
						}
					})
				}
			})
		})
	})
}
