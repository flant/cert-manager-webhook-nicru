package nicru

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.nic.ru"

	oauthPath   = "/oauth/token"
	zonesPath   = "/dns-master/zones/"
	recordsPath = "/dns-master/services/%s/zones/%s/records"
	recordPath  = "/dns-master/services/%s/zones/%s/records/%s"
	commitPath  = "/dns-master/services/%s/zones/%s/commit"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *slog.Logger
}

func NewClient(logger *slog.Logger) *Client {
	return &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

func (c *Client) doRequest(method, path, token, contentType string, body io.Reader) ([]byte, int, error) {
	url := c.baseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	c.logger.Info("sending API request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	logBody := string(respBody)
	if strings.Contains(path, "oauth") {
		logBody = redactTokensInJSON(logBody)
	}

	c.logger.Info("received API response",
		"method", method,
		"path", path,
		"status_code", resp.StatusCode,
		"response_body", logBody,
	)

	return respBody, resp.StatusCode, nil
}

func (c *Client) doXML(method, path, token string, xmlPayload interface{}) ([]byte, int, error) {
	var body io.Reader
	if xmlPayload != nil {
		data, err := xml.MarshalIndent(xmlPayload, "", "  ")
		if err != nil {
			return nil, 0, fmt.Errorf("marshal xml payload: %w", err)
		}
		body = bytes.NewReader(data)
	}

	ct := ""
	if body != nil {
		ct = "application/xml"
	}
	return c.doRequest(method, path, token, ct, body)
}

func (c *Client) parseXMLResponse(data []byte) (*APIResponse, error) {
	var resp APIResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal xml response: %w (body: %s)", err, truncate(string(data), 500))
	}

	c.logUnknownFields(&resp)

	if resp.Status == "" {
		return nil, fmt.Errorf("nic.ru API returned empty status (body: %s)", truncate(string(data), 500))
	}
	if resp.Status != "success" {
		return &resp, fmt.Errorf("nic.ru API error: %s", resp.FormatErrors())
	}
	return &resp, nil
}

func (c *Client) logUnknownFields(resp *APIResponse) {
	if names := extraFieldNames(resp.Extra); names != nil {
		c.logger.Warn("nic.ru API returned unknown XML elements in response", "fields", names)
	}
	if names := extraFieldNames(resp.Data.Extra); names != nil {
		c.logger.Warn("nic.ru API returned unknown XML elements in data", "fields", names)
	}
	for _, z := range resp.Data.Zones {
		if names := extraFieldNames(z.Extra); names != nil {
			c.logger.Warn("nic.ru API returned unknown XML elements in zone",
				"zone", z.Name, "fields", names,
			)
		}
		for _, rr := range z.Records {
			if names := extraFieldNames(rr.Extra); names != nil {
				c.logger.Warn("nic.ru API returned unknown XML elements in record",
					"zone", z.Name, "record", rr.Name, "fields", names,
				)
			}
		}
	}
}

func (c *Client) GetServiceForZone(token, zoneName string) (string, error) {
	start := time.Now()
	body, _, err := c.doXML(http.MethodGet, zonesPath, token, nil)
	if err != nil {
		return "", fmt.Errorf("list zones: %w", err)
	}

	resp, err := c.parseXMLResponse(body)
	if err != nil {
		return "", err
	}

	available := make([]string, 0, len(resp.Data.Zones))
	for _, z := range resp.Data.Zones {
		available = append(available, z.Name)
	}
	c.logger.Info("fetched zone list from nic.ru",
		"total", len(resp.Data.Zones),
		"zones", available,
		"looking_for", zoneName,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	for _, z := range resp.Data.Zones {
		if z.Name == zoneName {
			if z.Service == "" {
				return "", fmt.Errorf("zone %q found but has no service attribute", zoneName)
			}
			return z.Service, nil
		}
	}
	return "", fmt.Errorf("zone %q not found in nic.ru account (available: %v)", zoneName, available)
}

func (c *Client) CreateTXTRecord(token, service, zone, name, value string, ttl int) (string, error) {
	start := time.Now()

	reqPayload := &RecordRequest{
		RrList: RrList{
			Rr: []Rr{{
				Name: name,
				TTL:  ttl,
				Type: "TXT",
				Txt:  &TxtRecord{Strings: []string{value}},
			}},
		},
	}

	c.logger.Info("creating TXT record via nic.ru API",
		"service", service,
		"zone", zone,
		"record", name,
		"ttl", ttl,
	)

	path := fmt.Sprintf(recordsPath, service, zone)
	body, _, err := c.doXML(http.MethodPut, path, token, reqPayload)
	if err != nil {
		return "", fmt.Errorf("create record: %w", err)
	}

	resp, err := c.parseXMLResponse(body)
	if err != nil {
		return "", err
	}

	var rrID string
	if len(resp.Data.Zones) > 0 && len(resp.Data.Zones[0].Records) > 0 {
		rrID = resp.Data.Zones[0].Records[0].ID
	}

	c.logger.Info("TXT record created, committing zone",
		"rr_id", rrID,
		"zone", zone,
		"record", name,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	if err := c.CommitZone(token, service, zone); err != nil {
		return rrID, fmt.Errorf("commit after create: %w", err)
	}

	c.logger.Info("zone committed, TXT record is live",
		"rr_id", rrID,
		"zone", zone,
		"total_duration_ms", time.Since(start).Milliseconds(),
	)

	return rrID, nil
}

func (c *Client) FindTXTRecord(token, service, zone, name, content string) (string, error) {
	start := time.Now()

	c.logger.Info("searching for TXT record in zone",
		"service", service,
		"zone", zone,
		"record", name,
		"match_content", content != "",
	)

	path := fmt.Sprintf(recordsPath, service, zone)
	body, _, err := c.doXML(http.MethodGet, path, token, nil)
	if err != nil {
		return "", fmt.Errorf("list records: %w", err)
	}

	resp, err := c.parseXMLResponse(body)
	if err != nil {
		return "", err
	}

	var totalRecords, txtRecords, nameMatches int
	for _, z := range resp.Data.Zones {
		totalRecords += len(z.Records)
		for _, rr := range z.Records {
			if rr.Type == "TXT" {
				txtRecords++
			}
			if rr.Name == name && rr.Type == "TXT" {
				nameMatches++
				if content == "" {
					c.logger.Info("found TXT record by name",
						"rr_id", rr.ID,
						"record", name,
						"total_records", totalRecords,
						"txt_records", txtRecords,
						"duration_ms", time.Since(start).Milliseconds(),
					)
					return rr.ID, nil
				}
				if rr.Txt != nil {
					for _, s := range rr.Txt.Strings {
						if s == content {
							c.logger.Info("found TXT record by name and content",
								"rr_id", rr.ID,
								"record", name,
								"total_records", totalRecords,
								"txt_records", txtRecords,
								"duration_ms", time.Since(start).Milliseconds(),
							)
							return rr.ID, nil
						}
					}
				}
			}
		}
	}

	c.logger.Warn("TXT record not found in zone",
		"record", name,
		"zone", zone,
		"total_records", totalRecords,
		"txt_records", txtRecords,
		"name_matches", nameMatches,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return "", fmt.Errorf("TXT record %q (content=%q) not found in zone %q (scanned %d records, %d TXT, %d name matches)",
		name, content, zone, totalRecords, txtRecords, nameMatches)
}

func (c *Client) DeleteRecord(token, service, zone, rrID string) error {
	start := time.Now()

	c.logger.Info("deleting record from nic.ru",
		"rr_id", rrID,
		"zone", zone,
	)

	path := fmt.Sprintf(recordPath, service, zone, rrID)
	body, _, err := c.doXML(http.MethodDelete, path, token, nil)
	if err != nil {
		return fmt.Errorf("delete record: %w", err)
	}

	if _, err := c.parseXMLResponse(body); err != nil {
		return err
	}

	c.logger.Info("record deleted, committing zone",
		"rr_id", rrID,
		"zone", zone,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	if err := c.CommitZone(token, service, zone); err != nil {
		return fmt.Errorf("commit after delete: %w", err)
	}

	c.logger.Info("zone committed, record removal is live",
		"rr_id", rrID,
		"zone", zone,
		"total_duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (c *Client) CommitZone(token, service, zone string) error {
	path := fmt.Sprintf(commitPath, service, zone)
	body, _, err := c.doXML(http.MethodPost, path, token, nil)
	if err != nil {
		return fmt.Errorf("commit zone: %w", err)
	}

	if _, err := c.parseXMLResponse(body); err != nil {
		return err
	}
	return nil
}

func (c *Client) RefreshToken(clientID, clientSecret, refreshToken string) (*TokenResponse, error) {
	form := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s",
		refreshToken, clientID, clientSecret)

	body, statusCode, err := c.doRequest(
		http.MethodPost, oauthPath, "",
		"application/x-www-form-urlencoded",
		strings.NewReader(form),
	)
	if err != nil {
		return nil, fmt.Errorf("refresh token request: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token failed: HTTP %d, body: %s", statusCode, redactTokensInJSON(string(body)))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}

	c.logger.Info("received new tokens from nic.ru OAuth",
		"token_type", token.TokenType,
		"expires_in", token.ExpiresIn,
	)
	return &token, nil
}

func (c *Client) ValidateToken(token string) error {
	c.logger.Info("validating access token against nic.ru API")

	body, statusCode, err := c.doXML(http.MethodGet, zonesPath, token, nil)
	if err != nil {
		return fmt.Errorf("validate token: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("validate token: HTTP %d", statusCode)
	}

	resp, err := c.parseXMLResponse(body)
	if err != nil {
		return fmt.Errorf("validate token: %w", err)
	}

	zones := make([]string, 0, len(resp.Data.Zones))
	for _, z := range resp.Data.Zones {
		zones = append(zones, z.Name)
	}
	c.logger.Info("token is valid, account has access to zones",
		"zones_count", len(resp.Data.Zones),
		"zones", zones,
	)
	return nil
}

func redactTokensInJSON(raw string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	for _, key := range []string{"access_token", "refresh_token", "client_secret", "password"} {
		if _, ok := m[key]; ok {
			m[key] = "[REDACTED]"
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return string(b)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
