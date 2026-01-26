//go:build integration

package integration

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestCredentials contains NIC.RU account credentials from environment
type TestCredentials struct {
	Username     string
	Password     string
	ClientID     string
	ClientSecret string
	TestZone     string // REQUIRED for DNS tests: DNS zone name for testing
}

// NicruTokens represents OAuth tokens from NIC.RU API
type NicruTokens struct {
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
}

// ZoneInfo contains basic zone information
type ZoneInfo struct {
	Name    string
	Service string
	ID      string
}

// Shared state between tests (package-level variables)
var (
	// sharedTokens stores tokens obtained in first test for reuse
	sharedTokens *NicruTokens

	// sharedZones stores user zones for DNS tests
	sharedZones []ZoneInfo

	// testRecordID stores created test record ID for cleanup
	testRecordID string

	// testServiceName and testZoneName for DNS operations
	testServiceName string
	testZoneName    string
)

// LoadCredentials loads credentials from environment variables
// Returns nil if any required credential is missing
func LoadCredentials() *TestCredentials {
	username := os.Getenv("NICRU_USERNAME")
	password := os.Getenv("NICRU_PASSWORD")
	clientID := os.Getenv("NICRU_CLIENT_ID")
	clientSecret := os.Getenv("NICRU_CLIENT_SECRET")
	testZone := os.Getenv("NICRU_TEST_ZONE")

	if username == "" || password == "" || clientID == "" || clientSecret == "" {
		return nil
	}

	return &TestCredentials{
		Username:     username,
		Password:     password,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TestZone:     testZone,
	}
}

// skipIfNoCredentials skips test if credentials are not provided
func skipIfNoCredentials(t *testing.T) *TestCredentials {
	t.Helper()
	creds := LoadCredentials()
	if creds == nil {
		t.Skip("NIC.RU credentials not set (NICRU_USERNAME, NICRU_PASSWORD, NICRU_CLIENT_ID, NICRU_CLIENT_SECRET), skipping integration test")
	}
	return creds
}

// skipIfNoTokens skips test if shared tokens are not available
func skipIfNoTokens(t *testing.T) {
	t.Helper()
	if sharedTokens == nil {
		t.Skip("No tokens available from previous test, skipping")
	}
}

// skipIfNoZones skips test if no zones are available
func skipIfNoZones(t *testing.T) {
	t.Helper()
	if len(sharedZones) == 0 {
		t.Skip("No DNS zones available on NIC.RU account, skipping DNS operation test")
	}
}

// createTestRecordName generates unique test record name with timestamp
func createTestRecordName() string {
	return fmt.Sprintf("_acme-challenge-integration-test-%d", time.Now().Unix())
}

// maskToken masks token for logging (shows first 4 and last 4 chars)
func maskToken(token string) string {
	if len(token) < 12 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// logTestStart logs test start with separator
func logTestStart(t *testing.T, description string) {
	t.Helper()
	t.Logf("\n========================================")
	t.Logf("TEST: %s", description)
	t.Logf("========================================")
}

// logTestSuccess logs test success
func logTestSuccess(t *testing.T, message string) {
	t.Helper()
	t.Logf("✅ SUCCESS: %s", message)
}

// logTestWarning logs test warning
func logTestWarning(t *testing.T, message string) {
	t.Helper()
	t.Logf("⚠️  WARNING: %s", message)
}
