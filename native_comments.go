package main

import (
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
)

type commentNGEdit struct {
	author commentAuthor
	block  bool
}

type commentFollow struct {
	manual, positioned bool
	lastRow            int
}

func (f *commentFollow) row(at time.Duration, comments []timedComment, rows []int) (int, bool) {
	if f.manual || len(rows) == 0 {
		return 0, false
	}
	ms := at.Milliseconds()
	end := sort.Search(len(rows), func(i int) bool { return comments[rows[i]].VposMs > ms })
	row := 0
	if end > 0 {
		group := comments[rows[end-1]].VposMs
		row = sort.Search(end, func(i int) bool { return comments[rows[i]].VposMs >= group })
	}
	if f.positioned && f.lastRow == row {
		return 0, false
	}
	f.positioned, f.lastRow = true, row
	return row, true
}

// Rebuild both views only when the comment response or the local NG list changes.
func (n *nativePlayer) applyCommentNG(blocked map[commentAuthor]bool) {
	n.commentRows = n.commentRows[:0]
	for i, row := range n.allComments {
		if !blocked[row.author] {
			n.commentRows = append(n.commentRows, i)
		}
	}
	n.comments = n.comments[:0]
	for _, cue := range n.allCues {
		if !blocked[cue.author] {
			n.comments = append(n.comments, cue)
		}
	}
	n.danmaku.cuesChanged = true
	n.follow.positioned = false
}

func (b *browser) applyPendingCommentNG(n *nativePlayer) {
	edit := n.pendingNG
	if edit.author.ID == "" {
		return
	}
	n.pendingNG = commentNGEdit{}
	if err := b.commentNG.change(edit.author, edit.block); err != nil {
		n.ngNotice = "NG設定を保存できませんでした。"
		return
	}
	n.applyCommentNG(b.commentNG.blocked)
	if edit.block {
		n.ngNotice = "NGに登録しました。"
	} else {
		n.ngNotice = "NGを解除しました。"
	}
}

func (b *browser) drawNativeComments(c *ui.Context, n *nativePlayer, at time.Duration) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
		label := "コメント一覧"
		switchLabel := "NG管理"
		if n.manageNG {
			label, switchLabel = "NGユーザー", "一覧に戻る"
		}
		ui.Text(c, label).Bold().Grow(1).MinWidth(0)
		if !n.manageNG {
			action := "追従を停止"
			if n.follow.manual {
				action = "再生位置に追従"
			}
			if ui.Button(c, action).Clicked() {
				n.follow.manual = !n.follow.manual
				n.follow.positioned = false
			}
		}
		if ui.Button(c, switchLabel).Clicked() {
			n.manageNG = !n.manageNG
			n.follow.positioned = false
		}
	})
	if n.ngNotice != "" {
		ui.Text(c, n.ngNotice).TextColor(t.TextMuted).Role(ui.RoleStatus).Wrap()
	}
	if n.manageNG {
		b.drawCommentNG(c, n)
		return
	}
	if len(n.commentRows) == 0 {
		msg := n.commentNotice
		if msg == "" {
			msg = "表示できるコメントはありません。"
		}
		ui.Text(c, msg).TextColor(t.TextMuted).Wrap()
		return
	}
	n.commentList.Key = func(i int) any { return n.commentRows[i] }
	if row, move := n.follow.row(at, n.allComments, n.commentRows); move {
		n.commentList.ScrollTo(row, ui.Start)
	}
	ui.List(c, &n.commentList, len(n.commentRows), func(i int) {
		row := n.allComments[n.commentRows[i]]
		line := ui.Row(c).FillWidth().AlignItems(ui.Start).Padding(3, 4).Gap(6)
		if i%2 == 0 {
			line.Background(t.Surface)
		}
		line.ContextMenu(func(m *ui.Menu) {
			n.follow.manual = true
			if m.Item("このユーザーを NG").Disabled(row.author.ID == "" || b.commentNG.path == "").Chosen() {
				n.pendingNG = commentNGEdit{row.author, true}
				c.Invalidate()
			}
		}).Children(func() {
			ui.Text(c, formatPlaybackTime(time.Duration(row.VposMs)*time.Millisecond)).Width(46).TextColor(t.TextMuted)
			ui.Text(c, row.Body).Grow(1).MinWidth(0).Wrap()
		})
	}).Grow(1).MinHeight(0).FillWidth().Focusable().ObserveInput(func(ev ui.InputEvent) {
		switch ev.Kind {
		case ui.InputPointerDown:
			n.follow.manual = true
		case ui.InputKeyDown:
			switch ev.Key {
			case ui.KeyUp, ui.KeyDown, ui.KeyPageUp, ui.KeyPageDown, ui.KeyHome, ui.KeyEnd, ui.KeyContextMenu:
				n.follow.manual = true
			}
		}
	}).HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputScroll && ev.DY != 0 {
			n.follow.manual = true
		}
		return false
	})
}

func (b *browser) drawCommentNG(c *ui.Context, n *nativePlayer) {
	t := c.Theme()
	if b.commentNG.path == "" {
		ui.Text(c, "NG設定の保存先を利用できません。").TextColor(t.TextMuted).Wrap()
		return
	}
	entries := b.commentNG.entries
	if len(entries) == 0 {
		ui.Text(c, "NGユーザーは登録されていません。").TextColor(t.TextMuted).Wrap()
		return
	}
	ui.List(c, nil, len(entries), func(i int) {
		author := entries[i]
		label := "ユーザーID " + author.ID
		if author.Video != "" {
			label = "匿名 " + author.Video + " / " + author.ID[len(author.ID)-8:]
		}
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, label).Grow(1).MinWidth(0).Wrap()
			if ui.Button(c, "解除").Label("解除 " + label).Clicked() {
				n.pendingNG = commentNGEdit{author, false}
				c.Invalidate()
			}
		})
	}).Grow(1).MinHeight(0).FillWidth()
}
