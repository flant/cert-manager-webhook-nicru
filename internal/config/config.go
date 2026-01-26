// Package config provides configuration constants and variables for the webhook.
// This includes API URLs, HTTP client settings, and environment variables.
package config

import (
	"net/http"
	"os"
	"time"
)

// Constants
const (
	NameSecret  = "nicru-tokens"
	APIUrl      = "https://api.nic.ru/"
	HTTPTimeout = 30 * time.Second
	MaxRetries  = 3
	RetryDelay  = 2 * time.Second
)

// API URLs - variables for testing purposes
var (
	OAuthURL        = APIUrl + "oauth/token"
	URLCommit       = APIUrl + "dns-master/services/%s/zones/%s/commit"
	URLCreateRecord = APIUrl + "dns-master/services/%s/zones/%s/records"
	URLDeleteRecord = APIUrl + "dns-master/services/%s/zones/%s/records/%s"
	URLGetRecord    = APIUrl + "dns-master/services/%s/zones/%s/records"
	URLGetZoneInfo  = APIUrl + "dns-master/zones/?token=%s"
)

// Environment variables
var (
	GroupName = os.Getenv("GROUP_NAME")
	Namespace = os.Getenv("NAMESPACE")
)

// HTTPClient is the shared HTTP client with timeout
var HTTPClient = &http.Client{
	Timeout: HTTPTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}
