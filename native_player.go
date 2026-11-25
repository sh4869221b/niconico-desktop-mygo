package main

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

type nativePlayer struct {
	cancel                context.CancelFunc
	stream                *ui.VideoPlayer
	comments              []timedComment
	status, commentNotice string
	showComments          bool
	recorded              bool
}

func (p *playerWindow) stopNative() {
	if n := p.native; n != nil {
		n.cancel()
		if n.stream != nil {
			n.stream.Close()
		}

		p.native = nil
	}
}

func (b *browser) loadNativePlayer(p *playerWindow, route screenRoute) {
	p.stopNative()
	ctx, cancel := context.WithCancel(context.Background())
	n := &nativePlayer{cancel: cancel, status: "配信を準備しています…", showComments: true}
	p.native, p.target = n, route.link
	p.win.SetTitle(appName + " ・ 独自プレーヤー")
	p.win.Invalidate()
	var session *http.Cookie
	if b.session != nil {
		session, _ = readPlaybackCookie(b.session.Cookies.List, time.Now())
	}
	active := func() bool { return !b.closed && b.player == p && p.native == n }
	go func() {
		req, cancelReq := context.WithTimeout(ctx, 25*time.Second)
		source, err := nativeStream(req, route.key, session)
		cancelReq()
		local := ""
		if err == nil {
			local, _, err = startHLS(ctx, source)
		}
		mygo.RunOnMain(func() {
			if !active() {

				return
			}
			if err != nil {
				n.cancel()
				n.status = "独自プレーヤーで再生できませんでした。 " + err.Error()
			} else if n.stream, err = video.OpenURL(local); err != nil {
				n.cancel()

				n.status = "Windowsの動画再生機能で配信を開けませんでした。"
			} else {
				n.status = ""
				p.title = source.title
				p.win.SetTitle(source.title + " ・ " + appName)
			}
			p.win.Invalidate()
		})
		if err != nil || ctx.Err() != nil {
			return
		}
		comments, err := nativeComments(ctx, source.watch)
		mygo.RunOnMain(func() {
			if !active() {

				return
			}
			n.comments = comments
			if err != nil {
				n.commentNotice = "コメントを取得できませんでした。"
			} else if len(comments) == 0 {
				n.commentNotice = "表示できる日本語コメントはありません。"
			}
			p.win.Invalidate()
		})
	}()
}

func readPlaybackCookie(list func(mygo.CookieFilter) ([]mygo.Cookie, error), now time.Time) (*http.Cookie, error) {
	// Query the app's own WebView2 profile, not an external browser.
	values, err := list(mygo.CookieFilter{URL: home + "api/watch/v3/sm9", Name: "user_session"})
	if err != nil {
		return nil, errSignIn
	}
	for _, value := range values {
		if value.Name != "user_session" || !value.Secure || value.Domain != ".nicovideo.jp" ||
			(!value.Expires.IsZero() && !value.Expires.After(now)) {
			continue
		}
		c := &http.Cookie{Name: value.Name, Value: value.Value, Secure: true, HttpOnly: true}
		if c.Value != "" && c.Valid() == nil {
			return c, nil
		}
	}
	return nil, errSignIn
}

