// Package dns provides a client for interacting with the NIC.RU DNS API.
// It handles DNS zone operations, record management (create, read, delete),
// and zone commits for ACME DNS-01 challenge validation.
package dns

import (
	"net/http"

	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
)

// Client handles DNS operations with NIC.RU API
type Client struct {
	httpClient   *http.Client
	tokenManager *tokenmanager.TokenManager
}

// NewClient creates a new DNS client
func NewClient(httpClient *http.Client, tm *tokenmanager.TokenManager) *Client {
	return &Client{
		httpClient:   httpClient,
		tokenManager: tm,
	}
}
