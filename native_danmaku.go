package main

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/egoist/mygo/ui"
)

type commentPosition uint8

const (
	scrollComment commentPosition = iota
	topComment
	bottomComment
	scrollLifetime int64 = 4000
	fixedLifetime  int64 = 3000
)

type commentStyle struct {
	position commentPosition
	size     float32 // relative to a 360-DIP video
	rgb      uint32
}

var commentColors = map[string]uint32{
	"white": 0xffffff, "red": 0xff0000, "pink": 0xff8080, "orange": 0xffcc00,
	"yellow": 0xffff00, "green": 0x00ff00, "cyan": 0x00ffff, "blue": 0x0000ff,
	"purple": 0xc000ff, "black": 0x000000,
	"white2": 0xcccc99, "niconicowhite": 0xcccc99, "red2": 0xcc0033, "truered": 0xcc0033,
	"pink2": 0xff33cc, "orange2": 0xff6600, "passionorange": 0xff6600,
	"yellow2": 0x999900, "madyellow": 0x999900, "green2": 0x00cc66, "elementalgreen": 0x00cc66,
	"cyan2": 0x00cccc, "blue2": 0x3399ff, "marineblue": 0x3399ff,
	"purple2": 0x6633cc, "nobleviolet": 0x6633cc, "black2": 0x666666,
}

func parseCommentStyle(commands []string) commentStyle {
	s := commentStyle{size: 26, rgb: 0xffffff}
	var position, size, color bool
	for _, raw := range commands {
		cmd := strings.ToLower(strings.TrimSpace(raw))
		if !position {
			switch cmd {
			case "ue":
				s.position, position = topComment, true
			case "shita":
				s.position, position = bottomComment, true
			case "naka":
				position = true
			}
		}
		if !size {
			switch cmd {
			case "big":
				s.size, size = 38, true
			case "small":
				s.size, size = 15, true
			case "medium":
				size = true
			}
		}
		if !color {
			if c, ok := commentColors[cmd]; ok {
				s.rgb, color = c, true
			} else if len(cmd) == 7 && cmd[0] == '#' {
				if c, err := strconv.ParseUint(cmd[1:], 16, 24); err == nil {
					s.rgb, color = uint32(c), true
				}
			}
		}
	}
	return s
}

// Keep intentional line breaks, indentation and spacing, but bound the work per comment.
func cleanDanmakuText(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	var b strings.Builder
	lines, runes := 1, 0
	for _, r := range body {
		if runes >= 240 {
			break
		}
		if r == '\n' {
			if lines >= 12 {
				break
			}
			lines++
		} else if r == '\t' {
			r = '　'
		} else if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		runes++
	}
	if strings.TrimSpace(b.String()) == "" {
		return ""
	}
	return b.String()
}

type danmakuCue struct {
	body       string
	start, end int64
	style      commentStyle
}

func makeDanmakuCues(comments []timedComment) []danmakuCue {
	cues := make([]danmakuCue, 0, len(comments))
	for _, c := range comments {
		style := parseCommentStyle(c.Commands)
		start, lifetime := c.VposMs, fixedLifetime
		if style.position == scrollComment {
			// The official overlay starts the entrance before the marked comment time.
			start -= 1000
			lifetime = scrollLifetime
		}
		cues = append(cues, danmakuCue{c.Body, start, start + lifetime, style})
	}
	slices.SortStableFunc(cues, func(a, b danmakuCue) int {
		if a.start < b.start {
			return -1
		}
		if a.start > b.start {
			return 1
		}
		return 0
	})
	return cues
}

type commentBox struct {
	start, end    int64
	width, height float32
	y             float32
}

func (c commentBox) x(at int64, areaWidth float32) float32 {
	return areaWidth - (areaWidth+c.width)*float32(at-c.start)/float32(c.end-c.start)
}

type commentLanes struct {
	rowHeight float32
	rows      [3][]commentBox
	overflow  [3]int
}

func (l *commentLanes) reset(height float32) {
	l.rowHeight = max(2, height/120)
	n := max(1, int(height/l.rowHeight))
	for i := range l.rows {
		l.rows[i] = slices.Grow(l.rows[i][:0], n)[:n]
		clear(l.rows[i])
	}
	l.overflow = [3]int{}
}

func (l *commentLanes) place(c commentBox, pos commentPosition, width, height float32) float32 {
	rows := l.rows[pos]
	slots := min(len(rows), max(1, int((c.height+2+l.rowHeight-0.001)/l.rowHeight)))
	conflicts := func(previous commentBox) bool {
		if previous.end <= c.start {
			return false
		}
		if pos != scrollComment {
			return true
		}
		// Relative motion is linear: a gap at both endpoints guarantees no catch-up.
		gap := max(4, height/30)
		return previous.x(c.start, width)+previous.width+gap > width ||
			previous.x(previous.end, width)+previous.width+gap > c.x(previous.end, width)
	}
	// A conflict at row k rules out every candidate starting from first
	// through k. Check the far end first so a full lane takes one probe.
search:
	for first := 0; first+slots <= len(rows); {
		end := first + slots
		for k := end - 1; k >= first; k-- {
			if conflicts(rows[k]) {
				first = k + 1
				continue search
			}
		}
		for i := first; i < end; i++ {
			rows[i] = c
		}
		y := float32(first) * l.rowHeight
		if pos == bottomComment {
			y = height - c.height - y
		}
		return max(0, y)
	}
	// Under saturation, preserve the stream and distribute unavoidable overlaps
	// across the visible area rather than dropping all comments past a fixed count.
	alternatives := max(1, len(rows)/slots)
	offset := float32((l.overflow[pos]%alternatives)*slots) * l.rowHeight
	l.overflow[pos]++
	if pos == bottomComment {
		offset = height - c.height - offset
	}
	return max(0, offset)
}

