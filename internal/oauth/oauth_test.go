package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordExchangeAndUserID(t *testing.T) {
	// mock token 端点 + user 端点
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"fake-token","token_type":"Bearer"}`))
	}))
	defer tokenSrv.Close()
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"discord-uid-123"}`))
	}))
	defer userSrv.Close()

	d := NewDiscordWithEndpoints("cid", "secret", "http://cb", tokenSrv.URL, userSrv.URL)
	url := d.AuthURL("mystate")
	if url == "" {
		t.Fatal("empty auth url")
	}
	tok, err := d.Exchange(context.Background(), "code", "mystate")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "fake-token" {
		t.Errorf("token = %q", tok.AccessToken)
	}
	uid, err := d.FetchUserID(context.Background(), tok)
	if err != nil || uid != "discord-uid-123" {
		t.Errorf("uid = %q err %v", uid, err)
	}
}

func TestXExchangeAndUserID(t *testing.T) {
	// X 返回 {"data":{"id":"x-uid-456"}}
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"x-token","token_type":"bearer"}`))
	}))
	defer tokenSrv.Close()
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer x-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"x-uid-456"}}`))
	}))
	defer userSrv.Close()

	x := NewXWithEndpoints("cid", "secret", "http://cb", tokenSrv.URL, userSrv.URL)
	url := x.AuthURL("mystate")
	if url == "" {
		t.Fatal("empty auth url")
	}
	tok, err := x.Exchange(context.Background(), "code", "mystate")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "x-token" {
		t.Errorf("token = %q", tok.AccessToken)
	}
	uid, err := x.FetchUserID(context.Background(), tok)
	if err != nil || uid != "x-uid-456" {
		t.Errorf("uid = %q err %v", uid, err)
	}
}
