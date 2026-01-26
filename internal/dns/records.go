package dns

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"github.com/flant/cert-manager-webhook-nicru/internal/config"
	"k8s.io/klog/v2"
)

// CreateTXTRecord creates a TXT record for ACME challenge
func (c *Client) CreateTXTRecord(recordName, serviceName, zoneName, content string) error {
	var record = &Request{
		RrList: &RrList{
			Rr: []*Rr{},
		},
	}

	record.RrList.Rr = append(record.RrList.Rr, &Rr{
		Name: recordName,
		TTL:  60,
		Type: "TXT",
		Txt: &TxtRecord{
			String: content,
		},
	})

	err := c.createRecord(record, serviceName, zoneName)
	if err != nil {
		return fmt.Errorf("failed to create new record: %w", err)
	}

	return nil
}

func (c *Client) createRecord(request *Request, serviceName, zoneName string) error {
	var zone Zone
	accessToken, err := c.tokenManager.GetCurrentAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}

	url := fmt.Sprintf(config.URLCreateRecord, serviceName, zoneName)

	payload, err := xml.MarshalIndent(request, "", "")
	if err != nil {
		return fmt.Errorf("failed to marshal indent XML: %w", err)
	}
	bodyPayload := bytes.NewBuffer(payload)

	req, err := http.NewRequest(http.MethodPut, url, bodyPayload)
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

	err = xml.Unmarshal(body, &zone)
	if err != nil {
		return fmt.Errorf("failed to unmarshal body: %w", err)
	}

	if zone.Status != "success" {
		return fmt.Errorf("failed to create record: status=%s", zone.Status)
	}

	if len(zone.Data.Zone) == 0 || len(zone.Data.Zone[0].RR) == 0 {
		return fmt.Errorf("no resource records returned from API")
	}

	rrID := zone.Data.Zone[0].RR[0].ID

	err = c.CommitZone(serviceName, zoneName)
	if err != nil {
		return fmt.Errorf("failed to commit the zone: %w", err)
	}
	klog.Infof("Record successfully added. rrID=%s", rrID)
	return nil
}

// GetRecord retrieves a DNS record ID by name
func (c *Client) GetRecord(serviceName, zoneName, recordName string) (string, error) {
	var zone Zone
	accessToken, err := c.tokenManager.GetCurrentAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	urlRecord := fmt.Sprintf(config.URLGetRecord, serviceName, zoneName)

	req, err := http.NewRequest(http.MethodGet, urlRecord, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate request: %w", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)
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
		return "", fmt.Errorf("failed to unmarshal: %w", err)
	}

	var rrID string
	for i := 0; i < len(zone.Data.Zone); i++ {
		for j := 0; j < len(zone.Data.Zone[i].RR); j++ {
			if zone.Data.Zone[i].RR[j].Name == recordName {
				rrID = zone.Data.Zone[i].RR[j].ID
			}
		}
	}

	if rrID == "" {
		return "", ErrRecordNotFound
	}
	return rrID, nil
}

// DeleteRecord deletes a DNS record by ID
func (c *Client) DeleteRecord(serviceName, zoneName, rrID string) error {
	var response Response
	accessToken, err := c.tokenManager.GetCurrentAccessToken()
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	url := fmt.Sprintf(config.URLDeleteRecord, serviceName, zoneName, rrID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to generate request: %w", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)

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
		// Check if error is "record not found" (error code 4035)
		if response.Errors.Error.Code == "4035" {
			klog.Infof("Record already deleted (rrID=%s), skipping", rrID)
			return nil // Idempotent: already deleted is success
		}
		return fmt.Errorf("failed to delete record: status=%s, error=%s", response.Status, response.Errors.Error.Text)
	}

	err = c.CommitZone(serviceName, zoneName)
	if err != nil {
		return fmt.Errorf("failed to commit the zone: %w", err)
	}

	klog.Infof("Record successfully deleted")
	return nil
}
