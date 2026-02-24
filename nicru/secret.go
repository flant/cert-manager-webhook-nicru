package nicru

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	refreshMarginRatio  = 0.75
	minRefreshInterval  = 5 * time.Minute
	maxRetryInterval    = 10 * time.Minute
	maxConsecutiveFails = 5
)

func (s *Solver) getAccessToken() (string, error) {
	secret, err := s.k8s.CoreV1().Secrets(s.namespace).Get(
		context.Background(), s.secretName, metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("read secret %s/%s: %w", s.namespace, s.secretName, err)
	}

	token := strings.TrimSpace(string(secret.Data["ACCESS_TOKEN"]))
	if token == "" {
		return "", fmt.Errorf("ACCESS_TOKEN is empty in secret %s/%s", s.namespace, s.secretName)
	}
	return token, nil
}

func (s *Solver) getRefreshToken() (string, error) {
	secret, err := s.k8s.CoreV1().Secrets(s.namespace).Get(
		context.Background(), s.secretName, metav1.GetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("read secret %s/%s: %w", s.namespace, s.secretName, err)
	}

	token := strings.TrimSpace(string(secret.Data["REFRESH_TOKEN"]))
	if token == "" {
		return "", fmt.Errorf("REFRESH_TOKEN is empty in secret %s/%s", s.namespace, s.secretName)
	}
	return token, nil
}

func (s *Solver) getAppCredentials() (string, string, error) {
	secret, err := s.k8s.CoreV1().Secrets(s.namespace).Get(
		context.Background(), s.secretName, metav1.GetOptions{},
	)
	if err != nil {
		return "", "", fmt.Errorf("read secret %s/%s: %w", s.namespace, s.secretName, err)
	}

	appID := strings.TrimSpace(string(secret.Data["APP_ID"]))
	appSecret := strings.TrimSpace(string(secret.Data["APP_SECRET"]))
	if appID == "" || appSecret == "" {
		return "", "", fmt.Errorf("APP_ID or APP_SECRET is empty in secret %s/%s", s.namespace, s.secretName)
	}
	return appID, appSecret, nil
}

func (s *Solver) patchTokenSecret(accessToken, refreshToken string) error {
	patch := v1.Secret{
		Data: map[string][]byte{
			"ACCESS_TOKEN":  []byte(accessToken),
			"REFRESH_TOKEN": []byte(refreshToken),
		},
	}

	payload, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal secret patch: %w", err)
	}

	_, err = s.k8s.CoreV1().Secrets(s.namespace).Patch(
		context.Background(), s.secretName,
		types.StrategicMergePatchType, payload, metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("patch secret %s/%s: %w", s.namespace, s.secretName, err)
	}
	return nil
}

func (s *Solver) runTokenManager(stopCh <-chan struct{}) {
	s.logger.Info("token manager started, validating current access token")

	expiresIn, err := s.startupTokenCheck()
	if err != nil {
		s.logger.Error("both access and refresh tokens are invalid, cannot operate - exiting",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	refreshInterval := calculateRefreshInterval(expiresIn)
	s.logger.Info("token refresh scheduled",
		"refresh_in", refreshInterval.String(),
		"token_expires_in_sec", expiresIn,
	)

	consecutiveFails := 0
	timer := time.NewTimer(refreshInterval)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			s.logger.Info("scheduled token refresh starting")
			newExpiresIn, err := s.refreshTokensWithExpiry()
			if err != nil {
				consecutiveFails++
				retryIn := retryInterval(consecutiveFails)
				s.logger.Error("scheduled token refresh failed, will retry",
					"error", err.Error(),
					"consecutive_failures", consecutiveFails,
					"retry_in", retryIn.String(),
				)
				if consecutiveFails >= maxConsecutiveFails {
					s.logger.Error("too many consecutive refresh failures, webhook is non-functional - exiting",
						"consecutive_failures", consecutiveFails,
					)
					os.Exit(1)
				}
				timer.Reset(retryIn)
			} else {
				consecutiveFails = 0
				nextRefresh := calculateRefreshInterval(newExpiresIn)
				s.logger.Info("scheduled token refresh completed",
					"token_expires_in_sec", newExpiresIn,
					"next_refresh_in", nextRefresh.String(),
				)
				timer.Reset(nextRefresh)
			}
		case <-stopCh:
			s.logger.Info("token manager stopped")
			return
		}
	}
}

func (s *Solver) startupTokenCheck() (int, error) {
	if err := s.validateToken(); err != nil {
		s.logger.Warn("access token is invalid or expired, will try to refresh",
			"error", err.Error(),
		)
	} else {
		s.logger.Info("current access token is valid")
	}

	s.logger.Info("refreshing tokens on startup to get a fresh token and known expiry")
	expiresIn, err := s.refreshTokensWithExpiry()
	if err != nil {
		return 0, err
	}
	s.logger.Info("tokens refreshed on startup, webhook is ready",
		"expires_in", expiresIn,
	)
	return expiresIn, nil
}

func (s *Solver) validateToken() error {
	token, err := s.getAccessToken()
	if err != nil {
		return err
	}
	return s.api.ValidateToken(token)
}

func (s *Solver) refreshTokensWithExpiry() (int, error) {
	start := time.Now()

	refreshToken, err := s.getRefreshToken()
	if err != nil {
		return 0, fmt.Errorf("get refresh token: %w", err)
	}

	appID, appSecret, err := s.getAppCredentials()
	if err != nil {
		return 0, fmt.Errorf("get app credentials: %w", err)
	}

	newTokens, err := s.api.RefreshToken(appID, appSecret, refreshToken)
	if err != nil {
		return 0, fmt.Errorf("oauth token exchange: %w", err)
	}

	if err := s.patchTokenSecret(newTokens.AccessToken, newTokens.RefreshToken); err != nil {
		return 0, fmt.Errorf("save tokens to secret: %w", err)
	}

	s.logger.Info("new tokens saved to k8s secret",
		"secret", s.secretName,
		"namespace", s.namespace,
		"expires_in", newTokens.ExpiresIn,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return newTokens.ExpiresIn, nil
}

func calculateRefreshInterval(expiresInSec int) time.Duration {
	d := time.Duration(float64(time.Duration(expiresInSec)*time.Second) * refreshMarginRatio)
	if d < minRefreshInterval {
		return minRefreshInterval
	}
	return d
}

func retryInterval(consecutiveFails int) time.Duration {
	d := time.Duration(1<<uint(consecutiveFails)) * 30 * time.Second
	if d > maxRetryInterval {
		return maxRetryInterval
	}
	return d
}
