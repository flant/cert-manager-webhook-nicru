package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	"k8s.io/klog/v2"
)

func (c *DNSProviderSolver) Txt(recordName, serviceName, zoneName, content string) error {
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
			String: content}})

	err := c.createRecord(record, serviceName, zoneName)
	if err != nil {
		return fmt.Errorf("failed to create new record: %w", err)
	}

	return nil
}

func (c *DNSProviderSolver) createRecord(request *Request, serviceName, zoneName string) error {
	var zone Zone
	accessToken := c.getAccessToken()

	url := fmt.Sprintf(urlCreateRecord, serviceName, zoneName)

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

	res, err := http.DefaultClient.Do(req)
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
	rrID := zone.Data.Zone[0].RR[0].ID
	if zone.Status != "success" {
		return fmt.Errorf("failed to create record: status does not indicate success")
	}

	err = c.Commit(serviceName, zoneName)
	if err != nil {
		return fmt.Errorf("failed to commit the zone: %w", err)
	}
	klog.Infof("Record successfully added. rrID=%s", rrID)
	return nil
}

func (c *DNSProviderSolver) getRecord(serviceName, zoneName, recordName string) (string, error) {
	var zone Zone
	accessToken := c.getAccessToken()

	urlRecord := fmt.Sprintf(urlGetRecord, serviceName, zoneName)

	req, err := http.NewRequest(http.MethodGet, urlRecord, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate request: %w", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)
	res, err := http.DefaultClient.Do(req)
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
		return "", fmt.Errorf("record not found")
	}
	return rrID, nil
}

func (c *DNSProviderSolver) deleteRecord(serviceName, zoneName, rrID string) error {
	var response Response
	accessToken := c.getAccessToken()
	url := fmt.Sprintf(urlDeleteRecord, serviceName, zoneName, rrID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to generate request: %w", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)

	res, err := http.DefaultClient.Do(req)
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
		return fmt.Errorf("failed to unmarhal body: %w", err)
	}
	if response.Status != "success" {
		return fmt.Errorf("return fmt.Errorf")
	}

	err = c.Commit(serviceName, zoneName)
	if err != nil {
		return fmt.Errorf("failed to commit the zone: %w", err)
	}

	klog.Infof("Record successfully deleted")
	return nil
}

func (c *DNSProviderSolver) Commit(serviceName, zoneName string) error {
	var response Response
	accessToken := c.getAccessToken()

	url := fmt.Sprintf(urlCommit, serviceName, zoneName)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to generate request: %w", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
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
		return fmt.Errorf("failed to unmarhal body: %w", err)
	}

	if response.Status != "success" {
		return fmt.Errorf("commit zone failed: %w", err)
	}

	return nil
}
