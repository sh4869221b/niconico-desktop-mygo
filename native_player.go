package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

type nativePlayer struct {
	cancel                context.CancelFunc
	stream                *ui.VideoPlayer
	relay                 *hlsRelay
	startDeadline         time.Duration
	loadingStream         bool
	pauseRequested        bool
	stagedComments        *nativeCommentData
	comments              []danmakuCue
	danmaku               danmakuOverlay
	commentViewport       ui.Rect
	status, commentNotice string
	startAt               time.Duration
	showComments          bool
	recorded              bool
	hideCommentPanel      bool
	showOverview          bool
	description           []descriptionPart
	descriptionSource     string
	publisher             videoPublisher
	allComments           []timedComment
	allCues               []danmakuCue
	commentRows           []int
	commentList           ui.ListState
	manageNG              bool
	ngNotice              string
	pendingNG             commentNGEdit
	follow                commentFollow
	tags                  []string
	tagNotice             string
	showKeys              bool
	displayPending        bool

	metadataCtx                      context.Context
	metadataRoute                    screenRoute
	metadataWatch                    watchData
	tagLoading, tagFailed            bool
	commentsLoading, commentsFailed  bool
	tagGeneration, commentGeneration uint64
}

const defaultNativeVolume = 80
const nativeStartTimeout = 20 * time.Second

type nativeCommentData struct {
	rows   []timedComment
	cues   []danmakuCue
	notice string
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
	n := &nativePlayer{
		descriptionSource: route.target, cancel: cancel, loadingStream: true, status: "配信を準備しています…", startAt: route.startTime(),
		showComments: true, displayPending: true, commentNotice: "コメントを読み込んでいます…", tagNotice: "タグを読み込んでいます…",
	}
	p.native, p.target = n, route.target
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
		var relay *hlsRelay
		if err == nil {
			local, relay, err = startHLS(ctx, source)
			if err != nil {
				err = errors.New("配信のローカル中継を開始できませんでした。")
			}
		}
		mygo.RunOnMain(func() {
			if !active() {
				return
			}
			n.loadingStream, n.relay = false, relay
			if err != nil {
				n.failStream(err.Error())
			} else if n.stream, err = video.OpenURL(local); err != nil {
				n.failStream("Windows の動画再生機能で配信を開けませんでした。")
			} else {
				b.applyNativeAudio(n.stream)
				if n.stream.PrepareStart(n.startAt) {
					n.startAt = 0
				}
				if n.pauseRequested {
					n.stream.Pause()
				}
				n.status = ""
				p.title = source.title
				p.win.SetTitle(source.title + " ・ " + appName)
			}
			p.win.Invalidate()
		})
		if err != nil || ctx.Err() != nil {
			return
		}
		_, _, description := plainDescription(source.watch.Video.Description)
		mygo.RunOnMain(func() {
			if active() && ctx.Err() == nil {
				n.metadataCtx, n.metadataRoute, n.metadataWatch = ctx, route, source.watch
				n.description = description
				p.win.Invalidate()
				b.fetchNativeTags(p, n)
				b.fetchNativeComments(p, n)
			}
		})
	}()
}

func (n *nativePlayer) failStream(message string) {
	if n.relay != nil {
		if reason := n.relay.lastFailure.Load(); reason != nil {
			message += " 中継で記録した直近のエラーは「" + *reason + "」です。"
		}
		n.relay = nil
	}
	n.cancel()
	if n.stream != nil {
		n.stream.Close()
		n.stream = nil
	}
	n.status = message + " 公式 WebView で開き直してください。"
}

func nativeDecoderFailure(message string, err error) string {
	var failure interface{ MediaFoundationCodes() (uint32, uint32) }
	if errors.As(err, &failure) {
		code, extended := failure.MediaFoundationCodes()
		message += fmt.Sprintf(" (Media Foundation %d, 0x%08X)", code, extended)
	}
	return message
}

// A startup deadline only requests a view rebuild. The current player checks
// it on that frame, after any visibility change, so an old deadline cannot
// close a replacement stream.
func (n *nativePlayer) startExpired(c *ui.Context) bool {
	elapsed, hidden := c.WindowVisibleTime()
	if n.startDeadline == 0 {
		n.startDeadline = elapsed + nativeStartTimeout
	}
	if hidden {
		return false
	}
	remaining := n.startDeadline - elapsed
	if remaining <= 0 {
		return true
	}
	c.After(remaining)
	return false
}

