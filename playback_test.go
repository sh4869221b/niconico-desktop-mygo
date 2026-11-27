package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type decoderErrorFixture struct{ error }

func (decoderErrorFixture) MediaFoundationCodes() (uint32, uint32) { return 4, 0xc00d36c4 }

func TestNativeDecoderFailureShowsOnlySafeDetails(t *testing.T) {
	private := errors.New("could not open http://127.0.0.1:80/signed-playlist")
	const stage = "decoder startup failed"
	if got := nativeDecoderFailure(stage, private); got != stage {
		t.Fatalf("decoder details escaped the status: %q", got)
	}
	if got := nativeDecoderFailure(stage, decoderErrorFixture{private}); got != stage+" (Media Foundation 4, 0xC00D36C4)" {
		t.Fatalf("incorrect Media Foundation details: %q", got)
	}
}

func TestPlaybackPreference(t *testing.T) {
	p := filepath.Join(t.TempDir(), "folder", "playback.json")
	if got, err := loadPlayback(p); got != webPlayback || err != nil {
		t.Fatalf("default %q %v", got, err)
	}
	for _, want := range []playbackMode{nativePlayback, webPlayback} {
		if err := savePlayback(p, want); err != nil {
			t.Fatal(err)
		}
		if got, err := loadPlayback(p); got != want || err != nil {
			t.Fatalf("saved %q %v", got, err)
		}
	}
	if err := os.WriteFile(p, []byte(`{"player":"other"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := loadPlayback(p); got != webPlayback || err == nil {
		t.Fatalf("invalid %q %v", got, err)
	}
}

func TestPlaybackSessionScope(t *testing.T) {
	for _, x := range []struct {
		m, u  string
		allow bool
	}{
		{"GET", "https://www.nicovideo.jp/api/watch/v3/sm9", true},
		{"POST", "https://nvapi.nicovideo.jp/v1/watch/sm9/access-rights/hls", true},
		{"GET", "https://www.nicovideo.jp/api/watch/v3_guest/sm9", false},
		{"POST", "https://nvapi.nicovideo.jp/v1/watch/sm9%2Fextra/access-rights/hls", false},
		{"POST", "https://delivery.domand.nicovideo.jp/v1/watch/sm9/access-rights/hls", false},
		{"POST", "https://nvcomment.nicovideo.jp/v1/threads", false},
		{"GET", "https://user@www.nicovideo.jp/api/watch/v3/sm9", false},
		{"GET", "https://www.nicovideo.jp:80/api/watch/v3/sm9", false},
	} {
		u, err := url.Parse(x.u)
		if err != nil || playbackSessionEndpoint(u, x.m) != x.allow {
			t.Errorf("wrong scope for %s %s", x.m, x.u)
		}
	}
}

func TestRewriteHLS(t *testing.T) {
	base, _ := url.Parse("https://delivery.domand.nicovideo.jp/a/master.m3u8?x=y")
	pass := "#EXTM3U\r\n#EXT-X-MEDIA:TYPE=AUDIO,URI=\"/audio.m3u8?a=b\"\r\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXT-X-KEY:METHOD=AES-128,URI=\"keys/one\"\n#EXTINF:5,\nvideo.m4s\n"
	got, err := rewriteHLS(pass, base, func(u *url.URL) string { return "http://127.0.0.1:99" + u.RequestURI() })
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"/audio.m3u8?a=b", "/a/init.mp4", "/a/keys/one", "/a/video.m4s"} {
		if !strings.Contains(got, "http://127.0.0.1:99"+s) {
			t.Errorf("missing rewritten URI %s", s)
		}
	}
	for _, s := range []string{"https://external.invalid/a", "#EXT-X-KEY:METHOD=AES-128,URI=\"https://external.invalid/a\"", "#EXT-X-DEFINE:NAME=\"x\",VALUE=\"y\""} {
		if _, err := rewriteHLS("#EXTM3U\n"+s+"\n", base, func(u *url.URL) string { return u.String() }); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}

type streamTransportFunc func(*http.Request) (*http.Response, error)

func (f streamTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHLSRelayAuthorization(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	target := "https://" + deliveryHost + "/vod/master.m3u8"
	local, relay, err := startHLS(ctx, nativeSource{playlist: target, delivery: []*http.Cookie{{Name: "domand_bid", Value: "delivery-test"}, {Name: "user_session", Value: "must-not-send"}}})
	if err != nil {
		t.Fatal(err)
	}
	called := make(chan bool, 1)
	relay.client.Transport = streamTransportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != target || r.Header.Get("Cookie") != "domand_bid=delivery-test" {
			t.Errorf("wrong upstream request or cookie")
		}
		called <- true
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("#EXTM3U\nsegment.m4s\n"))}, nil
	})
	response, err := http.Get(local)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(string(b), "http://"+relay.host+"/") {
		t.Fatalf("invalid response %d", response.StatusCode)
	}
	<-called
	u, _ := url.Parse(local)
	u.Path = "/tampered" + u.Path
	response, err = http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Errorf("tampered response %d", response.StatusCode)
	}
}

func TestHLSRelayExtensionlessPlaylist(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	local, relay, err := startHLS(ctx, nativeSource{playlist: "https://" + deliveryHost + "/vod/manifest?ticket=fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(local, ".m3u8") {
		t.Fatal("Media Foundation would receive a URL without the playlist extension")
	}
	relay.client.Transport = streamTransportFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/vod/manifest":
			if r.URL.Query().Get("ticket") != "fixture" || r.Header.Get("Range") != "" {
				t.Error("expected the signed upstream URL and a complete playlist request")
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/octet-stream"}},
				Body: io.NopCloser(strings.NewReader("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=100000\nvideo.m3u8\n"))}, nil
		case "/vod/video.m3u8":
			return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("fixture rejected"))}, nil
		}
		t.Errorf("unexpected relay request: %s", r.URL.Path)
		return nil, context.Canceled
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, local, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Range", "bytes=0-15")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || response.Header.Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Fatalf("playlist response %d, read error %v", response.StatusCode, err)
	}
	lines := strings.Split(string(body), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[2], "http://"+relay.host+"/") {
		t.Fatalf("variant was not routed through the relay: %q", body)
	}
	response, err = http.Get(lines[2])
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatalf("upstream denial was changed to %d", response.StatusCode)
	}
	n := &nativePlayer{cancel: cancel, relay: relay}
	n.failStream("Windows の動画再生機能で最初の映像を準備できませんでした。")
	if !strings.Contains(n.status, "プレイリストを取得できませんでした (HTTP 403)") ||
		strings.Contains(n.status, "fixture") || !strings.Contains(n.status, "公式 WebView") {
		t.Fatalf("incorrect or unsanitized playback status: %q", n.status)
	}
}
