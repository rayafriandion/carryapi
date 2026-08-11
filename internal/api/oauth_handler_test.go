package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/auth"
	"carryapi/internal/oauth"
)

// mockProvider is a test double for oauth.Provider. It returns deterministic
// auth URL, token, and user-id values so the OAuth callback can be exercised
// end-to-end without a real provider round-trip.
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) AuthURL(state string) string {
	return "http://example.com/auth?state=" + state
}
func (m *mockProvider) Exchange(ctx context.Context, code, state string) (*oauth.Token, error) {
	return &oauth.Token{AccessToken: "mock-token"}, nil
}
func (m *mockProvider) FetchUserID(ctx context.Context, token *oauth.Token) (string, error) {
	return "mock-uid-789", nil
}

// TestOAuthCallbackIdentifiesProvider is a regression test for the bug where the
// callback route was mounted without a {provider} param, causing
// chi.URLParam(r, "provider") to return "" and every callback to fail with
// 400 "unknown provider". It verifies that, with the route including the
// provider param, the callback identifies the provider, finds the existing
// binding, and establishes a session.
func TestOAuthCallbackIdentifiesProvider(t *testing.T) {
	f := setupAPI(t)
	oh := NewOAuthHandler(f.users, f.sessions, f.settings)
	oh.RegisterProvider(&mockProvider{name: "mock"})
	// Seed: create a user + bind the mock provider so callback finds an
	// existing binding (rather than taking the create-new-user path).
	u, _ := f.users.Create("oauth@x.com", "", "user")
	if err := f.users.AddAuthMethod(u.ID, "mock", "mock-uid-789", nil); err != nil {
		t.Fatalf("seed AddAuthMethod: %v", err)
	}

	// Build a chi router with both OAuth routes mounted exactly as the
	// production router mounts them (provider in the path).
	r := chi.NewRouter()
	r.Get("/api/auth/oauth/{provider}", oh.Begin)
	r.Get("/api/auth/oauth/callback/{provider}", oh.Callback)

	// Call Begin to obtain the oauth_state cookie that the callback must echo.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/oauth/mock", nil)
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("begin code=%d, want 307", rec.Code)
	}
	var stateVal string
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" {
			stateVal = c.Value
		}
	}
	if stateVal == "" {
		t.Fatal("no oauth_state cookie set by Begin")
	}

	// Now call callback with the state cookie + code. The provider is
	// identified from the path, the existing binding is found, and a session
	// cookie should be set.
	req = httptest.NewRequest("GET", "/api/auth/oauth/callback/mock?code=abc&state="+stateVal, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: stateVal})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code == 400 {
		t.Errorf("callback returned 400 (provider not identified): %s", rec.Body.String())
	}
	// Successful callback redirects home.
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("callback code=%d, want 307", rec.Code)
	}
	// Should set a session cookie (existing binding -> session created).
	hasSession := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			hasSession = true
		}
	}
	if !hasSession {
		t.Error("expected session cookie after successful callback")
	}
}

// TestOAuthCallbackUnknownProvider ensures that an unregistered provider name
// still returns 400 (guards against accidentally treating "" as valid).
func TestOAuthCallbackUnknownProvider(t *testing.T) {
	f := setupAPI(t)
	oh := NewOAuthHandler(f.users, f.sessions, f.settings)

	r := chi.NewRouter()
	r.Get("/api/auth/oauth/callback/{provider}", oh.Callback)

	req := httptest.NewRequest("GET", "/api/auth/oauth/callback/nope?code=abc&state=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "xyz"})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("unknown provider: code=%d, want 400", rec.Code)
	}
}
