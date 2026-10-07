package provider

// oauthProviders lists the providers whose subscription OAuth passed the
// phase-7 policy gate. openai only, via the documented "Sign in with
// ChatGPT" open-source token sharing flow — every value below is copied from
// docs/specs/oauth-verification.md §2 (verified live 2026-10-06). Anthropic
// is api_key only by policy and is rejected by config validation before
// reaching this table.
var oauthProviders = map[string]OAuthConfig{
	"openai": {
		Provider:     "openai",
		AuthorizeURL: "https://auth.openai.com/api/accounts/authorize",
		TokenURL:     "https://auth.openai.com/api/accounts/oauth/token",
		RevokeURL:    "https://auth.openai.com/api/accounts/oauth/revoke",
		JWKSURL:      "https://auth.openai.com/.well-known/jwks.json",
		Issuer:       "https://auth.openai.com",
		Scopes: []string{
			"openid", "profile", "email", // identity
			"offline_access", "resource.invoke", "chatgpt.tokens.use.direct", // plan usage
		},
		RequiredScopes:  []string{"chatgpt.tokens.use.direct"},
		Resource:        "https://api.openai.com/v1",
		RedirectHost:    "127.0.0.1", // the flow rejects localhost
		RedirectPath:    "/auth/callback",
		DynamicClientID: "dynamic_agent_client",
		AgentName:       "moca",
	},
}

// OAuthProvider returns the OAuth parameters for p; ok is false when the
// provider ships api_key only (or is unknown to the OAuth layer).
func OAuthProvider(p string) (OAuthConfig, bool) {
	c, ok := oauthProviders[p]
	return c, ok
}
