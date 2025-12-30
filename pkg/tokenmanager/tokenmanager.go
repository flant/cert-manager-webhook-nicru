// Package tokenmanager provides OAuth token lifecycle management for NIC.RU API.
// It handles token acquisition from account credentials, validation, refresh,
// and storage in Kubernetes secrets with automatic retry and metrics support.
package tokenmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	nameAccountSecret = "nicru-account"
	nameTokensSecret  = "nicru-tokens"
)

// Config holds external dependencies for TokenManager
type Config struct {
	HTTPClient       *http.Client
	OAuthURL         string
	ZoneInfoURL      string
	MetricsCallbacks MetricsCallbacks
}

// MetricsCallbacks holds optional metric recording functions
type MetricsCallbacks struct {
	IncTokenRefreshTotal       func()
	IncTokenRefreshErrors      func()
	IncTokenValidationTotal    func()
	SetLastTokenRefreshTime    func()
	SetLastTokenValidationTime func()
	SetTokenValidationStatus   func(float64)
}

// TokenManager manages OAuth token lifecycle
type TokenManager struct {
	client    kubernetes.Interface
	namespace string
	config    Config
}

// NewTokenManager creates a new TokenManager
func NewTokenManager(client kubernetes.Interface, namespace string, config Config) *TokenManager {
	return &TokenManager{
		client:    client,
		namespace: namespace,
		config:    config,
	}
}

// EnsureValidTokens ensures tokens are available and valid
func (tm *TokenManager) EnsureValidTokens() error {
	klog.Info("Ensuring valid tokens...")

	// Try to get existing tokens secret
	tokensSecret, err := tm.getTokensSecret()
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Info("nicru-tokens secret not found, initializing from account...")
			return tm.initializeTokensFromAccount()
		}
		return fmt.Errorf("failed to get tokens secret: %w", err)
	}

	// Check if tokens are present
	refreshToken := string(tokensSecret.Data["REFRESH_TOKEN"])
	accessToken := string(tokensSecret.Data["ACCESS_TOKEN"])

	if refreshToken == "" || accessToken == "" {
		klog.Info("Tokens are empty, initializing from account...")
		return tm.initializeTokensFromAccount()
	}

	// Validate access token
	klog.Info("Validating access token...")
	valid, err := tm.ValidateAccessToken(accessToken)
	if err != nil {
		return fmt.Errorf("failed to validate token: %w", err)
	}

	if valid {
		klog.Info("Access token is valid")
		if tm.config.MetricsCallbacks.SetTokenValidationStatus != nil {
			tm.config.MetricsCallbacks.SetTokenValidationStatus(1)
		}
		return nil
	}

	// Token is invalid, try to refresh
	klog.Info("Access token expired, attempting refresh...")
	if tm.config.MetricsCallbacks.SetTokenValidationStatus != nil {
		tm.config.MetricsCallbacks.SetTokenValidationStatus(0)
	}

	newTokens, err := tm.RefreshAccessToken(refreshToken)
	if err != nil {
		klog.Warningf("Failed to refresh token: %v, getting new tokens from account", err)
		return tm.initializeTokensFromAccount()
	}

	// Update secret with new tokens (access + potentially new refresh)
	tokensSecret.Data["ACCESS_TOKEN"] = []byte(newTokens.AccessToken)
	if newTokens.RefreshToken != "" {
		tokensSecret.Data["REFRESH_TOKEN"] = []byte(newTokens.RefreshToken)
	}

	_, err = tm.client.CoreV1().Secrets(tm.namespace).Update(
		context.Background(), tokensSecret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	klog.Info("Tokens refreshed successfully")
	if tm.config.MetricsCallbacks.SetTokenValidationStatus != nil {
		tm.config.MetricsCallbacks.SetTokenValidationStatus(1)
	}
	return nil
}

// ValidateAccessToken validates token by making test API request
func (tm *TokenManager) ValidateAccessToken(token string) (bool, error) {
	url := fmt.Sprintf(tm.config.ZoneInfoURL, maskToken(token))
	klog.V(2).Infof("Validating token with URL: %s", url)

	url = fmt.Sprintf(tm.config.ZoneInfoURL, token) // Use real token for request

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := tm.config.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if tm.config.MetricsCallbacks.IncTokenValidationTotal != nil {
		tm.config.MetricsCallbacks.IncTokenValidationTotal()
	}
	if tm.config.MetricsCallbacks.SetLastTokenValidationTime != nil {
		tm.config.MetricsCallbacks.SetLastTokenValidationTime()
	}

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return false, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, maskSensitiveData(string(body)))
}

