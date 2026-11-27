package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/egoist/mygo"
)

const accountStatusPath = "/v1/users/me"

type loginState uint8

const (
	loginUnknown loginState = iota
	loginChecking
	loginGuest
	loginAuthenticated
)

func (s loginState) label() string {
	switch s {
	case loginChecking:
		return "ログイン状態を確認中…"
	case loginGuest:
		return "未ログイン"
	case loginAuthenticated:
		return "ログイン済み"
	default:
		return "ログイン状態を確認できません"
	}
}

func (c catalog) checkLogin(ctx context.Context) loginState {
	data, err := c.request(ctx, apiHost, accountStatusPath, nil, true)
	if errors.Is(err, errSignIn) {
		return loginGuest
	}
	if err != nil {
		return loginUnknown
	}
	var result struct {
		Meta struct{ Status int }
		Data map[string]json.RawMessage
	}
	if json.Unmarshal(data, &result) != nil {
		return loginUnknown
	}
	switch result.Meta.Status {
	case 401:
		return loginGuest
	case 200:
		if len(result.Data) > 0 {
			return loginAuthenticated
		}
	}
	return loginUnknown
}

func (b *browser) checkLogin() {
	if b.closed || b.session == nil {
		return
	}
	if b.loginCancel != nil {
		b.loginCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	b.loginCancel = cancel
	b.loginCheck++
	check := b.loginCheck
	b.loginState = loginChecking
	b.win.Invalidate()
	go func() {
		defer cancel()
		state := b.catalog.checkLogin(ctx)
		mygo.RunOnMain(func() {
			if b.closed || b.loginCheck != check {
				return
			}
			b.loginCancel = nil
			b.loginState = state
			b.win.Invalidate()
		})
	}()
}

// The shared profile can change without a full document navigation.
func (b *browser) watchLogin() {
	ctx, cancel := context.WithCancel(context.Background())
	b.stopLoginWatch = cancel
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mygo.RunOnMain(func() {
					if b.loginState != loginChecking {
						b.checkLogin()
					}
				})
			}
		}
	}()
}
