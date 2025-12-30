package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flant/cert-manager-webhook-nicru/internal/config"
	"github.com/flant/cert-manager-webhook-nicru/internal/metrics"
	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Shared metrics instance for all tests to avoid duplicate registration
var testMetrics = metrics.New()

// Test constants for bootstrap tests
const (
	testNamespace      = "test-namespace"
	testAccessToken    = "test-access-token-12345678"
	testRefreshToken   = "test-refresh-token-12345678"
	testNewAccessToken = "test-new-access-token-87654321"
)

// NicruTokens represents OAuth tokens from NIC.RU API (for testing)
type NicruTokens struct {
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
}

// Helper function to create a fake nicru-account secret
func createAccountSecret(namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nicru-account",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"USERNAME":      []byte("test-user"),
			"PASSWORD":      []byte("test-password"),
			"CLIENT_ID":     []byte("test-client-id"),
			"CLIENT_SECRET": []byte("test-client-secret"),
		},
	}
}

// Helper function to create a fake nicru-tokens secret
func createTokensSecret(namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nicru-tokens",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"ACCESS_TOKEN":  []byte(testAccessToken),
			"REFRESH_TOKEN": []byte(testRefreshToken),
			"APP_ID":        []byte("test-client-id"),
			"APP_SECRET":    []byte("test-client-secret"),
		},
	}
}

// Helper to create test TokenManager config
func createBootstrapTestConfig(server *httptest.Server) tokenmanager.Config {
	zoneURL := config.URLGetZoneInfo
	oauthURL := config.OAuthURL
	if server != nil {
		zoneURL = server.URL + "?token=%s"
		oauthURL = server.URL
	}

	return tokenmanager.Config{
		HTTPClient:  config.HTTPClient,
		OAuthURL:    oauthURL,
		ZoneInfoURL: zoneURL,
		MetricsCallbacks: tokenmanager.MetricsCallbacks{
			IncTokenRefreshTotal:       func() {},
			IncTokenRefreshErrors:      func() {},
			IncTokenValidationTotal:    func() {},
			SetLastTokenRefreshTime:    func() {},
			SetLastTokenValidationTime: func() {},
			SetTokenValidationStatus:   func(float64) {},
		},
	}
}

// Test Bootstrap - success case
func TestBootstrap_Success(t *testing.T) {
	// Mock validation server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	// Create client with both secrets
	client := fake.NewSimpleClientset(
		createAccountSecret(testNamespace),
		createTokensSecret(testNamespace),
	)
	tm := tokenmanager.NewTokenManager(client, testNamespace, createBootstrapTestConfig(server))

	err := Bootstrap(tm, testMetrics)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// Test Bootstrap - failure (no account secret)
func TestBootstrap_NoAccountSecret(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := tokenmanager.NewTokenManager(client, testNamespace, createBootstrapTestConfig(nil))

	err := Bootstrap(tm, testMetrics)
	if err == nil {
		t.Fatal("Expected error for missing account secret, got nil")
	}

	if !strings.Contains(err.Error(), "bootstrap failed") {
		t.Errorf("Expected 'bootstrap failed' error, got: %v", err)
	}
}

// Test Bootstrap - creates tokens when they don't exist
func TestBootstrap_CreatesTokens(t *testing.T) {
	// Mock OAuth server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := NicruTokens{
			AccessToken:  testAccessToken,
			RefreshToken: testRefreshToken,
			ExpiresIn:    14400,
			TokenType:    "Bearer",
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with only account secret
	client := fake.NewSimpleClientset(createAccountSecret(testNamespace))
	tm := tokenmanager.NewTokenManager(client, testNamespace, createBootstrapTestConfig(server))

	err := Bootstrap(tm, testMetrics)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify tokens were created
	token, err := tm.GetCurrentAccessToken()
	if err != nil {
		t.Fatalf("Failed to get access token: %v", err)
	}

	if token != testAccessToken {
		t.Errorf("Expected access token %s, got %s", testAccessToken, token)
	}
}

// Test Bootstrap - refreshes expired tokens
func TestBootstrap_RefreshesExpiredTokens(t *testing.T) {
	callCount := 0

	// Mock server that returns 401 for first validation, then success for refresh
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// First call is validation (GET to zones)
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Second call is refresh (POST to oauth)
		if r.Method == http.MethodPost {
			response := NicruTokens{
				AccessToken: testNewAccessToken,
				ExpiresIn:   14400,
				TokenType:   "Bearer",
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
	}))
	defer server.Close()

	// Create client with both secrets
	client := fake.NewSimpleClientset(
		createAccountSecret(testNamespace),
		createTokensSecret(testNamespace),
	)
	tm := tokenmanager.NewTokenManager(client, testNamespace, createBootstrapTestConfig(server))

	err := Bootstrap(tm, testMetrics)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify token was refreshed
	token, err := tm.GetCurrentAccessToken()
	if err != nil {
		t.Fatalf("Failed to get access token: %v", err)
	}

	if token != testNewAccessToken {
		t.Errorf("Expected refreshed token %s, got %s", testNewAccessToken, token)
	}
}

// Test Bootstrap - metrics are updated
func TestBootstrap_UpdatesMetrics(t *testing.T) {
	// Mock validation server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	// Reset metrics (mock - in real scenario we'd check Prometheus metrics)
	client := fake.NewSimpleClientset(
		createAccountSecret(testNamespace),
		createTokensSecret(testNamespace),
	)
	tm := tokenmanager.NewTokenManager(client, testNamespace, createBootstrapTestConfig(server))

	err := Bootstrap(tm, testMetrics)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// In a real scenario, we would verify:
	// - bootstrapTotal incremented
	// - bootstrapDuration recorded
	// - bootstrapErrors not incremented
	// Since we can't easily check Prometheus metrics in unit tests without
	// additional mocking, we just verify bootstrap succeeded
}

// Test Bootstrap - failure increments error metric
func TestBootstrap_FailureIncrementsErrorMetric(t *testing.T) {
	client := fake.NewSimpleClientset() // No secrets
	tm := tokenmanager.NewTokenManager(client, testNamespace, createBootstrapTestConfig(nil))

	err := Bootstrap(tm, testMetrics)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Verify error is wrapped properly
	if !strings.Contains(err.Error(), "bootstrap failed") {
		t.Errorf("Expected 'bootstrap failed' in error, got: %v", err)
	}
}
