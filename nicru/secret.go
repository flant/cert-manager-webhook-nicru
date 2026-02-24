package nicru

import (
	"context"
	"encoding/json"
	"fmt"
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

	token := string(secret.Data["ACCESS_TOKEN"])
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

	token := string(secret.Data["REFRESH_TOKEN"])
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

	appID := string(secret.Data["APP_ID"])
	appSecret := string(secret.Data["APP_SECRET"])
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
	s.logger.Info("token_manager_started")

	if err := s.validateToken(); err != nil {
		s.logger.Error("token_validation_failed", "error", err.Error())
		if err := s.refreshTokens(); err != nil {
			s.logger.Error("token_refresh_failed_on_startup", "error", err.Error())
		} else {
			s.logger.Info("token_refreshed_on_startup")
		}
	} else {
		s.logger.Info("token_valid")
	}

	ticker := time.NewTicker(3 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.refreshTokens(); err != nil {
				s.logger.Error("scheduled_token_refresh_failed", "error", err.Error())
			} else {
				s.logger.Info("scheduled_token_refresh_ok")
			}
		case <-stopCh:
			s.logger.Info("token_manager_stopped")
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
	s.logger.Info("token_refresh_starting")

	refreshToken, err := s.getRefreshToken()
	if err != nil {
		return fmt.Errorf("get refresh token: %w", err)
	}
	s.logger.Info("token_refresh_got_refresh_token")

	appID, appSecret, err := s.getAppCredentials()
	if err != nil {
		return fmt.Errorf("get app credentials: %w", err)
	}
	s.logger.Info("token_refresh_got_credentials", "app_id", appID)

	newTokens, err := s.api.RefreshToken(appID, appSecret, refreshToken)
	if err != nil {
		return fmt.Errorf("refresh token: %w", err)
	}
	s.logger.Info("token_refresh_received_new_tokens",
		"expires_in", newTokens.ExpiresIn,
		"token_type", newTokens.TokenType,
	)

	if err := s.patchTokenSecret(newTokens.AccessToken, newTokens.RefreshToken); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}
	s.logger.Info("token_refresh_saved_to_secret",
		"secret", s.secretName,
		"namespace", s.namespace,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return nil
}