func (n *nativePlayer) installComments(blocked map[commentAuthor]bool) {
	if data := n.stagedComments; data != nil && n.stream.Ready() && !n.stream.Preparing() {
		n.stagedComments = nil
		n.allComments, n.allCues, n.commentNotice = data.rows, data.cues, data.notice
		n.applyCommentNG(blocked)
	}
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
	c.SetTheme(readableAppTheme(c.Theme()))
	n := p.native
	if n == nil {
		p.draw(c, func(mode playbackMode) { b.recoverPlayer(p, mode) })
		return
	}
	if n.displayPending {
		b.applyNativeDisplay(n, c.Preferences().ReduceMotion)
		n.displayPending = false
	}
	t := c.Theme()
	w, h := c.Size()
	b.applyPendingCommentNG(n)
	ui.Column(c).Fill().Background(t.Surface).Children(func() {
		ui.Scroll(c.Key("player-toolbar")).FillWidth().MaxHeight(h * 0.15).Shrink(0).Children(func() {
			ui.Row(c).FillWidth().Padding(8, 12).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "独自プレーヤー（試験版）").Bold()
				fullscreen := "全画面"
				if p.win != nil && p.win.IsFullScreen() {
					fullscreen = "全画面を解除"
				}
				fullscreenButton := ui.Button(c, fullscreen).Tooltip("F11: 全画面切替 / Esc: 解除").Disabled(p.win == nil)
				if playerToggleButton(c, fullscreenButton) {
					p.win.ToggleFullScreen()
				}
				if ui.Button(c, "操作キー").Checked(n.showKeys).Clicked() {
					n.showKeys = !n.showKeys
				}
				if ui.Button(c, "公式 WebView で開く").Clicked() {
					b.openPlayerMode(p.target, webPlayback)
				}
				if n.stream != nil && ui.Button(c, "弾幕を表示").Checked(n.showComments).Clicked() {
					n.showComments = !n.showComments
					b.nativeDisplay.Danmaku = "hide"
					if n.showComments {
						b.nativeDisplay.Danmaku = "show"
					}
					b.saveNativeDisplay()
				}
				if n.stream != nil && ui.Button(c, "コメント一覧").Checked(!n.hideCommentPanel).Clicked() {
					n.hideCommentPanel = !n.hideCommentPanel
					b.nativeDisplay.HideComments = n.hideCommentPanel
					b.saveNativeDisplay()
					n.follow.positioned = false
				}
				p.drawSharing(c, nil)
				if n.loadingStream {
					label := "一時停止"
					if n.pauseRequested {
						label = "再生"
					}
					if playerToggleButton(c, ui.Button(c, label).Tooltip("Space: 再生・一時停止")) {
						n.pauseRequested = !n.pauseRequested
					}
				}
			})
			if n.showKeys {
				ui.Text(c, "Space: 再生・一時停止 / M: ミュート / F11: 全画面切替 / Esc: 全画面解除。フォーカス中のボタン・コメント一覧・メニュー・スライダーの操作を優先します。").Wrap()
			}
			if b.nativeDisplayNotice != "" {
				ui.Text(c, b.nativeDisplayNotice).Role(ui.RoleStatus).Wrap()
			}
		})
		s := n.stream
		if s != nil {
			switch decodeErr := s.Err(); {
			case decodeErr != nil:
				message := "Windows の動画再生機能で最初の映像を準備できませんでした。"
				if s.Ready() && !s.Preparing() {
					message = "Windows の動画再生機能で再生が止まりました。"
				}
				n.failStream(nativeDecoderFailure(message, decodeErr))
			case (s.Preparing() || !s.Ready()) && n.startExpired(c):
				if s.Ready() {
					n.failStream("最初の映像を取得しましたが、表示が完了しませんでした。")
				} else {
					n.failStream("制限時間内に最初の映像を取得できませんでした。")
				}
			}
			s = n.stream
		}
		if s == nil {
			ui.Column(c).Grow(1).FillWidth().Padding(20).Justify(ui.Center).Children(func() {
				ui.Text(c, n.status).Wrap()
			})
			return
		}
		duration := s.Duration()
		if s.Ready() && !s.Preparing() {
			n.installComments(b.commentNG.blocked)
			if n.startAt > 0 && duration > 0 {
				s.Seek(min(n.startAt, duration))
				n.startAt = 0
			}
			if !n.recorded {
				n.recorded = true
				b.recordVideo(p, p.target, p.title)
			}
		}
		current := s.Position()
		compact := w < 780
		var body ui.Element
		if compact {
			body = ui.Column(c)
		} else {
			body = ui.Row(c)
		}
		body.FillWidth().Grow(1).MinHeight(0).Gap(8).Padding(0, 8).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
				b.drawNativeVideoInfo(c, n, h)
				drawNativeVideo(c, n, current)
			})
			if n.hideCommentPanel {
				return
			}
			panel := ui.Column(c).Padding(6, 8).Gap(5).Border(1, t.Border).Radius(6).Background(t.Background)
			if compact {
				commentsHeight := h * 0.21
				if h < 520 {
					commentsHeight = h * 0.16
				}
				panel.Height(min(144, commentsHeight)).Shrink(0).FillWidth()
			} else {
				panel.Width(max(246, min(340, w*0.30))).FillHeight()
			}
			panel.Children(func() { b.drawNativeComments(c, n, current) })
		})
		ui.Scroll(c.Key("player-controls")).FillWidth().MaxHeight(h * 0.15).Shrink(0).Children(func() {
			ui.Row(c).FillWidth().Padding(8, 12).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
				label := "再生"
				if s.Playing() {
					label = "一時停止"
				}
				if playerToggleButton(c, ui.Button(c, label).Tooltip("Space: 再生・一時停止")) {
					toggleNativePlayback(s)
				}
				seconds := current.Seconds()
				if duration > 0 && ui.Slider(c, &seconds, 0, duration.Seconds()).Grow(1).MinWidth(50).Label("再生位置").Value(formatPlaybackTime(current)).Tooltip("矢印: 5秒移動 / Page Up・Down: 30秒移動 / Home・End: 先頭・末尾").Disabled(s.Preparing()).KeyboardStep(5, 30).Changed() {
					s.Seek(time.Duration(seconds * float64(time.Second)))
					n.follow.positioned = false
				}
				ui.Text(c, formatPlaybackTime(current)+" / "+formatPlaybackTime(duration))
				volume := s.Volume() * 100
				if ui.Slider(c, &volume, 0, 100).Step(1).Width(94).Label("音量").Changed() {
					b.nativeVolume = volume
					s.SetVolume(volume / 100)
					b.saveNativeAudio()
				}
				ui.Text(c, fmt.Sprintf("音量 %d%%", int(volume+0.5))).Width(68).TextAlign(ui.End)
				mute := "ミュート"
				if s.Muted() {
					mute = "ミュート解除"
				}
				if playerToggleButton(c, ui.Button(c, mute).Tooltip("M: ミュート切替")) {
					b.toggleNativeMute(s)
				}
			})
		})
		if b.nativeAudioNotice != "" {
			ui.Text(c, b.nativeAudioNotice).Role(ui.RoleStatus).Wrap()
		}
	})
	// Drawing the video first synchronizes the engine's latest state. The
	// final frame can then record completion even if no next frame is scheduled.
	b.recordPlaybackProgress(p, c.Now(), false)
	if !c.FocusTakesText() && !n.commentList.FocusWithin(c) {
		if playerToggleShortcut(c, ui.KeySpace) {
			if n.stream != nil {
				toggleNativePlayback(n.stream)
			} else if n.loadingStream {
				n.pauseRequested = !n.pauseRequested
			}
		}
		if n.stream != nil && playerToggleShortcut(c, ui.KeyM) {
			b.toggleNativeMute(n.stream)
		}
	}
	if p.win != nil {
		if playerToggleShortcut(c, ui.KeyF11) {
			p.win.ToggleFullScreen()
		}
		if p.win.IsFullScreen() && playerToggleShortcut(c, ui.KeyEscape) {
			p.win.SetFullScreen(false)
		}
	}
}

