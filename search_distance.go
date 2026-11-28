package main

import (
	"fmt"
	"strconv"
)

func (p catalogPage) lastSearchPage() int {
	limit := (maxCatalogPage + 1) * searchPageSize
	if p.searchMode == "スナップショット検索" {
		limit = min(limit, snapshotResultLimit)
	}
	if p.total != nil {
		limit = min(limit, max(0, *p.total))
	}
	return max(0, (limit+searchPageSize-1)/searchPageSize-1)
}
func (p catalogPage) searchRange(r screenRoute) string {
	start, end := r.page*searchPageSize+1, r.page*searchPageSize+p.rawCount
	span := "0件"
	if p.rawCount > 0 {
		span = fmt.Sprintf("%d–%d件", start, end)
	}
	if p.total == nil {
		return span + "（総件数不明）"
	}
	text := fmt.Sprintf("%s / 全%d件", span, max(0, *p.total))
	if p.searchMode == "スナップショット検索" && *p.total > snapshotResultLimit {
		text += fmt.Sprintf("（取得上限%d件）", snapshotResultLimit)
	}
	return text
}
func (p catalogPage) searchPageNumber(text string) (int, bool) {
	n, err := strconv.Atoi(text)
	return n - 1, err == nil && numeric(text, len(text)) && n >= 1 && n <= p.lastSearchPage()+1
}
