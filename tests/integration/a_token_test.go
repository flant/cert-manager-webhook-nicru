//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	oauthURL    = "https://api.nic.ru/oauth/token"
	zonesURL    = "https://api.nic.ru/dns-master/zones/?token=%s"
	httpTimeout = 30 * time.Second
)

var testHTTPClient = &http.Client{
	Timeout: httpTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	},
}

// TestIntegration_01_RequestTokensSuccess tests obtaining OAuth tokens from NIC.RU API
func TestIntegration_01_RequestTokensSuccess(t *testing.T) {
	logTestStart(t, "Request OAuth tokens from NIC.RU API (grant_type=password)")
	creds := skipIfNoCredentials(t)

	// Prepare request parameters
	params := url.Values{
		"grant_type":    {"password"},
		"username":      {creds.Username},
		"password":      {creds.Password},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
		"scope":         {".*"},
		"offline":       {"999999"},
	}

	t.Logf("Sending POST to %s", oauthURL)
	t.Logf("Credentials: username=%s, client_id=%s", creds.Username, creds.ClientID)

	// Send POST request
	resp, err := testHTTPClient.PostForm(oauthURL, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var tokens NicruTokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Validate response fields
	if tokens.AccessToken == "" {
		t.Error("access_token is empty")
	}
	if tokens.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}
	if tokens.ExpiresIn <= 0 {
		t.Errorf("expires_in is invalid: %d", tokens.ExpiresIn)
	}
	if tokens.TokenType != "Bearer" {
		t.Errorf("Expected token_type=Bearer, got %s", tokens.TokenType)
	}

	// Save tokens for subsequent tests
	sharedTokens = &tokens

	logTestSuccess(t, fmt.Sprintf("Obtained tokens: access=%s, refresh=%s, expires_in=%d, token_type=%s",
		maskToken(tokens.AccessToken), maskToken(tokens.RefreshToken), tokens.ExpiresIn, tokens.TokenType))
}

// TestIntegration_02_RequestTokensInvalidCredentials tests error handling with invalid credentials
func TestIntegration_02_RequestTokensInvalidCredentials(t *testing.T) {
	logTestStart(t, "Request OAuth tokens with invalid credentials (should fail)")
	creds := skipIfNoCredentials(t)

	// Prepare request with WRONG password
	params := url.Values{
		"grant_type":    {"password"},
		"username":      {creds.Username},
		"password":      {creds.Password + "-wrong-invalid"},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
		"scope":         {".*"},
		"offline":       {"999999"},
	}

	t.Logf("Sending POST with intentionally wrong password")

	// Send POST request
	resp, err := testHTTPClient.PostForm(oauthURL, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Check that we got an error status (not 200)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("Expected error status (400 or 401), got HTTP 200 with body: %s", string(body))
	}

	t.Logf("Got expected error status: %d", resp.StatusCode)
	t.Logf("Error response: %s", string(body))

	logTestSuccess(t, fmt.Sprintf("Invalid credentials correctly rejected with HTTP %d", resp.StatusCode))
}

