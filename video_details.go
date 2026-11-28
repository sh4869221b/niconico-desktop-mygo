package main

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type videoDetails struct{ duration, posted, views, comments, publisherKind string }

var videoLengthPattern = regexp.MustCompile(`^[0-9]{1,6}:[0-5][0-9]$`)

func videoLength(raw string) string {
	if videoLengthPattern.MatchString(raw) {
		return raw
	}
	return ""
}
func videoCount(raw string) string {
	if _, err := strconv.ParseUint(raw, 10, 64); err == nil && numeric(raw, 20) {
		return raw
	}
	return ""
}
func videoPostedDate(raw string) string {
	if value, err := time.Parse(time.RFC3339, raw); err == nil {
		return value.Format("2006-01-02 15:04 -0700")
	}
	return ""
}
func (v videoDetails) withFallback(other videoDetails) videoDetails {
	if v.duration == "" {
		v.duration = other.duration
	}
	if v.posted == "" {
		v.posted = other.posted
	}
	if v.views == "" {
		v.views = other.views
	}
	if v.comments == "" {
		v.comments = other.comments
	}
	if v.publisherKind == "" {
		v.publisherKind = other.publisherKind
	}
	return v
}
func (b *browser) rememberVideoDetails(r screenRoute) {
	i := slices.IndexFunc(b.result.items, func(item card) bool { return item.link == r.link })
	if i < 0 {
		return
	}
	if b.knownVideoDetails == nil {
		b.knownVideoDetails = make(map[string]videoDetails)
	}
	previous, cached := b.knownVideoDetails[r.link]
	if !cached && len(b.knownVideoDetails) >= 100 {
		for key := range b.knownVideoDetails {
			delete(b.knownVideoDetails, key)
			break
		}
	}
	item := b.result.items[i]
	details := item.details
	details.publisherKind = videoPublisherKind(item.owner)
	b.knownVideoDetails[r.link] = details.withFallback(previous)
}
func videoDetailText(r screenRoute, p catalogPage) string {
	facts := []string{"動画ID: " + r.key}
	if p.details.duration != "" {
		facts = append(facts, "長さ: "+p.details.duration)
	}
	if p.details.posted != "" {
		facts = append(facts, "投稿日: "+p.details.posted)
	}
	if p.details.views != "" {
		facts = append(facts, "再生: "+p.details.views)
	}
	if p.details.comments != "" {
		facts = append(facts, "コメント: "+p.details.comments)
	}
	kind := videoPublisherKind(p.publisher.videos)
	if kind == "" {
		kind = p.details.publisherKind
	}
	if kind != "" {
		facts = append(facts, "投稿者の種別: "+kind)
	}
	return strings.Join(facts, " · ")
}

func videoPublisherKind(videos string) string {
	if routeFor(videos).kind == screenUser {
		return "ユーザー"
	}
	if u, err := url.Parse(videos); err == nil && u.Hostname() == "ch.nicovideo.jp" {
		return "チャンネル"
	}
	return ""
}
