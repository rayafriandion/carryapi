package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

// X implements Provider for Twitter/X OAuth2 with PKCE (S256).
// Because oauth2.Config.Exchange does not support code_verifier, Exchange
// manually POSTs to the token endpoint with the verifier in the form body.
type X struct {
	config    *oauth2.Config
	userURL   string
	mu        sync.Mutex
	verifiers map[string]string // state -> code_verifier
}

// NewX builds an X provider pointing at the live Twitter endpoints.
func NewX(clientID, secret, redirectURL string) *X {
	return NewXWithEndpoints(clientID, secret, redirectURL,
		"https://api.twitter.com/2/oauth2/token", "https://api.twitter.com/2/users/me")
}

// NewXWithEndpoints is the testable constructor for httptest mock servers.
func NewXWithEndpoints(clientID, secret, redirectURL, tokenURL, userURL string) *X {
	return &X{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: secret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://twitter.com/i/oauth2/authorize",
				TokenURL: tokenURL,
			},
			Scopes: []string{"users.read", "tweet.read"},
		},
		userURL:   userURL,
		verifiers: make(map[string]string),
	}
}

func (x *X) Name() string { return "x" }

func (x *X) AuthURL(state string) string {
	// PKCE: generate a random code_verifier, cache it by state, and send the
	// S256 challenge as an AuthCodeURL parameter.
	verifier := randomHex(32)
	challenge := pkceS256(verifier)
	x.mu.Lock()
	x.verifiers[state] = verifier
	x.mu.Unlock()
	return x.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))
}

func (x *X) Exchange(ctx context.Context, code, state string) (*Token, error) {
	verifier, _ := x.ConsumeVerifier(state)
	// Manually POST to the token endpoint with code_verifier (PKCE), since
	// oauth2.Config.Exchange does not support the code_verifier parameter.
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {x.config.RedirectURL},
		"client_id":     {x.config.ClientID},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", x.config.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s: %s", resp.Status, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	return &Token{AccessToken: tok.AccessToken, TokenType: tok.TokenType}, nil
}

// ConsumeVerifier fetches and deletes the code_verifier cached for state
// (called during the OAuth callback). Returns false if no verifier was cached.
func (x *X) ConsumeVerifier(state string) (string, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	v, ok := x.verifiers[state]
	if ok {
		delete(x.verifiers, state)
	}
	return v, ok
}

func pkceS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (x *X) FetchUserID(ctx context.Context, token *Token) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", x.userURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// 非 2xx:读 body 并返回明确错误(否则会静默解码空 body 得到空 id)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("user endpoint returned %s: %s", resp.Status, body)
	}
	var u struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	return u.Data.ID, nil
}
