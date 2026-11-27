package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"
)

func addVideoTag(tags []string, name string) []string {
	if len(tags) < 24 && exactSearchURL("tag", name) != "" && !slices.Contains(tags, name) {
		return append(tags, name)
	}
	return tags
}

func searchVideoTags(raw json.RawMessage) []string {
	var tags []string
	var packed string
	if json.Unmarshal(raw, &packed) == nil {
		for _, name := range strings.Split(packed, " ") {
			tags = addVideoTag(tags, name)
		}
		return tags
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		var container struct{ Items []json.RawMessage }
		if json.Unmarshal(raw, &container) != nil {
			return nil
		}
		items = container.Items
	}
	for _, item := range items {
		var name string
		if json.Unmarshal(item, &name) != nil {
			var tag struct{ Name string }
			if json.Unmarshal(item, &tag) == nil {
				name = tag.Name
			}
		}
		tags = addVideoTag(tags, name)
	}
	return tags
}

func playbackDetails(ctx context.Context, c catalog, route screenRoute, watch watchData) ([]string, videoPublisher, error) {
	publisher := watch.Channel.publisher("channel")
	if publisher == (videoPublisher{}) {
		publisher = watch.Owner.publisher("user")
	}
	metadata := watch.Tag
	if metadata == nil || metadata.Items == nil {
		metadata = watch.Tags
	}
	var tags []string
	needsTags := metadata == nil || metadata.Items == nil
	if !needsTags {
		for _, item := range metadata.Items {
			tags = addVideoTag(tags, item.Name)
		}
	}
	if needsTags || publisher.videos == "" || publisher.name == "" {
		p, err := c.video(ctx, route)
		if publisher == (videoPublisher{}) {
			publisher = p.publisher
		} else if publisher.videos == p.publisher.videos && publisher.name == "" {
			publisher.name = p.publisher.name
		}
		if needsTags {
			return p.tags, publisher, err
		}
	}
	return tags, publisher, nil
}

func (b *browser) drawVideoTags(c *ui.Context, tags []string, notice string) {
	if len(tags) == 0 {
		ui.Text(c, notice).TextColor(c.Theme().TextMuted).Wrap()
		return
	}
	ui.Row(c).FillWidth().Wrap().Gap(5).Children(func() {
		for _, tag := range tags {
			if metadataButton(c.Key(tag), tag, "タグ検索 "+tag, 240).Clicked() {
				b.browseInMain(exactSearchURL("tag", tag))
			}
		}
	})
}

func metadataButton(c *ui.Context, text, label string, width float32) ui.Element {
	return ui.Button(c, "").Label(label).Tooltip(text).MaxWidth(width).MinWidth(0).Children(func() {
		ui.Text(c, text).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	})
}
