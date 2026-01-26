package tokenmanager

// NicruTokens represents OAuth tokens from NIC.RU API
type NicruTokens struct {
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`

	// AppID and AppSecret are not part of OAuth response
	// They are copied from nicru-account secret for storage in nicru-tokens
	AppID     string `json:"-"`
	AppSecret string `json:"-"`
}

// AccountCredentials contains user credentials from nicru-account secret
type AccountCredentials struct {
	Username     string
	Password     string
	ClientID     string
	ClientSecret string
}
