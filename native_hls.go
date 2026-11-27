package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func deliveryURL(u *url.URL) bool {
	return u != nil && u.Scheme == "https" && (u.Host == deliveryHost || u.Host == "asset.domand.nicovideo.jp" && (path.Ext(u.Path) == ".cmfa" || path.Ext(u.Path) == ".cmfv")) && u.User == nil && u.Fragment == "" && u.Opaque == "" &&
		strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(u.Path, "//") && !strings.Contains(u.Path, "\\")
}

type hlsRelay struct {
	server      *http.Server
	host        string
	key         [32]byte
	cookies     []*http.Cookie
	client      *http.Client
	rootContext context.Context
}

func startHLS(ctx context.Context, source nativeSource) (string, *hlsRelay, error) {
	u, err := url.Parse(source.playlist)
	if err != nil || !deliveryURL(u) {
		return "", nil, errors.New("invalid HLS address")
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	p := &hlsRelay{host: ln.Addr().String(), cookies: source.delivery, client: streamClient(nil), rootContext: ctx}
	if _, err := rand.Read(p.key[:]); err != nil {
		ln.Close()
		return "", nil, err
	}
	p.server = &http.Server{Handler: p, ReadHeaderTimeout: 3 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 20 * time.Second}
	go p.server.Serve(ln)
	go func() { <-ctx.Done(); p.server.Close() }()
	return p.local(u), p, nil
}

func (p *hlsRelay) token(raw string) string {
	m := hmac.New(sha256.New, p.key[:])
	m.Write([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (p *hlsRelay) local(u *url.URL) string {
	// The MAC authorizes only URLs seen in the upstream playlist and expires
	// when this playback closes. No credentials are passed to Media Foundation.
	raw := u.String()
	ext := path.Ext(u.Path)
	switch ext {
	case ".m3u8", ".mp4", ".m4s", ".ts", ".aac", ".cmfa", ".cmfv", ".key":
	default:
		ext = ".bin"
	}
	return "http://" + p.host + "/" + p.token(raw) + "/" + base64.RawURLEncoding.EncodeToString([]byte(raw)) + ext
}

func (p *hlsRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Host != p.host || (r.Method != http.MethodGet && r.Method != http.MethodHead) || r.URL.RawQuery != "" || len(r.URL.Path) > 16000 {
		http.NotFound(w, r)
		return
	}
	token, encoded, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSuffix(encoded, path.Ext(encoded)))
	if !ok || err != nil || !hmac.Equal([]byte(token), []byte(p.token(string(raw)))) {
		http.NotFound(w, r)
		return
	}
	target, err := url.Parse(string(raw))
	if err != nil || !deliveryURL(target) {
		http.NotFound(w, r)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), nil)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	request.Header.Set("Origin", "https://"+watchHost)
	request.Header.Set("Referer", home)
	if v := r.Header.Get("Range"); v != "" && path.Ext(target.Path) != ".m3u8" {
		request.Header.Set("Range", v)
	}
	for _, c := range p.cookies {
		if target.Host == deliveryHost && c.Name == "domand_bid" {
			request.AddCookie(c)
		}
	}
	res, err := p.client.Do(request)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusPartialContent {
		if res.StatusCode == http.StatusForbidden {
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if path.Ext(target.Path) == ".m3u8" || strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "mpegurl") {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		if r.Method == http.MethodHead {
			return
		}
		const maxPlaylist = 2 << 20
		body, err := io.ReadAll(io.LimitReader(res.Body, maxPlaylist+1))
		if err != nil || len(body) > maxPlaylist {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		modified, err := rewriteHLS(string(body), target, p.local)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(modified)))
		io.WriteString(w, modified)
		return
	}
	for _, k := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := res.Header.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
	w.WriteHeader(res.StatusCode)
	if r.Method == http.MethodGet {
		io.Copy(w, res.Body)
	}
}

var hlsURI = regexp.MustCompile(`\bURI="([^"]+)"`)

func rewriteHLS(body string, base *url.URL, route func(*url.URL) string) (string, error) {
	if !strings.HasPrefix(body, "#EXTM3U") {
		return "", errors.New("not an HLS playlist")
	}
	lines := strings.Split(body, "\n")
	resolve := func(ref string) (string, error) {
		u, err := base.Parse(ref)
		if err != nil || !deliveryURL(u) {
			return "", errors.New("unsupported HLS playlist address")
		}
		return route(u), nil
	}
	for i, line := range lines {
		line = strings.TrimSpace(line)
		var err error
		switch {
		case strings.HasPrefix(line, "#EXT-X-DEFINE:") || strings.HasPrefix(line, "#EXT-X-CONTENT-STEERING:"):
			return "", errors.New("unsupported HLS indirection")
		case strings.HasPrefix(line, "#"):
			// URI attributes include alternate audio, init segments, and HLS keys.
			line = hlsURI.ReplaceAllStringFunc(line, func(s string) string {
				if err != nil {
					return ""
				}
				var u string
				u, err = resolve(s[5 : len(s)-1])
				return "URI=\"" + u + "\""
			})
		case line != "":
			line, err = resolve(line)
		}
		if err != nil {
			return "", err
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n"), nil
}
