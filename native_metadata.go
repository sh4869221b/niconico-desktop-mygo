package main

import (
	"context"
	"time"

	"github.com/egoist/mygo"
)

func (b *browser) acceptsNativeMetadata(p *playerWindow, n *nativePlayer, tag bool, generation uint64) bool {
	if b.closed || p == nil || b.player != p || p.native != n || n.metadataCtx == nil || n.metadataCtx.Err() != nil || routeFor(p.target).link != n.metadataRoute.link {
		return false
	}
	if tag {
		return generation == n.tagGeneration
	}
	return generation == n.commentGeneration
}

func (b *browser) fetchNativeTags(p *playerWindow, n *nativePlayer) {
	if n.tagLoading || !b.acceptsNativeMetadata(p, n, true, n.tagGeneration) {
		return
	}
	n.tagGeneration++
	generation := n.tagGeneration
	n.tagLoading = true
	n.tagNotice = "タグを読み込んでいます…"
	go func() {
		ctx, cancel := context.WithTimeout(n.metadataCtx, 25*time.Second)
		defer cancel()
		tags, publisher, err := playbackDetails(ctx, b.catalog, n.metadataRoute, n.metadataWatch)
		mygo.RunOnMain(func() {
			if !b.acceptsNativeMetadata(p, n, true, generation) {
				return
			}
			n.tagLoading, n.tagFailed = false, err != nil
			n.tags, n.publisher, n.tagNotice = tags, publisher, "タグはありません。"
			if err != nil {
				n.tagNotice = "タグを取得できませんでした。"
			}
			p.win.Invalidate()
		})
	}()
}

func (b *browser) fetchNativeComments(p *playerWindow, n *nativePlayer) {
	if n.commentsLoading || !b.acceptsNativeMetadata(p, n, false, n.commentGeneration) {
		return
	}
	n.commentGeneration++
	generation := n.commentGeneration
	n.commentsLoading = true
	n.commentNotice = "コメントを読み込んでいます…"
	go func() {
		ctx, cancel := context.WithTimeout(n.metadataCtx, 25*time.Second)
		defer cancel()
		rows, err := nativeComments(ctx, n.metadataWatch)
		details := &nativeCommentData{rows: rows, cues: makeDanmakuCues(rows)}
		if err != nil {
			details.notice = "コメントを取得できませんでした。"
		} else if len(rows) == 0 {
			details.notice = "表示できる日本語コメントはありません。"
		}
		mygo.RunOnMain(func() {
			if !b.acceptsNativeMetadata(p, n, false, generation) {
				return
			}
			n.commentsLoading, n.commentsFailed = false, err != nil
			n.stagedComments = details
			p.win.Invalidate()
		})
	}()
}
