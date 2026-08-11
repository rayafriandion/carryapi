package oauth

import "context"

// Token holds the OAuth access token returned by a provider's token endpoint.
type Token struct {
	AccessToken string
	TokenType   string
}

// Provider abstracts an OAuth2 identity provider (Discord, X, ...).
// Exchange takes both code and state: the X implementation needs state to
// look up the PKCE code_verifier cached during AuthURL.
type Provider interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code, state string) (*Token, error)
	FetchUserID(ctx context.Context, token *Token) (string, error)
	Name() string
}