func (b *browser) drawPlayer(p *playerWindow, c *ui.Context) {
	n := p.native
	if n == nil {
		p.draw(c)
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "独自プレーヤー（試験版）").Bold()
			if ui.Button(c, "公式 WebView で開く").Clicked() {
				b.openPlayerMode(p.target, webPlayback)
			}
			if n.stream != nil && ui.Button(c, "コメント切替").Label("コメント表示切替").Clicked() {
				n.showComments = !n.showComments
			}
		})
		s := n.stream
		if s == nil || s.Err() != nil {
			ui.Column(c).Grow(1).FillWidth().Padding(20).Justify(ui.Center).Children(func() {
				msg := n.status
				if s != nil {
					msg = "配信を再生できませんでした。公式 WebView で開き直してください。"
					n.cancel()
				}
				ui.Text(c, msg).Wrap()
			})
			return
		}
		current, duration := s.Position(), s.Duration()
		if s.Ready() && !n.recorded {
			n.recorded = true
			b.recordVideoURL(p.target, p.title)
		}
		view := ui.Video(c, s).Grow(1).FillWidth().MinHeight(0).Fit(ui.Contain).Background(ui.RGB(0, 0, 0))
		if n.showComments && s.Ready() && len(n.comments) > 0 {
			view.DrawOver(func(paint *ui.Painter, r ui.Rect) {
				vw, vh := s.Size()
				drawDanmaku(paint, videoBounds(r, vw, vh), n.comments, current)
			})
		}
		ui.Row(c).Padding(8, 12).Gap(8).AlignItems(ui.Center).Children(func() {
			label := "再生"
			if s.Playing() {
				label = "一時停止"
			}
			if ui.Button(c, label).Disabled(!s.Ready()).Clicked() {
				if s.Playing() {
					s.Pause()
				} else {
					s.Play()
				}
			}
			seconds := current.Seconds()
			if duration > 0 && ui.Slider(c, &seconds, 0, duration.Seconds()).Grow(1).Label("再生位置").Changed() {
				s.Seek(time.Duration(seconds * float64(time.Second)))
			}
			ui.Text(c, formatPlaybackTime(current)+" / "+formatPlaybackTime(duration))
			mute := "ミュート"
			if s.Muted() {
				mute = "ミュート解除"
			}
			if ui.Button(c, mute).Clicked() {
				s.SetMuted(!s.Muted())
			}
		})
		if !s.Ready() || s.Buffering() {
			ui.Text(c, "配信を読み込んでいます…").Padding(4, 12)
		}
		if n.showComments && n.commentNotice != "" {
			ui.Text(c, n.commentNotice).Padding(4, 12)
		}
	})
}

func formatPlaybackTime(d time.Duration) string {
	s := int(max(0, d) / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
func videoBounds(r ui.Rect, w, h int) ui.Rect {
	if w <= 0 || h <= 0 || r.W <= 0 || r.H <= 0 {
		return r
	}
	scale := min(r.W/float32(w), r.H/float32(h))
	width, height := float32(w)*scale, float32(h)*scale
	return ui.Rect{X: r.X + (r.W-width)/2, Y: r.Y + (r.H-height)/2, W: width, H: height}
}

func drawDanmaku(p *ui.Painter, r ui.Rect, cs []timedComment, at time.Duration) {
	if r.W < 10 || r.H < 30 {
		return
	}
	const travel int64 = 4200
	now := at.Milliseconds()
	i, _ := slices.BinarySearchFunc(cs, now-travel, func(c timedComment, t int64) int { return int(c.VposMs - t) })
	size := float32(22)
	lanes := min(14, max(1, int(r.H/(size+6))-2))
	p.Clip(r, 0, func() {
		for j := i; j < len(cs) && j < i+60 && cs[j].VposMs <= now; j++ {
			c := cs[j]
			age := float32(now-c.VposMs) / float32(travel)
			length := float32(len([]rune(c.Body))) * size
			x := r.X + r.W - (r.W+length)*age
			y := r.Y + size + float32(j%lanes)*(size+6)
			if slices.Contains(c.Commands, "ue") {
				x = r.X + max(0, (r.W-length)/2)
				y = r.Y + 3
			}
			if slices.Contains(c.Commands, "shita") {
				x = r.X + max(0, (r.W-length)/2)
				y = r.Y + r.H - size - 4
			}
			color := ui.RGB(255, 255, 255)
			for _, command := range c.Commands {
				switch strings.ToLower(command) {
				case "red":
					color = ui.RGB(255, 74, 74)
				case "yellow":
					color = ui.RGB(255, 240, 0)
				case "blue":
					color = ui.RGB(65, 155, 255)
				case "green":
					color = ui.RGB(0, 238, 80)
				case "pink":
					color = ui.RGB(255, 155, 209)
				}
			}
			p.Text(x+1, y+2, c.Body, size, ui.RGB(0, 0, 0))
			p.Text(x, y, c.Body, size, color)
		}
	})
}
