package main

import (
	"bytes"
	"context"
	"image"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	thumbnailWorkers      = 4
	thumbnailCacheSize    = 128
	thumbnailByteLimit    = 1 << 20
	thumbnailMaxDimension = 1920
)

func normalizeThumbnailURL(raw string) string {
	if len(raw) > 2048 {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Opaque != "" || u.Fragment != "" ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	switch u.Host {
	case "nicovideo.cdn.nimg.jp":
		if !strings.HasPrefix(u.Path, "/thumbnails/") {
			return ""
		}
	case "img.cdn.nimg.jp":
		if !strings.HasPrefix(u.Path, "/s/nicovideo/thumbnails/") {
			return ""
		}
	default:
		return ""
	}
	u.Scheme = "https"
	return u.String()
}

type thumbnailEntry struct {
	image   *ui.Bitmap
	retryAt time.Time
	used    uint64
}

// The viewport and the cache are owned by the UI thread. Only I/O and
// decoding run in the background, with their results posted to the UI.
type thumbnails struct {
	client     *http.Client
	wanted     []string
	active     map[string]context.CancelFunc
	cache      map[string]thumbnailEntry
	access     uint64
	invalidate func()
}

func newThumbnails(invalidate func()) *thumbnails {
	return &thumbnails{
		client: &http.Client{
			Timeout:       8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		active:     make(map[string]context.CancelFunc),
		cache:      make(map[string]thumbnailEntry),
		invalidate: invalidate,
	}
}

func (s *thumbnails) setVisible(items []card) {
	s.wanted = s.wanted[:0]
	for _, item := range items {
		if target := item.thumbnail; target != "" && !slices.Contains(s.wanted, target) {
			s.wanted = append(s.wanted, target)
		}
	}
	for target, cancel := range s.active {
		if !slices.Contains(s.wanted, target) {
			cancel()
		}
	}
	s.start()
}

func (s *thumbnails) start() {
	for _, target := range s.wanted {
		if len(s.active) >= thumbnailWorkers {
			return
		}
		if _, running := s.active[target]; running {
			continue
		}
		if entry, cached := s.cache[target]; cached && (entry.image != nil || time.Now().Before(entry.retryAt)) {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		s.active[target] = cancel
		go func() {
			img := fetchThumbnail(ctx, s.client, target)
			mygo.RunOnMain(func() {
				delete(s.active, target)
				if ctx.Err() == nil {
					s.store(target, img)
					s.invalidate()
				}
				cancel()
				s.start()
			})
		}()
	}
}

func (s *thumbnails) store(target string, img *ui.Bitmap) {
	s.access++
	s.cache[target] = thumbnailEntry{img, time.Now().Add(time.Minute), s.access}
	if len(s.cache) > thumbnailCacheSize {
		var oldest string
		for key, entry := range s.cache {
			if oldest == "" || entry.used < s.cache[oldest].used {
				oldest = key
			}
		}
		delete(s.cache, oldest)
	}
}

func (s *thumbnails) image(target string) *ui.Bitmap {
	entry, ok := s.cache[target]
	if !ok {
		return nil
	}
	s.access++
	entry.used = s.access
	s.cache[target] = entry
	return entry.image
}

func fetchThumbnail(ctx context.Context, client *http.Client, target string) *ui.Bitmap {
	if target == "" || normalizeThumbnailURL(target) != target {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/gif")
	res, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.ContentLength > thumbnailByteLimit {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, thumbnailByteLimit+1))
	if err != nil || len(data) > thumbnailByteLimit {
		return nil
	}
	info, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || info.Width <= 0 || info.Height <= 0 ||
		info.Width > thumbnailMaxDimension || info.Height > thumbnailMaxDimension {
		return nil
	}
	img, _ := ui.DecodeBitmapScaled(data, 320)
	return img
}