func playerToggleShortcut(c *ui.Context, key ui.Key) bool {
	return c.Shortcut(0, key) && !c.ShortcutRepeated()
}

func playerToggleButton(c *ui.Context, button ui.Element) bool {
	// Handle activation keys before the button turns them into clicks.
	space := button.Shortcut(0, ui.KeySpace) && !c.ShortcutRepeated()
	enter := button.Shortcut(0, ui.KeyEnter) && !c.ShortcutRepeated()
	return button.Clicked() || space || enter
}

func (b *browser) toggleNativeMute(s *ui.VideoPlayer) {
	s.SetMuted(!s.Muted())
	b.nativeMuted = s.Muted()
	b.saveNativeAudio()
}

func toggleNativePlayback(s *ui.VideoPlayer) {
	if s.Playing() {
		s.Pause()
	} else {
		s.Play()
	}
}

func drawNativeVideo(c *ui.Context, n *nativePlayer, current time.Duration) {
	s := n.stream
	ui.Box(c).Grow(1).MinWidth(0).MinHeight(0).Children(func() {
		vw, vh := s.Size()
		view := ui.Video(c, s).Fill().Fit(ui.Contain).Background(ui.RGB(0, 0, 0))
		if s.Ready() && vw > 0 && vh > 0 {
			view.HitTest(func(x, y float32, r ui.Rect) bool {
				picture := videoBounds(r, vw, vh)
				return x >= picture.X && x < picture.X+picture.W && y >= picture.Y && y < picture.Y+picture.H
			}).Cursor(ui.CursorPointer)
			if view.Clicked() {
				toggleNativePlayback(s)
			}
		}
		if n.showComments && s.Ready() && !s.Preparing() && len(n.comments) > 0 {
			// Use the actual laid-out video size, including during resize.
			view.DrawOver(func(_ *ui.Painter, r ui.Rect) {
				r.X, r.Y = 0, 0
				if n.commentViewport != r {
					n.commentViewport = r
					c.Invalidate()
				}
			})
			r := n.commentViewport
			n.danmaku.draw(c, danmakuBounds(r, videoBounds(r, vw, vh)), n.comments, current.Milliseconds())
		}
	})
}

