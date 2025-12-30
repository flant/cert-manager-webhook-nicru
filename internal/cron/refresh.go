// Package cron provides scheduled background tasks for the webhook.
// It handles periodic OAuth token refresh to ensure continuous webhook operation.
package cron

import (
	"fmt"
	"time"

	"github.com/flant/cert-manager-webhook-nicru/internal/metrics"
	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	"github.com/go-co-op/gocron"
	"k8s.io/klog/v2"
)

// TokenRefresher handles periodic token refresh
type TokenRefresher struct {
	tokenManager *tokenmanager.TokenManager
	metrics      *metrics.Metrics
}

// NewTokenRefresher creates a new token refresher
func NewTokenRefresher(tm *tokenmanager.TokenManager, m *metrics.Metrics) *TokenRefresher {
	return &TokenRefresher{
		tokenManager: tm,
		metrics:      m,
	}
}

// Start begins the token refresh cron job
func (r *TokenRefresher) Start() {
	s := gocron.NewScheduler(time.UTC)

	// First run after 3 hours, then every 3 hours
	firstRun := time.Now().Add(3 * time.Hour)

	_, err := s.Every(3).Hours().StartAt(firstRun).Do(func() {
		err := r.updateTokens()
		if err != nil {
			klog.Error(err)
		}
	})
	if err != nil {
		klog.Errorf("cron failed: %s", err)
	}

	klog.Infof("Token refresh scheduled: first run at %s, then every 3 hours", firstRun.Format(time.RFC3339))
	s.StartBlocking()
}

func (r *TokenRefresher) updateTokens() error {
	klog.Info("Scheduled token refresh started")

	// Get current refresh token
	refreshToken, err := r.tokenManager.GetCurrentRefreshToken()
	if err != nil {
		klog.Warningf("Failed to get refresh token, attempting to initialize from account: %v", err)
		// If refresh token is not available, try to get new tokens from account
		err = r.tokenManager.EnsureValidTokens()
		if err != nil {
			r.metrics.TokenRefreshErrors.Inc()
			return fmt.Errorf("failed to ensure valid tokens: %w", err)
		}
		r.metrics.TokenRefreshTotal.Inc()
		r.metrics.LastTokenRefreshTime.SetToCurrentTime()
		return nil
	}

	// Try to refresh access token
	newTokens, err := r.tokenManager.RefreshAccessToken(refreshToken)
	if err != nil {
		klog.Warningf("Failed to refresh token: %v, attempting to initialize from account", err)
		// If refresh fails, try to get new tokens from account
		err = r.tokenManager.EnsureValidTokens()
		if err != nil {
			r.metrics.TokenRefreshErrors.Inc()
			return fmt.Errorf("failed to ensure valid tokens: %w", err)
		}
		r.metrics.TokenRefreshTotal.Inc()
		r.metrics.LastTokenRefreshTime.SetToCurrentTime()
		return nil
	}

	// Update both access and refresh tokens in the secret
	if err := r.tokenManager.UpdateTokens(newTokens); err != nil {
		r.metrics.TokenRefreshErrors.Inc()
		return fmt.Errorf("failed to update tokens: %w", err)
	}

	r.metrics.TokenRefreshTotal.Inc()
	r.metrics.LastTokenRefreshTime.SetToCurrentTime()
	klog.Info("Scheduled token refresh completed successfully")
	return nil
}
