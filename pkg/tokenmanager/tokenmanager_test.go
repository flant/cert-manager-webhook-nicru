package tokenmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Test data constants
const (
	testNamespace      = "test-namespace"
	testUsername       = "test-user"
	testPassword       = "test-password"
	testClientID       = "test-client-id"
	testClientSecret   = "test-client-secret"
	testAccessToken    = "test-access-token-12345678"
	testRefreshToken   = "test-refresh-token-12345678"
	testNewAccessToken = "test-new-access-token-87654321"
	testOAuthURL       = "https://api.nic.ru/oauth/token"
	testZoneInfoURL    = "https://api.nic.ru/dns-master/zones/?token=%s"
)

// Helper function to create test config
func createTestConfig(server *httptest.Server) Config {
	oauthURL := testOAuthURL
	zoneInfoURL := testZoneInfoURL
	if server != nil {
		oauthURL = server.URL
		zoneInfoURL = server.URL + "/?token=%s"
	}

	return Config{
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		OAuthURL:    oauthURL,
		ZoneInfoURL: zoneInfoURL,
		MetricsCallbacks: MetricsCallbacks{
			// No-op callbacks for tests
			IncTokenRefreshTotal:       func() {},
			IncTokenRefreshErrors:      func() {},
			IncTokenValidationTotal:    func() {},
			SetLastTokenRefreshTime:    func() {},
			SetLastTokenValidationTime: func() {},
			SetTokenValidationStatus:   func(float64) {},
		},
	}
}

// Helper function to create a fake nicru-account secret
func createAccountSecret(namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameAccountSecret,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"USERNAME":      []byte(testUsername),
			"PASSWORD":      []byte(testPassword),
			"CLIENT_ID":     []byte(testClientID),
			"CLIENT_SECRET": []byte(testClientSecret),
		},
	}
}

// Helper function to create a fake nicru-tokens secret
func createTokensSecret(namespace string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameTokensSecret,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"APP_ID":        []byte(testClientID),
			"APP_SECRET":    []byte(testClientSecret),
			"ACCESS_TOKEN":  []byte(testAccessToken),
			"REFRESH_TOKEN": []byte(testRefreshToken),
		},
	}
}

// Test TokenManager creation
func TestNewTokenManager(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	if tm == nil {
		t.Fatal("NewTokenManager returned nil")
	}

	if tm.client == nil {
		t.Error("TokenManager client is nil")
	}

	if tm.namespace != testNamespace {
		t.Errorf("Expected namespace %s, got %s", testNamespace, tm.namespace)
	}
}

