package dns

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/flant/cert-manager-webhook-nicru/internal/config"
	"k8s.io/klog/v2"
)

// GetServiceName retrieves the service name for a given zone
func (c *Client) GetServiceName(zoneName string) (string, error) {
	var zone Zone
	var service string

	accessToken, err := c.tokenManager.GetCurrentAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	url := fmt.Sprintf(config.URLGetZoneInfo, accessToken)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate request: %w", err)
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	err = xml.Unmarshal(body, &zone)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal body: %w", err)
	}

	for i := 0; i < len(zone.Data.Zone); i++ {
		if zone.Data.Zone[i].Name == zoneName {
			service = zone.Data.Zone[i].Service
		}
	}

	klog.Infof("Service name: %s", service)
	return service, nil
}

// CommitZone commits changes to a DNS zone
func (c *Client) CommitZone(serviceName, zoneName string) error {
	var response Response
	accessToken, err := c.tokenManager.GetCurrentAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf(config.URLCommit, serviceName, zoneName)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to generate request: %w", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	err = xml.Unmarshal(body, &response)
	if err != nil {
		return fmt.Errorf("failed to unmarshal body: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("commit zone failed: status=%s, error=%s", response.Status, response.Errors.Error.Text)
	}

	return nil
}
