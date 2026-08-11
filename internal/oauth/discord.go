package oauth

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

// Discord implements Provider using OAuth2 with client_secret (no PKCE).
type Discord struct {
	config  *oauth2.Config
	userURL string
}

// NewDiscord builds a Discord provider pointing at the live Discord endpoints.
func NewDiscord(clientID, secret, redirectURL string) *Discord {
	return NewDiscordWithEndpoints(clientID, secret, redirectURL,
		"https://discord.com/api/oauth2/token", "https://discord.com/api/users/@me")
}

// NewXWithEndpoints is the testable constructor that lets callers point the
// token/user endpoints at httptest mock servers.
func NewDiscordWithEndpoints(clientID, secret, redirectURL, tokenURL, userURL string) *Discord {
	return &Discord{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: secret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://discord.com/api/oauth2/authorize",
				TokenURL: tokenURL,
			},
			Scopes: []string{"identify"},
		},
		userURL: userURL,
	}
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) AuthURL(state string) string {
	return d.config.AuthCodeURL(state)
}

func (d *Discord) Exchange(ctx context.Context, code, state string) (*Token, error) {
	// Discord uses client_secret to exchange for a token; PKCE is not needed.
	// state is validated by the handler (cookie check), ignored here.
	tok, err := d.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	return &Token{AccessToken: tok.AccessToken, TokenType: tok.TokenType}, nil
}

func (d *Discord) FetchUserID(ctx context.Context, token *Token) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", d.userURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var u struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	return u.ID, nil
}
