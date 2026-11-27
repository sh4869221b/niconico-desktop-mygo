package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
