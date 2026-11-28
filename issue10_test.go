package main

import (
	"errors"
	"net/http"
	"testing"
)

func TestPrivateListAuthFailuresAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code int
		want error
	}{
		{errors.New("profile locked"), 0, errSessionUnavailable},
		{errSignIn, 0, errSignIn}, {nil, 401, errSignIn}, {nil, 403, errAccessDenied},
	} {
		c := newCatalog(func(string) (*http.Cookie, error) {
			return &http.Cookie{Name: "user_session", Value: "fixture", Secure: true}, tc.err
		})
		c.http.Transport = requestFunc(func(*http.Request) (*http.Response, error) {
			if tc.err != nil {
				t.Fatal("request after cookie read failure")
			}
			return testResponse(tc.code, ""), nil
		})
		_, err := c.load(t.Context(), routeFor(mylists))
		if !errors.Is(err, tc.want) {
			t.Fatalf("got %v want %v", err, tc.want)
		}
	}
}
