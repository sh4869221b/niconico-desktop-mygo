package main

import (
	"encoding/json"
	"strings"
)

type videoPublisher struct {
	name, videos string
}

type publisherMetadata struct {
	ID                        json.RawMessage
	Name, Nickname, OwnerType string
}

func (m publisherMetadata) publisher(kind string) videoPublisher {
	var id string
	if json.Unmarshal(m.ID, &id) != nil {
		id = string(m.ID)
	}
	name := strings.TrimSpace(m.Name)
	if name == "" {
		name = strings.TrimSpace(m.Nickname)
	}
	return publisherFor(kind, id, name)
}

func publisherFor(kind, id, name string) videoPublisher {
	p := videoPublisher{name: strings.TrimSpace(name)}
	switch kind {
	case "user":
		if numeric(id, 9) {
			p.videos = home + "user/" + id + "/video"
		}
	case "channel":
		if id = strings.TrimPrefix(id, "ch"); numeric(id, 12) {
			p.videos = "https://ch.nicovideo.jp/ch" + id + "/video"
		}
	}
	return p
}

func (p videoPublisher) linkLabel() string {
	if p.name != "" {
		return p.name + "の投稿動画"
	}
	return "投稿動画"
}
