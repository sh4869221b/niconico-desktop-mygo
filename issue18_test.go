package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestMissingCountsAreUnknown(t *testing.T) {
	for _, snapshot := range []bool{false, true} {
		for _, present := range []bool{false, true} {
			c := newCatalog(nil)
			c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
				fields := ""
				if present {
					fields = `,"duration":0,"count":{"view":0,"comment":0}`
				}
				body := `{"meta":{"status":200},"data":{"items":[{"id":"sm9","title":"x"` + fields + `}]}}`
				if snapshot {
					fields = ""
					if present {
						fields = `,"lengthSeconds":0,"viewCounter":0,"commentCounter":0`
					}
					body = `{"meta":{"status":200},"data":[{"contentId":"sm9","title":"x"` + fields + `}]}`
				}
				return testResponse(200, body), nil
			})
			r := routeFor(home + "search/x?sort=f")
			var p catalogPage
			var err error
			if snapshot {
				p, err = c.publicSearch(t.Context(), r)
			} else {
				p, err = c.accountSearch(t.Context(), r, false)
			}
			if err != nil || len(p.items) != 1 {
				t.Fatalf("%+v %v", p, err)
			}
			s := p.items[0].summary
			if strings.Contains(s, "—") == present || strings.Contains(s, "0:00") != present {
				t.Fatalf("present=%v summary=%s", present, s)
			}
		}
	}
}
