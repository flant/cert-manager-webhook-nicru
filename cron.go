package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

const waitTime = 1 * time.Minute

func (c *DNSProviderSolver) cronUpdateToken() {
	// wait for k8s clientset initialized
	time.Sleep(waitTime)
	s := gocron.NewScheduler(time.UTC)

	_, err := s.Every(3).Hours().Do(func() {
		err := c.updateTokens()
		if err != nil {
			klog.Error(err)
		}
	})
	if err != nil {
		klog.Errorf("cron failed: %s", err)
	}

	s.StartBlocking()
}

func (c *DNSProviderSolver) updateTokens() error {
	var token NicruTokens

	currentRefreshToken := c.getRefreshToken()
	appID, appSecret := c.getAppSecrets()

	params := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s",
		currentRefreshToken, appID, appSecret)
	payload := strings.NewReader(params)

	req, err := http.NewRequest(http.MethodPost, oauthUrl, payload)
	if err != nil {
		return fmt.Errorf("failed to generate request: %w", err)
	}
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("status code not 200, got status: %d and body: %s", res.StatusCode, string(body))
	}

	defer res.Body.Close()

	err = json.Unmarshal(body, &token)
	if err != nil {
		return fmt.Errorf("failed to unmarshal tokens: %w", err)
	}

	err = c.patchSecret(token.RefreshToken, token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to update the secret: %w", err)
	}

	return nil
}

func (c *DNSProviderSolver) patchSecret(newRefreshToken, newAccessToken string) error {
	updateSecret := v1.Secret{
		Data: map[string][]byte{
			"REFRESH_TOKEN": []byte(newRefreshToken),
			"ACCESS_TOKEN":  []byte(newAccessToken),
		},
	}

	payload, err := json.Marshal(updateSecret)
	if err != nil {
		return fmt.Errorf("failed to marshal secret: %w", err)
	}

	_, err = c.client.CoreV1().Secrets(Namespace).Patch(context.Background(), nameSecret, types.StrategicMergePatchType,
		payload, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch the secret: %w", err)
	}

	return nil
}