func (b *browser) drawNativeVideoInfo(c *ui.Context, n *nativePlayer, height float32) {
	// Cap each band against the window height even when controls wrap at large
	// text sizes. The video gets the remaining space.
	infoHeight, overviewHeight := height*0.17, height*0.18
	if height < 520 {
		// At the smallest size the metadata and overview remain scrollable.
		// Leave room for the video even when both are open and text wraps.
		infoHeight, overviewHeight = height*0.13, height*0.13
	}
	ui.Scroll(c.Key("video-info")).FillWidth().MaxHeight(min(110, infoHeight)).Shrink(0).Children(func() {
		ui.Column(c).FillWidth().Gap(4).Padding(3, 0).Children(func() {
			ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
				label := "概要欄を表示"
				if n.showOverview {
					label = "概要欄を非表示"
				}
				if ui.Button(c.Key("overview-toggle"), label).Checked(n.showOverview).Clicked() {
					n.showOverview = !n.showOverview
					b.nativeDisplay.ShowOverview = n.showOverview
					b.saveNativeDisplay()
				}
				if p := n.publisher; p != (videoPublisher{}) {
					if p.name != "" {
						ui.Text(c, p.name).SingleLine().Ellipsis("…").MaxWidth(220).MinWidth(0).Focusable().Tooltip(p.name)
					}
					if p.videos != "" && ui.Button(c, "投稿動画").Label(p.linkLabel()).Clicked() {
						b.browseInMain(p.videos)
					}
					if target := p.seriesURL(); target != "" && ui.Button(c, "シリーズ").Clicked() {
						b.browseInMain(target)
					}
				}
				if s := n.stream; !s.Ready() || s.Preparing() || s.Buffering() {
					ui.Text(c, "配信を読み込んでいます…").Wrap()
				}
			})
			if n.tagNotice != "" {
				b.drawVideoTags(c, n.tags, n.tagNotice)
			}
			if n.tagFailed && ui.Button(c, "タグを再取得").Disabled(n.tagLoading).Clicked() {
				b.fetchNativeTags(b.player, n)
			}
		})
	})
	if n.showOverview {
		ui.Scroll(c.Key("overview")).FillWidth().MaxHeight(min(160, overviewHeight)).Shrink(0).Focusable().Label("概要欄").Children(func() {
			ui.Column(c).FillWidth().Padding(6, 8).Background(c.Theme().Background).Children(func() {
				if len(n.description) == 0 {
					ui.Text(c, "概要はありません。").Wrap()
				} else {
					b.drawDescription(c, n.description, n.descriptionSource)
				}
			})
		})
	}
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