// initializeTokensFromAccount gets new tokens using nicru-account credentials
func (tm *TokenManager) initializeTokensFromAccount() error {
	klog.Info("Initializing tokens from nicru-account...")

	var tokens *NicruTokens

	operation := func() error {
		// Get account credentials
		creds, err := tm.getAccountCredentials()
		if err != nil {
			return err
		}

		// Request OAuth tokens
		tokens, err = tm.requestOAuthTokens(creds)
		if err != nil {
			return err
		}

		// Add app credentials to tokens
		tokens.AppID = creds.ClientID
		tokens.AppSecret = creds.ClientSecret

		return nil
	}

	// Retry with backoff
	err := tm.retryWithBackoff(operation, 3)
	if err != nil {
		if tm.config.MetricsCallbacks.IncTokenRefreshErrors != nil {
			tm.config.MetricsCallbacks.IncTokenRefreshErrors()
		}
		return fmt.Errorf("failed to get tokens after retries: %w", err)
	}

	// Create or update tokens secret
	if err := tm.createOrUpdateTokensSecret(tokens); err != nil {
		return fmt.Errorf("failed to save tokens: %w", err)
	}

	if tm.config.MetricsCallbacks.IncTokenRefreshTotal != nil {
		tm.config.MetricsCallbacks.IncTokenRefreshTotal()
	}
	if tm.config.MetricsCallbacks.SetLastTokenRefreshTime != nil {
		tm.config.MetricsCallbacks.SetLastTokenRefreshTime()
	}
	if tm.config.MetricsCallbacks.SetTokenValidationStatus != nil {
		tm.config.MetricsCallbacks.SetTokenValidationStatus(1)
	}

	klog.Info("Tokens initialized successfully")
	return nil
}

// getAccountCredentials retrieves credentials from nicru-account secret
func (tm *TokenManager) getAccountCredentials() (*AccountCredentials, error) {
	secret, err := tm.client.CoreV1().Secrets(tm.namespace).Get(
		context.Background(), nameAccountSecret, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("nicru-account secret not found: %w", err)
	}

	creds := &AccountCredentials{
		Username:     string(secret.Data["USERNAME"]),
		Password:     string(secret.Data["PASSWORD"]),
		ClientID:     string(secret.Data["CLIENT_ID"]),
		ClientSecret: string(secret.Data["CLIENT_SECRET"]),
	}

	// Validate required fields
	if creds.Username == "" {
		return nil, fmt.Errorf("USERNAME is empty in nicru-account")
	}
	if creds.Password == "" {
		return nil, fmt.Errorf("PASSWORD is empty in nicru-account")
	}
	if creds.ClientID == "" {
		return nil, fmt.Errorf("CLIENT_ID is empty in nicru-account")
	}
	if creds.ClientSecret == "" {
		return nil, fmt.Errorf("CLIENT_SECRET is empty in nicru-account")
	}

	klog.V(2).Infof("Retrieved credentials for user: %s", creds.Username)
	return creds, nil
}

// requestOAuthTokens requests tokens from NIC.RU OAuth API
func (tm *TokenManager) requestOAuthTokens(creds *AccountCredentials) (*NicruTokens, error) {
	klog.Info("Requesting OAuth tokens from NIC.RU API...")

	params := fmt.Sprintf(
		"grant_type=password&username=%s&password=%s&client_id=%s&client_secret=%s&scope=.*&offline=999999",
		url.QueryEscape(creds.Username),
		url.QueryEscape(creds.Password),
		url.QueryEscape(creds.ClientID),
		url.QueryEscape(creds.ClientSecret),
	)

	payload := strings.NewReader(params)
	req, err := http.NewRequest(http.MethodPost, tm.config.OAuthURL, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := tm.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth failed: status=%d, body=%s",
			resp.StatusCode, maskSensitiveData(string(body)))
	}

	var tokens NicruTokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("failed to parse OAuth response: %w", err)
	}

	klog.Info("OAuth tokens received successfully")
	return &tokens, nil
}

// RefreshAccessToken refreshes access token using refresh token
func (tm *TokenManager) RefreshAccessToken(refreshToken string) (*NicruTokens, error) {
	klog.Info("Refreshing access token...")

	// Get app credentials from tokens secret
	tokensSecret, err := tm.getTokensSecret()
	if err != nil {
		return nil, err
	}

	appID := string(tokensSecret.Data["APP_ID"])
	appSecret := string(tokensSecret.Data["APP_SECRET"])

	params := fmt.Sprintf(
		"grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s",
		url.QueryEscape(refreshToken),
		url.QueryEscape(appID),
		url.QueryEscape(appSecret),
	)

	payload := strings.NewReader(params)
	req, err := http.NewRequest(http.MethodPost, tm.config.OAuthURL, payload)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := tm.config.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh failed: status=%d", resp.StatusCode)
	}

	var tokens NicruTokens
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, err
	}

	// NIC.RU uses rotating refresh tokens
	if tokens.RefreshToken == "" {
		// This should never happen with NIC.RU API, but keep as safety net
		klog.Warning("NIC.RU response missing refresh_token, reusing existing (unexpected behavior)")
		tokens.RefreshToken = refreshToken
	}

	// Log token rotation at debug level since it's normal behavior
	klog.V(2).Infof("Received new refresh_token (old: %s..., new: %s...)",
		maskToken(refreshToken), maskToken(tokens.RefreshToken))

	klog.Info("Access token refreshed successfully")
	return &tokens, nil
}

