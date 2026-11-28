package main

import (
	"strconv"
	"time"
)

func searchMetrics(posted, commented string, likes, mylists *int) map[string]string {
	out := make(map[string]string)
	for key, value := range map[string]string{"f": posted, "n": commented} {
		if at, err := time.Parse(time.RFC3339, value); err == nil {
			out[key] = at.Format("2006/01/02 15:04 -07:00")
		}
	}
	for key, value := range map[string]*int{"likeCount": likes, "m": mylists} {
		if value != nil && *value >= 0 {
			out[key] = strconv.Itoa(*value)
		}
	}
	return out
}
func (item card) searchMetric(r screenRoute) string {
	sort, _ := r.searchSort()
	if value := item.sortMetrics[sort.urlKey]; value != "" {
		return sort.label + "：" + value
	}
	return ""
}
