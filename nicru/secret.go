package nicru

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

	if err := s.validateToken(); err != nil {
		s.logger.Warn("access token is invalid or expired, will try to refresh",
			"error", err.Error(),
		)
		if err := s.refreshTokens(); err != nil {
			s.logger.Error("failed to refresh token on startup, webhook may not work until next retry",
				"error", err.Error(),
			)
		} else {
			s.logger.Info("token refreshed successfully on startup, webhook is ready")
		}
	} else {
		s.logger.Info("access token is valid, webhook is ready")
	}

	ticker := time.NewTicker(3 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.logger.Info("scheduled token refresh starting")
			if err := s.refreshTokens(); err != nil {
				s.logger.Error("scheduled token refresh failed", "error", err.Error())
			} else {
				s.logger.Info("scheduled token refresh completed")
			}
		case <-stopCh:
			s.logger.Info("token manager stopped")
			return
		}
	}
}

func (s *Solver) validateToken() error {
	token, err := s.getAccessToken()
	if err != nil {
		return err
	}
	return s.api.ValidateToken(token)
}

func (s *Solver) refreshTokens() error {
	start := time.Now()

	refreshToken, err := s.getRefreshToken()
	if err != nil {
		return fmt.Errorf("get refresh token: %w", err)
	}

	appID, appSecret, err := s.getAppCredentials()
	if err != nil {
		return fmt.Errorf("get app credentials: %w", err)
	}

	newTokens, err := s.api.RefreshToken(appID, appSecret, refreshToken)
	if err != nil {
		return fmt.Errorf("oauth token exchange: %w", err)
	}

	if err := s.patchTokenSecret(newTokens.AccessToken, newTokens.RefreshToken); err != nil {
		return fmt.Errorf("save tokens to secret: %w", err)
	}

	s.logger.Info("new tokens saved to k8s secret",
		"secret", s.secretName,
		"namespace", s.namespace,
		"expires_in", newTokens.ExpiresIn,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}