// createOrUpdateTokensSecret creates or updates nicru-tokens secret
func (tm *TokenManager) createOrUpdateTokensSecret(tokens *NicruTokens) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameTokensSecret,
			Namespace: tm.namespace,
			Labels: map[string]string{
				"app":        "cert-manager-webhook-nicru",
				"managed-by": "webhook",
			},
		},
		Type: v1.SecretTypeOpaque,
		Data: map[string][]byte{
			"APP_ID":        []byte(tokens.AppID),
			"APP_SECRET":    []byte(tokens.AppSecret),
			"REFRESH_TOKEN": []byte(tokens.RefreshToken),
			"ACCESS_TOKEN":  []byte(tokens.AccessToken),
		},
	}

	// Try to get existing secret
	existing, err := tm.client.CoreV1().Secrets(tm.namespace).Get(
		context.Background(), nameTokensSecret, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			// Create new secret
			_, err = tm.client.CoreV1().Secrets(tm.namespace).Create(
				context.Background(), secret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create secret: %w", err)
			}
			klog.Info("Created nicru-tokens secret")
			return nil
		}
		return err
	}

	// Update existing secret
	existing.Data = secret.Data
	_, err = tm.client.CoreV1().Secrets(tm.namespace).Update(
		context.Background(), existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret: %w", err)
	}

	klog.Info("Updated nicru-tokens secret")
	return nil
}

// getTokensSecret retrieves nicru-tokens secret
func (tm *TokenManager) getTokensSecret() (*v1.Secret, error) {
	return tm.client.CoreV1().Secrets(tm.namespace).Get(
		context.Background(), nameTokensSecret, metav1.GetOptions{})
}

// GetCurrentAccessToken returns current access token
func (tm *TokenManager) GetCurrentAccessToken() (string, error) {
	secret, err := tm.getTokensSecret()
	if err != nil {
		return "", err
	}

	token := string(secret.Data["ACCESS_TOKEN"])
	if token == "" {
		return "", fmt.Errorf("ACCESS_TOKEN is empty")
	}

	return token, nil
}

// GetCurrentRefreshToken returns current refresh token
func (tm *TokenManager) GetCurrentRefreshToken() (string, error) {
	secret, err := tm.getTokensSecret()
	if err != nil {
		return "", err
	}

	token := string(secret.Data["REFRESH_TOKEN"])
	if token == "" {
		return "", fmt.Errorf("REFRESH_TOKEN is empty")
	}

	return token, nil
}

// UpdateAccessToken updates only the access token in secret
func (tm *TokenManager) UpdateAccessToken(accessToken string) error {
	secret, err := tm.getTokensSecret()
	if err != nil {
		return err
	}

	secret.Data["ACCESS_TOKEN"] = []byte(accessToken)

	_, err = tm.client.CoreV1().Secrets(tm.namespace).Update(
		context.Background(), secret, metav1.UpdateOptions{})

	return err
}

// UpdateTokens updates both access and refresh tokens in secret
func (tm *TokenManager) UpdateTokens(tokens *NicruTokens) error {
	secret, err := tm.getTokensSecret()
	if err != nil {
		return err
	}

	secret.Data["ACCESS_TOKEN"] = []byte(tokens.AccessToken)

	// Update refresh_token if provided
	if tokens.RefreshToken != "" {
		secret.Data["REFRESH_TOKEN"] = []byte(tokens.RefreshToken)
	}

	_, err = tm.client.CoreV1().Secrets(tm.namespace).Update(
		context.Background(), secret, metav1.UpdateOptions{})

	return err
}

// retryWithBackoff executes operation with exponential backoff
func (tm *TokenManager) retryWithBackoff(operation func() error, maxAttempts int) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt < maxAttempts {
			// Exponential backoff: 2s, 6s, 18s
			backoff := time.Duration(2*math.Pow(3, float64(attempt-1))) * time.Second
			klog.Warningf("Attempt %d/%d failed: %v, retrying in %v...",
				attempt, maxAttempts, err, backoff)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("all %d attempts failed, last error: %w", maxAttempts, lastErr)
}

// maskToken masks token for logging (shows first/last 4 chars)
func maskToken(token string) string {
	if len(token) < 12 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// maskSensitiveData masks sensitive data in strings for logging
func maskSensitiveData(data string) string {
	// Mask common sensitive fields
	sensitive := []string{"password", "token", "secret", "refresh_token", "access_token"}
	result := data
	for _, s := range sensitive {
		if strings.Contains(strings.ToLower(result), s) {
			result = strings.ReplaceAll(result, s, "***"+s+"***")
		}
	}
	return result
}