// TestIntegration_03_ValidateAccessTokenSuccess tests that access token works with API
func TestIntegration_03_ValidateAccessTokenSuccess(t *testing.T) {
	logTestStart(t, "Validate access token by requesting zones from NIC.RU API")
	skipIfNoTokens(t)

	// Construct URL with access token
	url := fmt.Sprintf(zonesURL, sharedTokens.AccessToken)
	t.Logf("GET %s (token=%s)", zonesURL, maskToken(sharedTokens.AccessToken))

	// Send GET request
	resp, err := testHTTPClient.Get(url)
	if err != nil {
		t.Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Validate that response contains XML (zones)
	if !strings.Contains(string(body), "<response") {
		t.Errorf("Response doesn't look like valid XML: %s", string(body[:100]))
	}

	t.Logf("Response length: %d bytes", len(body))
	logTestSuccess(t, "Access token is valid and works with API")
}

// TestIntegration_04_ValidateAccessTokenInvalid tests handling of invalid token
func TestIntegration_04_ValidateAccessTokenInvalid(t *testing.T) {
	logTestStart(t, "Validate invalid access token (should fail with 401)")

	// Use intentionally invalid token
	invalidToken := "invalid-test-token-12345-this-should-not-work"
	url := fmt.Sprintf(zonesURL, invalidToken)
	t.Logf("GET with invalid token: %s", maskToken(invalidToken))

	// Send GET request
	resp, err := testHTTPClient.Get(url)
	if err != nil {
		t.Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Should get 401 Unauthorized
	if resp.StatusCode == http.StatusOK {
		t.Errorf("Expected HTTP 401, got 200 with body: %s", string(body))
	}

	t.Logf("Got expected error status: %d", resp.StatusCode)
	t.Logf("Error response: %s", string(body))

	logTestSuccess(t, fmt.Sprintf("Invalid token correctly rejected with HTTP %d", resp.StatusCode))
}

// TestIntegration_05_RefreshAccessTokenSuccess tests refreshing access token
func TestIntegration_05_RefreshAccessTokenSuccess(t *testing.T) {
	logTestStart(t, "Refresh access token using refresh_token")
	skipIfNoTokens(t)
	creds := skipIfNoCredentials(t)

	oldAccessToken := sharedTokens.AccessToken

	// Prepare refresh request
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {sharedTokens.RefreshToken},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
	}

	t.Logf("Refreshing token: refresh_token=%s", maskToken(sharedTokens.RefreshToken))

	// Send POST request
	resp, err := testHTTPClient.PostForm(oauthURL, params)
	if err != nil {
		t.Fatalf("Failed to send refresh request: %v", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var newTokens NicruTokens
	if err := json.NewDecoder(resp.Body).Decode(&newTokens); err != nil {
		t.Fatalf("Failed to decode JSON response: %v", err)
	}

	// Validate response
	if newTokens.AccessToken == "" {
		t.Error("New access_token is empty")
	}
	if newTokens.AccessToken == oldAccessToken {
		t.Error("New access_token is the same as old one (should be different)")
	}

	t.Logf("Old access_token: %s", maskToken(oldAccessToken))
	t.Logf("New access_token: %s", maskToken(newTokens.AccessToken))

	// Verify new token works with API
	url := fmt.Sprintf(zonesURL, newTokens.AccessToken)
	checkResp, err := testHTTPClient.Get(url)
	if err != nil {
		t.Fatalf("Failed to validate new token: %v", err)
	}
	defer checkResp.Body.Close()

	if checkResp.StatusCode != http.StatusOK {
		t.Errorf("New access token doesn't work, got HTTP %d", checkResp.StatusCode)
	}

	// Update shared tokens
	sharedTokens.AccessToken = newTokens.AccessToken

	logTestSuccess(t, fmt.Sprintf("Access token refreshed successfully: %s -> %s",
		maskToken(oldAccessToken), maskToken(newTokens.AccessToken)))
}

// TestIntegration_06_RefreshAccessTokenInvalid tests error handling with invalid refresh token
func TestIntegration_06_RefreshAccessTokenInvalid(t *testing.T) {
	logTestStart(t, "Refresh access token with invalid refresh_token (should fail)")
	creds := skipIfNoCredentials(t)

	// Prepare request with invalid refresh token
	params := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"invalid-refresh-token-that-should-not-work"},
		"client_id":     {creds.ClientID},
		"client_secret": {creds.ClientSecret},
	}

	t.Logf("Sending refresh request with invalid token")

	// Send POST request
	resp, err := testHTTPClient.PostForm(oauthURL, params)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Should get error status
	if resp.StatusCode == http.StatusOK {
		t.Errorf("Expected error status (400 or 401), got HTTP 200 with body: %s", string(body))
	}

	t.Logf("Got expected error status: %d", resp.StatusCode)
	t.Logf("Error response: %s", string(body))

	logTestSuccess(t, fmt.Sprintf("Invalid refresh token correctly rejected with HTTP %d", resp.StatusCode))
}

// TestIntegration_07_TokenManagerFullCycle tests full TokenManager lifecycle with real API
func TestIntegration_07_TokenManagerFullCycle(t *testing.T) {
	logTestStart(t, "Full TokenManager lifecycle test with real NIC.RU API")
	creds := skipIfNoCredentials(t)

	// Create fake Kubernetes client
	client := fake.NewSimpleClientset()

	// Create nicru-account secret with real credentials
	accountSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nicru-account",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"USERNAME":      []byte(creds.Username),
			"PASSWORD":      []byte(creds.Password),
			"CLIENT_ID":     []byte(creds.ClientID),
			"CLIENT_SECRET": []byte(creds.ClientSecret),
		},
	}

	_, err := client.CoreV1().Secrets("test-namespace").Create(
		context.Background(), accountSecret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create account secret: %v", err)
	}

	t.Logf("✅ Created nicru-account secret with real credentials")

	// Create TokenManager configuration
	tmConfig := tokenmanager.Config{
		HTTPClient:  testHTTPClient,
		OAuthURL:    oauthURL,
		ZoneInfoURL: zonesURL,
		MetricsCallbacks: tokenmanager.MetricsCallbacks{
			IncTokenRefreshTotal:       func() {},
			IncTokenRefreshErrors:      func() {},
			IncTokenValidationTotal:    func() {},
			SetLastTokenRefreshTime:    func() {},
			SetLastTokenValidationTime: func() {},
			SetTokenValidationStatus:   func(float64) {},
		},
	}

	// Step 1: Create TokenManager
	t.Log("Step 1: Create TokenManager")
	tm := tokenmanager.NewTokenManager(client, "test-namespace", tmConfig)
	if tm == nil {
		t.Fatal("TokenManager creation failed")
	}
	t.Log("✅ TokenManager created successfully")

	// Step 2: EnsureValidTokens (should create nicru-tokens secret)
	t.Log("Step 2: Call EnsureValidTokens() - initial token request from account")
	if err := tm.EnsureValidTokens(); err != nil {
		t.Fatalf("EnsureValidTokens failed: %v", err)
	}
	t.Log("✅ EnsureValidTokens completed")

	// Step 3: Verify nicru-tokens secret was created
	t.Log("Step 3: Verify nicru-tokens secret was created")
	tokensSecret, err := client.CoreV1().Secrets("test-namespace").Get(
		context.Background(), "nicru-tokens", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("nicru-tokens secret not found: %v", err)
	}

	accessToken := string(tokensSecret.Data["ACCESS_TOKEN"])
	refreshToken := string(tokensSecret.Data["REFRESH_TOKEN"])

	if accessToken == "" {
		t.Error("ACCESS_TOKEN is empty in nicru-tokens")
	}
	if refreshToken == "" {
		t.Error("REFRESH_TOKEN is empty in nicru-tokens")
	}

	t.Logf("✅ Tokens created: access=%s, refresh=%s",
		maskToken(accessToken), maskToken(refreshToken))

	// Step 4: ValidateAccessToken with real API
	t.Log("Step 4: Validate access token with real NIC.RU API")
	valid, err := tm.ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if !valid {
		t.Error("Access token should be valid but validation failed")
	}
	t.Log("✅ Access token is valid")

	// Step 5: RefreshAccessToken
	t.Log("Step 5: Refresh access token using refresh_token")
	newAccessToken, err := tm.RefreshAccessToken(refreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken failed: %v", err)
	}
	if newAccessToken == "" {
		t.Error("New access token is empty")
	}
	if newAccessToken == accessToken {
		t.Error("New access token should be different from old one")
	}
	t.Logf("✅ Token refreshed: old=%s, new=%s",
		maskToken(accessToken), maskToken(newAccessToken))

	// Step 6: Verify new token works with real API
	t.Log("Step 6: Validate new access token with real API")
	valid, err = tm.ValidateAccessToken(newAccessToken)
	if err != nil {
		t.Fatalf("New token validation failed: %v", err)
	}
	if !valid {
		t.Error("New access token should be valid")
	}
	t.Log("✅ New access token is valid")

	// Step 7: UpdateAccessToken
	t.Log("Step 7: Update access token in secret")
	if err := tm.UpdateAccessToken(newAccessToken); err != nil {
		t.Fatalf("UpdateAccessToken failed: %v", err)
	}

	// Verify it was updated
	updatedSecret, err := client.CoreV1().Secrets("test-namespace").Get(
		context.Background(), "nicru-tokens", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}

	updatedToken := string(updatedSecret.Data["ACCESS_TOKEN"])
	if updatedToken != newAccessToken {
		t.Errorf("Token not updated: expected %s, got %s",
			maskToken(newAccessToken), maskToken(updatedToken))
	}
	t.Log("✅ Access token updated in secret")

	logTestSuccess(t, "Full TokenManager lifecycle completed successfully - all 7 steps passed!")
}
