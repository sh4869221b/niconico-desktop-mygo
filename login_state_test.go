package main

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestSharedLoginState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		body   string
		want   loginState
	}{
		{name: "without cookie", err: errSignIn, want: loginGuest},
		{name: "store unavailable", err: errors.New("unavailable"), want: loginUnknown},
		{name: "server confirms", status: 200, body: `{"meta":{"status":200},"data":{"id":1}}`, want: loginAuthenticated},
		{name: "expired", status: 401, want: loginGuest},
		{name: "forbidden", status: 403, want: loginUnknown},
		{name: "offline", status: 503, want: loginUnknown},
		{name: "invalid response", status: 200, body: `{"meta":{"status":200}}`, want: loginUnknown},
		{name: "sign-out", status: 200, body: `{"meta":{"status":401}}`, want: loginGuest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCatalog(func(raw string) (*http.Cookie, error) {
				if raw != "https://"+apiHost+accountStatusPath {
					t.Fatalf("incorrect account query %q", raw)
				}
				if tc.err != nil {
					return nil, tc.err
				}
				return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, nil
			})
			c.http.Transport = requestFunc(func(r *http.Request) (*http.Response, error) {
				if tc.err != nil || r.Method != http.MethodGet || r.URL.Host != apiHost ||
					r.URL.Path != accountStatusPath || r.Header.Get("Cookie") != "user_session=fixture" {
					t.Fatal("incorrect status request")
				}
				return testResponse(tc.status, tc.body), nil
			})
			if got := c.checkLogin(t.Context()); got != tc.want {
				t.Fatalf("login check got %q, want %q", got.label(), tc.want.label())
			}
		})
	}
	const target = "https://" + apiHost + accountStatusPath
	_, err := readSessionCookie(func(mygo.CookieFilter) ([]mygo.Cookie, error) {
		return nil, errors.New("store unavailable")
	}, target, time.Now())
	if err == nil || errors.Is(err, errSignIn) {
		t.Fatalf("failed to distinguish the cookie store error: %v", err)
	}
}
