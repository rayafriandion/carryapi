package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testProvider(baseURL, protocol string) Provider {
	return Provider{BaseURL: baseURL, APIKey: "sk-test", Protocol: protocol}
}

func TestFetchModelsOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	models, err := p.FetchModels(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4o-mini" {
		t.Errorf("models = %v", models)
	}
}

func TestFetchModelsAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet"}]}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	models, err := p.FetchModels(context.Background(), testProvider(srv.URL, "anthropic"))
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 1 || models[0] != "claude-3-5-sonnet" {
		t.Errorf("models = %v", models)
	}
}

func TestFetchModelsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	_, err := p.FetchModels(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("err = %q, want status 401", err)
	}
	if !strings.Contains(err.Error(), "bad key") {
		t.Errorf("err = %q, want body message", err)
	}
}

func TestPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	d, err := p.Ping(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if d < 0 {
		t.Errorf("latency negative: %v", d)
	}
}

func TestPingNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	_, err := p.Ping(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %q, want status 500", err)
	}
}
