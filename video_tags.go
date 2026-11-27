package main

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"
)

func addVideoTag(tags []string, name string) []string {
	if len(tags) < 24 && exactSearchURL("tag", name) != "" && !slices.Contains(tags, name) {
		return append(tags, name)
	}
	return tags
}

func playbackTags(ctx context.Context, c catalog, route screenRoute, watch watchData) ([]string, error) {
	if watch.Tags != nil && watch.Tags.Items != nil {
		var tags []string
		for _, item := range watch.Tags.Items {
			tags = addVideoTag(tags, item.Name)
		}
		return tags, nil
	}
	p, err := c.video(ctx, route)
	return p.tags, err
}

func (b *browser) drawVideoTags(c *ui.Context, tags []string, notice string) {
	if len(tags) == 0 {
		ui.Text(c, notice).TextColor(c.Theme().TextMuted).Wrap()
		return
	}
	ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
		for _, tag := range tags {
			if ui.Button(c.Key(tag), tag).Label("タグ検索 " + tag).Tooltip(tag).MaxWidth(260).Clicked() {
				b.navigate(exactSearchURL("tag", tag))
				if b.win != nil {
					b.win.Show()
					b.win.Focus()
				}
			}
		}
	})
}
