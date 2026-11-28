package main

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
	"golang.org/x/net/html"
)

var descriptionReferences = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://|//|(?:www\.|sp\.)?nicovideo\.jp/|nico\.ms/)[^\s\p{Z}<>“”"「」『』、。！？，；]+|(?:mylist/|(?:watch/)?(?:sm|nm|so))[0-9]+`)

type descriptionPart struct {
	text    string
	link    card
	omitted bool
}

func descriptionLink(raw string) (card, string) {
	raw = strings.TrimRight(raw, "。．、！？：；,.)]}!?;）】」")
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "http://"):
		raw = "https://" + raw[len("http://"):]
	case strings.HasPrefix(raw, "//"):
		raw = "https:" + raw
	case strings.HasPrefix(raw, "/"):
		raw = strings.TrimSuffix(home, "/") + raw
	case videoID.MatchString(raw):
		raw = home + "watch/" + lower
	case shortPath.MatchString(raw):
		raw = home + lower
	case allowedURL(raw) == nil && !strings.Contains(raw, ":"):
		raw = "https://" + raw
	}
	link, kind := canonical(raw)
	r := routeFor(raw)
	switch {
	case r.kind == screenVideo:
		u, _ := url.Parse(r.target)
		if from := u.Query().Get("from"); from != "" {
			link += "?" + url.Values{"from": {from}}.Encode()
		}
	case kind == "マイリスト" && r.browsesInMain():
	case kind == "ユーザー" && r.browsesInMain():
	default:
		return card{}, ""
	}
	label := strings.TrimPrefix(link, home)
	label = strings.TrimPrefix(label, "watch/")
	return card{link: r.target, title: label}, link
}

func referenceNeighbor(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' ||
		strings.ContainsRune("_@%:/+-=?&#", rune(b))
}

func plainDescription(value string) (string, []card, []descriptionPart) {
	var text, plain strings.Builder
	var links []card
	var parts []descriptionPart
	seen := make(map[string]bool)
	omitted := false
	add := func(raw string) card {
		link, key := descriptionLink(raw)
		if key != "" && !seen[key] {
			if len(links) >= 24 {
				omitted = true
				return card{}
			}
			links = append(links, link)
			seen[key] = true
		}
		return link
	}
	interactive := 0
	write := func(s string, link card) {
		text.WriteString(s)
		if link.link != "" && interactive >= 64 {
			omitted = true
		}
		if link.link == "" || interactive >= 64 {
			plain.WriteString(s)
			return
		}
		if plain.Len() != 0 {
			parts = append(parts, descriptionPart{text: plain.String()})
			plain.Reset()
		}
		parts = append(parts, descriptionPart{text: s, link: link})
		interactive++
	}
	writeText := func(s string) {
		start := 0
		for _, match := range descriptionReferences.FindAllStringIndex(s, -1) {
			lo, hi := match[0], match[1]
			if lo > 0 && (referenceNeighbor(s[lo-1]) || s[lo-1] == '.') ||
				hi < len(s) && (referenceNeighbor(s[hi]) || s[hi] == '.' && hi+1 < len(s) && referenceNeighbor(s[hi+1])) {
				continue
			}
			raw := strings.TrimRight(s[lo:hi], "。．、！？：；,.)]}!?;）】」")
			if link := add(raw); link.link != "" {
				write(s[start:lo], card{})
				write(raw, link)
				start = lo + len(raw)
			}
		}
		write(s[start:], card{})
	}
	z := html.NewTokenizer(strings.NewReader(value))
	var ignored string
	var anchor card
	var inAnchor bool
	for {
		switch z.Next() {
		case html.ErrorToken:
			if plain.Len() != 0 {
				parts = append(parts, descriptionPart{text: plain.String()})
			}
			if len(parts) > 0 {
				parts[0].text = strings.TrimLeftFunc(parts[0].text, unicode.IsSpace)
				parts[len(parts)-1].text = strings.TrimRightFunc(parts[len(parts)-1].text, unicode.IsSpace)
			}
			if omitted {
				parts = append(parts, descriptionPart{omitted: true})
			}
			return strings.TrimSpace(text.String()), links, parts
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if ignored != "" {
				continue
			}
			switch t.Data {
			case "script", "style":
				ignored = t.Data
			case "br", "p", "div":
				write("\n", card{})
			case "a":
				inAnchor, anchor = true, card{}
				for _, attr := range t.Attr {
					if attr.Key == "href" {
						anchor = add(attr.Val)
					}
				}
			}
		case html.EndTagToken:
			switch tag := z.Token().Data; tag {
			case ignored:
				ignored = ""
			case "a":
				inAnchor = false
			}
		case html.TextToken:
			if ignored != "" {
				continue
			}
			if inAnchor {
				write(string(z.Text()), anchor)
			} else {
				writeText(string(z.Text()))
			}
		}
	}
}

func (b *browser) followDescription(target string) {
	if routeFor(target).kind == screenVideo {
		b.openPlayer(target)
		return
	}
	b.browseInMain(target)
}

func (b *browser) browseInMain(target string) {
	b.navigate(target)
	if b.win != nil {
		b.win.Show()
		b.win.Focus()
	}
}

func (b *browser) drawDescription(c *ui.Context, parts []descriptionPart, sources ...string) {
	ui.RichText(c).Wrap().Children(func() {
		for i, part := range parts {
			if part.text == "" {
				continue
			}
			if part.link.link == "" {
				ui.Text(c, part.text)
			} else if ui.Link(c.Key(i), part.text, "").Label(strings.TrimSpace(part.text)).Description(part.link.title + " · " + descriptionAction(part.link.link)).Clicked() {
				b.followDescription(part.link.link)
			}
		}
	})
	for _, part := range parts {
		if !part.omitted {
			continue
		}
		ui.Text(c, "一部のリンクを省略しています。公式プレーヤーの概要から残りを確認できます。").Wrap().Role(ui.RoleStatus)
		source := b.current()
		if len(sources) > 0 {
			source = sources[0]
		}
		if routeFor(source).kind == screenVideo && ui.Button(c, "公式プレーヤーで概要を確認").Clicked() {
			b.openPlayerMode(source, webPlayback)
		}
		break
	}
}

func descriptionAction(target string) string {
	if routeFor(target).kind == screenVideo {
		return "動画を再生"
	}
	return "一覧を開く"
}