// Test getAccountCredentials - success case
func TestGetAccountCredentials_Success(t *testing.T) {
	client := fake.NewSimpleClientset(createAccountSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	creds, err := tm.getAccountCredentials()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if creds.Username != testUsername {
		t.Errorf("Expected username %s, got %s", testUsername, creds.Username)
	}
	if creds.Password != testPassword {
		t.Errorf("Expected password %s, got %s", testPassword, creds.Password)
	}
	if creds.ClientID != testClientID {
		t.Errorf("Expected client ID %s, got %s", testClientID, creds.ClientID)
	}
	if creds.ClientSecret != testClientSecret {
		t.Errorf("Expected client secret %s, got %s", testClientSecret, creds.ClientSecret)
	}
}

// Test getAccountCredentials - secret not found
func TestGetAccountCredentials_NotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.getAccountCredentials()
	if err == nil {
		t.Fatal("Expected error for missing secret, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// Test getAccountCredentials - empty USERNAME
func TestGetAccountCredentials_EmptyUsername(t *testing.T) {
	secret := createAccountSecret(testNamespace)
	secret.Data["USERNAME"] = []byte("")
	client := fake.NewSimpleClientset(secret)
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.getAccountCredentials()
	if err == nil {
		t.Fatal("Expected error for empty USERNAME, got nil")
	}

	if !strings.Contains(err.Error(), "USERNAME is empty") {
		t.Errorf("Expected 'USERNAME is empty' error, got: %v", err)
	}
}

// Test getAccountCredentials - empty PASSWORD
func TestGetAccountCredentials_EmptyPassword(t *testing.T) {
	secret := createAccountSecret(testNamespace)
	secret.Data["PASSWORD"] = []byte("")
	client := fake.NewSimpleClientset(secret)
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.getAccountCredentials()
	if err == nil {
		t.Fatal("Expected error for empty PASSWORD, got nil")
	}

	if !strings.Contains(err.Error(), "PASSWORD is empty") {
		t.Errorf("Expected 'PASSWORD is empty' error, got: %v", err)
	}
}

// Test getAccountCredentials - empty CLIENT_ID
func TestGetAccountCredentials_EmptyClientID(t *testing.T) {
	secret := createAccountSecret(testNamespace)
	secret.Data["CLIENT_ID"] = []byte("")
	client := fake.NewSimpleClientset(secret)
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.getAccountCredentials()
	if err == nil {
		t.Fatal("Expected error for empty CLIENT_ID, got nil")
	}

	if !strings.Contains(err.Error(), "CLIENT_ID is empty") {
		t.Errorf("Expected 'CLIENT_ID is empty' error, got: %v", err)
	}
}

// Test getAccountCredentials - empty CLIENT_SECRET
func TestGetAccountCredentials_EmptyClientSecret(t *testing.T) {
	secret := createAccountSecret(testNamespace)
	secret.Data["CLIENT_SECRET"] = []byte("")
	client := fake.NewSimpleClientset(secret)
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.getAccountCredentials()
	if err == nil {
		t.Fatal("Expected error for empty CLIENT_SECRET, got nil")
	}

	if !strings.Contains(err.Error(), "CLIENT_SECRET is empty") {
		t.Errorf("Expected 'CLIENT_SECRET is empty' error, got: %v", err)
	}
}

// Test GetCurrentAccessToken - success
func TestGetCurrentAccessToken_Success(t *testing.T) {
	client := fake.NewSimpleClientset(createTokensSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	token, err := tm.GetCurrentAccessToken()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token != testAccessToken {
		t.Errorf("Expected token %s, got %s", testAccessToken, token)
	}
}

// Test GetCurrentAccessToken - secret not found
func TestGetCurrentAccessToken_NotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.GetCurrentAccessToken()
	if err == nil {
		t.Fatal("Expected error for missing secret, got nil")
	}
}

// Test GetCurrentAccessToken - empty token
func TestGetCurrentAccessToken_EmptyToken(t *testing.T) {
	secret := createTokensSecret(testNamespace)
	secret.Data["ACCESS_TOKEN"] = []byte("")
	client := fake.NewSimpleClientset(secret)
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.GetCurrentAccessToken()
	if err == nil {
		t.Fatal("Expected error for empty token, got nil")
	}

	if !strings.Contains(err.Error(), "ACCESS_TOKEN is empty") {
		t.Errorf("Expected 'ACCESS_TOKEN is empty' error, got: %v", err)
	}
}

// Test GetCurrentRefreshToken - success
func TestGetCurrentRefreshToken_Success(t *testing.T) {
	client := fake.NewSimpleClientset(createTokensSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	token, err := tm.GetCurrentRefreshToken()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if token != testRefreshToken {
		t.Errorf("Expected token %s, got %s", testRefreshToken, token)
	}
}

// Test GetCurrentRefreshToken - empty token
func TestGetCurrentRefreshToken_EmptyToken(t *testing.T) {
	secret := createTokensSecret(testNamespace)
	secret.Data["REFRESH_TOKEN"] = []byte("")
	client := fake.NewSimpleClientset(secret)
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	_, err := tm.GetCurrentRefreshToken()
	if err == nil {
		t.Fatal("Expected error for empty token, got nil")
	}

	if !strings.Contains(err.Error(), "REFRESH_TOKEN is empty") {
		t.Errorf("Expected 'REFRESH_TOKEN is empty' error, got: %v", err)
	}
}

// Test UpdateAccessToken - success
func TestUpdateAccessToken_Success(t *testing.T) {
	client := fake.NewSimpleClientset(createTokensSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	newToken := "new-access-token-99999"
	err := tm.UpdateAccessToken(newToken)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify the token was updated
	updatedToken, err := tm.GetCurrentAccessToken()
	if err != nil {
		t.Fatalf("Failed to get updated token: %v", err)
	}

	if updatedToken != newToken {
		t.Errorf("Expected updated token %s, got %s", newToken, updatedToken)
	}
}

// Test createOrUpdateTokensSecret - create new secret
func TestCreateOrUpdateTokensSecret_Create(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	tokens := &NicruTokens{
		AccessToken:  testAccessToken,
		RefreshToken: testRefreshToken,
		AppID:        testClientID,
		AppSecret:    testClientSecret,
	}

	err := tm.createOrUpdateTokensSecret(tokens)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify secret was created
	secret, err := client.CoreV1().Secrets(testNamespace).Get(
		context.Background(), nameTokensSecret, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get created secret: %v", err)
	}

	if string(secret.Data["ACCESS_TOKEN"]) != testAccessToken {
		t.Errorf("Expected ACCESS_TOKEN %s, got %s", testAccessToken, secret.Data["ACCESS_TOKEN"])
	}
	if string(secret.Data["REFRESH_TOKEN"]) != testRefreshToken {
		t.Errorf("Expected REFRESH_TOKEN %s, got %s", testRefreshToken, secret.Data["REFRESH_TOKEN"])
	}
	if string(secret.Data["APP_ID"]) != testClientID {
		t.Errorf("Expected APP_ID %s, got %s", testClientID, secret.Data["APP_ID"])
	}
	if string(secret.Data["APP_SECRET"]) != testClientSecret {
		t.Errorf("Expected APP_SECRET %s, got %s", testClientSecret, secret.Data["APP_SECRET"])
	}

	// Verify labels
	if secret.Labels["managed-by"] != "webhook" {
		t.Errorf("Expected label managed-by=webhook, got %s", secret.Labels["managed-by"])
	}
}

// Test createOrUpdateTokensSecret - update existing secret
func TestCreateOrUpdateTokensSecret_Update(t *testing.T) {
	client := fake.NewSimpleClientset(createTokensSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	newTokens := &NicruTokens{
		AccessToken:  testNewAccessToken,
		RefreshToken: "new-refresh-token",
		AppID:        "new-app-id",
		AppSecret:    "new-app-secret",
	}

	err := tm.createOrUpdateTokensSecret(newTokens)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify secret was updated
	secret, err := client.CoreV1().Secrets(testNamespace).Get(
		context.Background(), nameTokensSecret, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated secret: %v", err)
	}

	if string(secret.Data["ACCESS_TOKEN"]) != testNewAccessToken {
		t.Errorf("Expected updated ACCESS_TOKEN %s, got %s", testNewAccessToken, secret.Data["ACCESS_TOKEN"])
	}
}

// Test requestOAuthTokens with mock HTTP server
func TestRequestOAuthTokens_Success(t *testing.T) {
	// Create mock OAuth server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s",
				r.Header.Get("Content-Type"))
		}

		// Parse form data
		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		// Verify form fields
		if r.FormValue("grant_type") != "password" {
			t.Errorf("Expected grant_type=password, got %s", r.FormValue("grant_type"))
		}
		if r.FormValue("username") != testUsername {
			t.Errorf("Expected username=%s, got %s", testUsername, r.FormValue("username"))
		}

		// Return success response
		response := NicruTokens{
			AccessToken:  testAccessToken,
			RefreshToken: testRefreshToken,
			ExpiresIn:    14400,
			TokenType:    "Bearer",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Temporarily override oauthUrl

	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	creds := &AccountCredentials{
		Username:     testUsername,
		Password:     testPassword,
		ClientID:     testClientID,
		ClientSecret: testClientSecret,
	}

	tokens, err := tm.requestOAuthTokens(creds)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if tokens.AccessToken != testAccessToken {
		t.Errorf("Expected access token %s, got %s", testAccessToken, tokens.AccessToken)
	}
	if tokens.RefreshToken != testRefreshToken {
		t.Errorf("Expected refresh token %s, got %s", testRefreshToken, tokens.RefreshToken)
	}
}

// Test requestOAuthTokens - API error
func TestRequestOAuthTokens_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_credentials"}`))
	}))
	defer server.Close()

	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	creds := &AccountCredentials{
		Username:     testUsername,
		Password:     testPassword,
		ClientID:     testClientID,
		ClientSecret: testClientSecret,
	}

	_, err := tm.requestOAuthTokens(creds)
	if err == nil {
		t.Fatal("Expected error for 401 response, got nil")
	}

	if !strings.Contains(err.Error(), "OAuth failed") {
		t.Errorf("Expected 'OAuth failed' error, got: %v", err)
	}
}

// Test RefreshAccessToken with mock server
func TestRefreshAccessToken_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("Expected grant_type=refresh_token, got %s", r.FormValue("grant_type"))
		}

		response := NicruTokens{
			AccessToken: testNewAccessToken,
			ExpiresIn:   14400,
			TokenType:   "Bearer",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := fake.NewSimpleClientset(createTokensSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	newTokens, err := tm.RefreshAccessToken(testRefreshToken)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if newTokens.AccessToken != testNewAccessToken {
		t.Errorf("Expected new access token %s, got %s", testNewAccessToken, newTokens.AccessToken)
	}

	// Verify refresh_token is preserved when not returned by server
	if newTokens.RefreshToken != testRefreshToken {
		t.Errorf("Expected refresh token to be preserved as %s, got %s", testRefreshToken, newTokens.RefreshToken)
	}
}

// Test RefreshAccessToken - failure
func TestRefreshAccessToken_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	client := fake.NewSimpleClientset(createTokensSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	_, err := tm.RefreshAccessToken(testRefreshToken)
	if err == nil {
		t.Fatal("Expected error for failed refresh, got nil")
	}

	if !strings.Contains(err.Error(), "refresh failed") {
		t.Errorf("Expected 'refresh failed' error, got: %v", err)
	}
}

// Test ValidateAccessToken - valid token
func TestValidateAccessToken_Valid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response status="success"></response>`))
	}))
	defer server.Close()

	// Override urlGetZoneInfo

	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	valid, err := tm.ValidateAccessToken(testAccessToken)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !valid {
		t.Error("Expected token to be valid")
	}
}

// Test ValidateAccessToken - invalid token (401)
func TestValidateAccessToken_Invalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer server.Close()

	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	valid, err := tm.ValidateAccessToken(testAccessToken)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if valid {
		t.Error("Expected token to be invalid")
	}
}

// Test retryWithBackoff - success on first attempt
func TestRetryWithBackoff_SuccessFirstAttempt(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	attemptCount := 0
	operation := func() error {
		attemptCount++
		return nil
	}

	err := tm.retryWithBackoff(operation, 3)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt, got %d", attemptCount)
	}
}

// Test retryWithBackoff - success on second attempt
func TestRetryWithBackoff_SuccessSecondAttempt(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	attemptCount := 0
	operation := func() error {
		attemptCount++
		if attemptCount < 2 {
			return fmt.Errorf("temporary error")
		}
		return nil
	}

	startTime := time.Now()
	err := tm.retryWithBackoff(operation, 3)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}

	// Should have waited ~2 seconds (first backoff)
	if duration < 2*time.Second {
		t.Errorf("Expected at least 2s delay, got %v", duration)
	}
}

// Test retryWithBackoff - all attempts fail
func TestRetryWithBackoff_AllFail(t *testing.T) {
	client := fake.NewSimpleClientset()
	tm := NewTokenManager(client, testNamespace, createTestConfig(nil))

	attemptCount := 0
	testError := fmt.Errorf("persistent error")
	operation := func() error {
		attemptCount++
		return testError
	}

	err := tm.retryWithBackoff(operation, 3)
	if err == nil {
		t.Fatal("Expected error after all retries, got nil")
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	if !strings.Contains(err.Error(), "all 3 attempts failed") {
		t.Errorf("Expected 'all 3 attempts failed' error, got: %v", err)
	}
}

// Test maskToken
func TestMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal token",
			input:    "abcdefghijklmnopqrstuvwxyz",
			expected: "abcd...wxyz",
		},
		{
			name:     "short token",
			input:    "short",
			expected: "***",
		},
		{
			name:     "exactly 12 chars",
			input:    "123456789012",
			expected: "1234...9012",
		},
		{
			name:     "empty token",
			input:    "",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskToken(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// Test maskSensitiveData
func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "masks password",
			input:    "user password is secret123",
			contains: "***password***",
		},
		{
			name:     "masks token",
			input:    "the token is abc123",
			contains: "***token***",
		},
		{
			name:     "masks secret",
			input:    "client secret value",
			contains: "***secret***",
		},
		{
			name:     "masks refresh_token",
			input:    "refresh_token: xyz789",
			contains: "***token***", // 'token' is masked within 'refresh_token'
		},
		{
			name:     "masks access_token",
			input:    "access_token: abc456",
			contains: "***token***", // 'token' is masked within 'access_token'
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveData(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %s, got %s", tt.contains, result)
			}
		})
	}
}

// Test EnsureValidTokens - tokens not found
func TestEnsureValidTokens_NotFound(t *testing.T) {
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

	// Create client with only account secret (no tokens secret)
	client := fake.NewSimpleClientset(createAccountSecret(testNamespace))
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	err := tm.EnsureValidTokens()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify tokens secret was created
	secret, err := client.CoreV1().Secrets(testNamespace).Get(
		context.Background(), nameTokensSecret, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected tokens secret to be created, got error: %v", err)
	}

	if string(secret.Data["ACCESS_TOKEN"]) != testAccessToken {
		t.Errorf("Expected ACCESS_TOKEN to be set")
	}
}

// Test EnsureValidTokens - valid tokens exist
func TestEnsureValidTokens_ValidTokensExist(t *testing.T) {
	// Mock validation server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	client := fake.NewSimpleClientset(
		createAccountSecret(testNamespace),
		createTokensSecret(testNamespace),
	)
	tm := NewTokenManager(client, testNamespace, createTestConfig(server))

	err := tm.EnsureValidTokens()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify tokens were not changed
	token, _ := tm.GetCurrentAccessToken()
	if token != testAccessToken {
		t.Error("Token should not have changed")
	}
}
