package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

const watchHost = "www.nicovideo.jp"
const deliveryHost = "delivery.domand.nicovideo.jp"

type mediaTrack struct {
	ID           string
	IsAvailable  bool
	QualityLevel int
}
type watchData struct {
	Video  struct{ ID, Title, Description string }
	Tags   *struct{ Items []struct{ Name string } }
	Client struct{ WatchTrackID string }
	Media  struct {
		Domand struct {
			Videos, Audios []mediaTrack
			AccessRightKey string
		}
	}
	Comment struct {
		NVComment struct {
			Server, ThreadKey string
			Params            json.RawMessage
		}
	}
}
type nativeSource struct {
	title, playlist string
	delivery        []*http.Cookie
	watch           watchData
}
type timedComment struct {
	Body     string
	VposMs   int64
	Commands []string
	UserID   json.RawMessage
	author   commentAuthor
}

func streamClient(jar http.CookieJar) *http.Client {
	return &http.Client{Jar: jar, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func streamJSON[T any](ctx context.Context, h *http.Client, target string, payload any, mime string, headers map[string]string, session *http.Cookie) (T, error) {
	var zero T
	method := http.MethodGet
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return zero, err
		}
		method, body = http.MethodPost, bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return zero, errors.New("再生リクエストを作成できません")
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "ja-JP,ja;q=0.9")
	req.Header.Set("X-Niconico-Language", "ja-jp")
	req.Header.Set("X-Frontend-ID", "6")
	req.Header.Set("X-Frontend-Version", "0")
	req.Header.Set("Referer", home)
	req.Header.Set("Origin", "https://"+watchHost)
	req.Header.Set("User-Agent", "NiconicoDesktopMyGo/native-player")
	if body != nil {
		req.Header.Set("Content-Type", mime)
		req.Header.Set("Origin", "https://"+watchHost)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// The application session is attached only to these two first-party API
	// calls; never put it in a cookie jar, the decoder, or the comment request.
	if session != nil && playbackSessionEndpoint(req.URL, method) {
		req.AddCookie(session)
	}
	res, err := h.Do(req)
	if err != nil {
		return zero, errors.New("ニコニコと通信できませんでした")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return zero, fmt.Errorf("ニコニコがリクエストを拒否しました (HTTP %d)", res.StatusCode)
	}
	const limit = 8 << 20
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || len(data) > limit {
		return zero, errors.New("再生情報を読み取れませんでした")
	}
	var out struct {
		Meta struct{ Status int }
		Data T
	}
	if err = json.Unmarshal(data, &out); err != nil || out.Meta.Status < 200 || out.Meta.Status >= 300 {
		return zero, errors.New("ニコニコから有効な再生情報が得られませんでした")
	}
	return out.Data, nil
}
func playbackSessionEndpoint(u *url.URL, method string) bool {
	if u == nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	if u.Host == watchHost && method == http.MethodGet {
		id, ok := strings.CutPrefix(u.Path, "/api/watch/v3/")
		return ok && watchSegment.MatchString(id)
	}
	if u.Host == apiHost && method == http.MethodPost {
		id, ok := strings.CutPrefix(u.Path, "/v1/watch/")
		id, ok2 := strings.CutSuffix(id, "/access-rights/hls")
		return ok && ok2 && watchSegment.MatchString(id)
	}
	return false
}

func nativeStream(ctx context.Context, id string, session *http.Cookie) (nativeSource, error) {
	var result nativeSource
	if !watchSegment.MatchString(id) {
		return result, errors.New("無効な動画IDです")
	}
	jar, _ := cookiejar.New(nil)
	h := streamClient(jar)
	variant := "v3_guest"
	if session != nil {
		variant = "v3"
	}
	q := url.Values{"actionTrackId": {"AAAAAAAAAA_" + strconv.FormatInt(time.Now().UnixMilli(), 10)}}
	w, err := streamJSON[watchData](ctx, h, home+"api/watch/"+variant+"/"+id+"?"+q.Encode(), nil, "", nil, session)
	if err != nil {
		return result, err
	}
	m := w.Media.Domand
	best := func(xs []mediaTrack) string {
		var selected mediaTrack
		for _, x := range xs {
			if x.IsAvailable && x.ID != "" && (selected.ID == "" || x.QualityLevel > selected.QualityLevel) {
				selected = x
			}
		}
		return selected.ID
	}
	v, a := best(m.Videos), best(m.Audios)
	if v == "" || a == "" || m.AccessRightKey == "" || w.Client.WatchTrackID == "" {
		return result, errors.New("独自プレーヤーで利用できる配信がありません")
	}
	q.Set("actionTrackId", w.Client.WatchTrackID)
	stream, err := streamJSON[struct{ ContentURL string }](ctx, h, "https://"+apiHost+"/v1/watch/"+id+"/access-rights/hls?"+q.Encode(),
		map[string]any{"outputs": [][2]string{{v, a}}}, "application/json", map[string]string{"X-Access-Right-Key": m.AccessRightKey, "X-Request-With": "https://" + watchHost}, session)
	if err != nil {
		return result, err
	}
	u, err := url.Parse(stream.ContentURL)
	if err != nil || !deliveryURL(u) || u.Host != deliveryHost {
		return result, errors.New("未対応の配信アドレスです")
	}
	for _, c := range jar.Cookies(u) {
		if c.Name == "domand_bid" && c.Value != "" && c.Valid() == nil {
			result.delivery = append(result.delivery, c)
		}
	}
	if len(result.delivery) == 0 {
		return result, errors.New("再生用の配信許可を取得できませんでした")
	}
	result.title, result.playlist, result.watch = w.Video.Title, u.String(), w
	return result, nil
}

func nativeComments(ctx context.Context, w watchData) ([]timedComment, error) {
	n := w.Comment.NVComment
	u, err := url.Parse(n.Server)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Path != "" && u.Path != "/") ||
		(u.Host != "public.nvcomment.nicovideo.jp" && u.Host != "nvcomment.nicovideo.jp" && u.Host != "nv-comment.nicovideo.jp") ||
		len(n.Params) == 0 || n.ThreadKey == "" {
		return nil, errors.New("コメント配信の情報がありません")
	}
	var params map[string]json.RawMessage
	if err := json.Unmarshal(n.Params, &params); err != nil || params == nil {
		return nil, errors.New("コメント設定を読み取れませんでした")
	}
	params["language"] = json.RawMessage(`"ja-jp"`)
	u.Path = "/v1/threads"
	payload := map[string]any{"params": params, "threadKey": n.ThreadKey, "additionals": map[string]any{}}
	rows, err := streamJSON[struct {
		Threads []struct{ Comments []timedComment }
	}](ctx, streamClient(nil), u.String(), payload,
		"text/plain;charset=UTF-8", map[string]string{"X-Client-Os-Type": "others"}, nil)
	if err != nil {
		return nil, err
	}
	out := make([]timedComment, 0, 1024)
	for _, t := range rows.Threads {
		for _, c := range t.Comments {
			if c.VposMs < 0 || c.VposMs > 86_400_000 || strings.TrimSpace(c.Body) == "" || len(out) >= 15000 {
				continue
			}
			c.Body = cleanDanmakuText(c.Body)
			if c.Body == "" {
				continue
			}
			c.author = authorForComment(w.Video.ID, c.UserID, c.Commands)
			c.UserID = nil
			out = append(out, c)
		}
	}
	slices.SortStableFunc(out, func(a, b timedComment) int { return int(a.VposMs - b.VposMs) })
	return out, nil
}