type drawnComment struct {
	commentBox
	body     string
	fontSize float32
	style    commentStyle
}

type commentMeasureKey struct {
	body  string
	size  float32
	fixed bool
}

type commentMeasure struct {
	width, height, fontSize float32
}

type danmakuOverlay struct {
	width, height float32
	// The last video time and next comment allow incremental playback. On a seek,
	// prime the lanes from earlier comments instead of depending on wall-clock time.
	last     int64
	next     int
	lanes    commentLanes
	active   []drawnComment
	measures map[commentMeasureKey]commentMeasure
}

// The official player lets comments cross the letterbox bars of a narrow
// picture. Expand up to its standard 16:9 canvas without changing video fit.
func danmakuBounds(view, picture ui.Rect) ui.Rect {
	w := min(view.W, max(picture.W, picture.H*16/9))
	picture.X += (picture.W - w) / 2
	picture.W = w
	return picture
}
func (d *danmakuOverlay) draw(ctx *ui.Context, r ui.Rect, comments []danmakuCue, ms int64) {
	if r.W < 10 || r.H < 30 {
		return
	}
	resized := d.width != r.W || d.height != r.H
	if resized {
		clear(d.measures)
	}
	if resized || ms < d.last || ms-d.last >= scrollLifetime {
		d.width, d.height = r.W, r.H
		d.lanes.reset(r.H)
		clear(d.active)
		d.active = d.active[:0]
		d.next, _ = slices.BinarySearchFunc(comments, ms-2*scrollLifetime, func(c danmakuCue, t int64) int {
			if c.start < t {
				return -1
			}
			if c.start > t {
				return 1
			}
			return 0
		})
	}
	d.last = ms
	d.active = slices.DeleteFunc(d.active, func(c drawnComment) bool { return c.end <= ms })
	for d.next < len(comments) && comments[d.next].start <= ms {
		c := comments[d.next]
		d.next++
		style := c.style
		m := d.measure(c)
		width, height := m.width, m.height
		if width <= 0 || height <= 0 {
			continue
		}
		box := commentBox{start: c.start, end: c.end, width: width, height: height}
		box.y = d.lanes.place(box, style.position, r.W, r.H)
		if c.end > ms {
			d.active = append(d.active, drawnComment{box, c.body, m.fontSize, style})
		}
	}
	if len(d.active) == 0 {
		return
	}
	// The toolkit draws a single cached stroke glyph and a fill. Its width
	// includes both sides of the outline, so double the old outward offset.
	stroke := 2 * max(1, r.H/280)
	ui.Box(ctx).Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).Clip().IgnoreInput(true).AccessHidden(true).Children(func() {
		for i := range d.active {
			item := &d.active[i]
			x := (r.W - item.width) / 2
			if item.style.position == scrollComment {
				x = item.x(ms, r.W)
			}
			if x >= r.W || x+item.width <= 0 {
				continue
			}
			rgb := item.style.rgb
			color := ui.RGB(uint8(rgb>>16), uint8(rgb>>8), uint8(rgb))
			border := ui.RGB(0, 0, 0)
			if rgb == 0 {
				border = ui.RGB(255, 255, 255)
			}
			// Reuse the box measured for lane placement so moving the label
			// does not ask layout for its natural height again.
			label := ui.Text(ctx, item.body).Absolute().Left(x).Top(item.y).Size(item.width+0.5, item.height).
				Font("Arial").FontSize(item.fontSize).FontWeight(700).Lang("ja").LineHeight(1.15).
				NoWrap().TextColor(color).TextStroke(stroke, border).IgnoreInput(true)
			if item.style.position != scrollComment {
				label.TextAlign(ui.Center)
			}
		}
	})
}

// The same text, size and alignment share metrics within a video viewport.
// Keep only numeric geometry. Shaped layouts and glyphs belong to MyGo.
func (d *danmakuOverlay) measure(c danmakuCue) commentMeasure {
	key := commentMeasureKey{c.body, c.style.size, c.style.position != scrollComment}
	if m, ok := d.measures[key]; ok {
		return m
	}
	size := max(6, c.style.size*d.height/360)
	font := ui.Font{Family: "Arial", Size: size, Weight: 700, Lang: "ja", LineHeight: 1.15}
	width, height := ui.MeasureText(c.body, font, 0)
	if key.fixed {
		// Fixed comments should remain readable even when a line is wider than the picture.
		if width > d.width-4 || height > d.height-4 {
			font.Size *= max(0.15, min((d.width-4)/width, (d.height-4)/height))
			width, height = ui.MeasureText(c.body, font, 0)
		}
		if strings.Contains(c.body, "\n") {
			layout := ui.ShapeRichTextWith([]ui.Span{{Text: c.body}}, font, ui.TextShape{Width: width + 0.5, Align: ui.Center})
			width, height = commentExtent(layout)
		}
	}

	m := commentMeasure{width, height, font.Size}
	// The player owns one cue list (at most 15,000 comments).
	if d.measures == nil {
		d.measures = make(map[commentMeasureKey]commentMeasure)
	}
	d.measures[key] = m
	return m
}

func commentExtent(layout *ui.TextLayout) (w, h float32) {
	for _, line := range layout.Lines() {
		w = max(w, line.Rect.X+line.Rect.W)
		h = max(h, line.Rect.Y+line.Rect.H)
	}
	return
}
