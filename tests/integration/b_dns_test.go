//go:build integration

package integration

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Zone structures for XML parsing (from main package)
type Zone struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status"`
	Data    struct {
		Zone []struct {
			Name    string `xml:"name,attr"`
			Service string `xml:"service,attr"`
			ID      string `xml:"id,attr"`
			RR      []struct {
				ID   string `xml:"id,attr"`
				Name string `xml:"name"`
				Type string `xml:"type"`
			} `xml:"rr"`
		} `xml:"zone"`
	} `xml:"data"`
}

type Response struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status"`
	Data    struct {
		Zone []struct {
			RR []struct {
				ID string `xml:"id,attr"`
			} `xml:"rr"`
		} `xml:"zone"`
	} `xml:"data"`
	Errors struct {
		Error struct {
			Text string `xml:",chardata"`
			Code string `xml:"code,attr"`
		} `xml:"error"`
	} `xml:"errors"`
}

// TestIntegration_08_GetUserZones tests listing user's DNS zones
func TestIntegration_08_GetUserZones(t *testing.T) {
	logTestStart(t, "Get list of user's DNS zones")
	skipIfNoTokens(t)

	url := fmt.Sprintf(zonesURL, sharedTokens.AccessToken)
	t.Logf("GET %s", url)

	resp, err := testHTTPClient.Get(url)
	if err != nil {
		t.Fatalf("Failed to get zones: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var zone Zone
	if err := xml.Unmarshal(body, &zone); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	t.Logf("Found %d zones", len(zone.Data.Zone))

	if len(zone.Data.Zone) == 0 {
		t.Fatal("No DNS zones found in your NIC.RU account. Please add a zone before running DNS tests.")
	}

	// Get test zone from credentials (REQUIRED)
	creds := skipIfNoCredentials(t)
	if creds.TestZone == "" {
		t.Fatal("NICRU_TEST_ZONE environment variable is required for DNS tests. Please set it in test-credentials.env")
	}

	// Find the specified test zone in user's zones
	var foundZone *ZoneInfo

	for _, z := range zone.Data.Zone {
		if z.Name == creds.TestZone {
			if z.Service == "" {
				t.Fatalf("Test zone '%s' found but has no service assigned. Please check zone configuration in NIC.RU.", creds.TestZone)
			}
			foundZone = &ZoneInfo{
				Name:    z.Name,
				Service: z.Service,
				ID:      z.ID,
			}
			break
		}
	}

	if foundZone == nil {
		// List available zones to help user
		var availableZones []string
		for _, z := range zone.Data.Zone {
			if z.Name != "" {
				availableZones = append(availableZones, z.Name)
			}
		}

		if len(availableZones) > 0 {
			t.Fatalf("Test zone '%s' not found in your NIC.RU account.\n\nAvailable zones:\n  - %s\n\nPlease update NICRU_TEST_ZONE in test-credentials.env",
				creds.TestZone, strings.Join(availableZones, "\n  - "))
		} else {
			t.Fatal("No DNS zones found in your NIC.RU account. Please add a zone before running DNS tests.")
		}
	}

	// Save zone info for subsequent tests
	sharedZones = append(sharedZones, *foundZone)
	testServiceName = foundZone.Service
	testZoneName = foundZone.Name

	t.Logf("Using test zone: %s (service=%s, id=%s)", testZoneName, testServiceName, foundZone.ID)
	logTestSuccess(t, fmt.Sprintf("Found %d total zones, using specified test zone: %s", len(zone.Data.Zone), testZoneName))
}

// TestIntegration_09_GetZoneRecords tests getting records from a zone
func TestIntegration_09_GetZoneRecords(t *testing.T) {
	logTestStart(t, "Get DNS records from zone")
	skipIfNoTokens(t)
	skipIfNoZones(t)

	url := fmt.Sprintf("https://api.nic.ru/dns-master/services/%s/zones/%s/records",
		testServiceName, testZoneName)
	t.Logf("GET %s", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", sharedTokens.AccessToken))

	resp, err := testHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to get records: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	var zone Zone
	if err := xml.Unmarshal(body, &zone); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	recordCount := 0
	if len(zone.Data.Zone) > 0 {
		recordCount = len(zone.Data.Zone[0].RR)
	}

	t.Logf("Found %d records in zone %s", recordCount, testZoneName)
	logTestSuccess(t, fmt.Sprintf("Successfully retrieved %d records", recordCount))
}

// TestIntegration_10_CreateTXTRecord tests creating a TXT record
func TestIntegration_10_CreateTXTRecord(t *testing.T) {
	logTestStart(t, "Create TXT record for ACME challenge")
	skipIfNoTokens(t)
	skipIfNoZones(t)

	recordName := createTestRecordName()
	recordValue := "integration-test-value-" + fmt.Sprintf("%d", 12345)

	t.Logf("Creating record: %s.%s = %s", recordName, testZoneName, recordValue)

	// Prepare XML payload
	xmlPayload := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<request>
	<rr-list>
		<rr>
			<name>%s</name>
			<ttl>60</ttl>
			<type>TXT</type>
			<txt>
				<string>%s</string>
			</txt>
		</rr>
	</rr-list>
</request>`, recordName, recordValue)

	url := fmt.Sprintf("https://api.nic.ru/dns-master/services/%s/zones/%s/records",
		testServiceName, testZoneName)
	t.Logf("PUT %s", url)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBufferString(xmlPayload))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", sharedTokens.AccessToken))
	req.Header.Add("Content-Type", "application/xml")

	resp, err := testHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create record: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := xml.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	if response.Status != "success" {
		t.Fatalf("Expected status=success, got %s: %s", response.Status, response.Errors.Error.Text)
	}

	// Extract record ID
	if len(response.Data.Zone) > 0 && len(response.Data.Zone[0].RR) > 0 {
		testRecordID = response.Data.Zone[0].RR[0].ID
		t.Logf("Created record with ID: %s", testRecordID)
	} else {
		t.Fatal("No record ID in response")
	}

	logTestSuccess(t, fmt.Sprintf("Created TXT record: %s (ID: %s)", recordName, testRecordID))
}

// TestIntegration_11_CommitZone tests committing zone changes
func TestIntegration_11_CommitZone(t *testing.T) {
	logTestStart(t, "Commit zone changes")
	skipIfNoTokens(t)
	skipIfNoZones(t)

	if testRecordID == "" {
		t.Skip("No test record created, skipping commit test")
	}

	url := fmt.Sprintf("https://api.nic.ru/dns-master/services/%s/zones/%s/commit",
		testServiceName, testZoneName)
	t.Logf("POST %s", url)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", sharedTokens.AccessToken))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := testHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to commit zone: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := xml.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	if response.Status != "success" {
		t.Fatalf("Expected status=success, got %s: %s", response.Status, response.Errors.Error.Text)
	}

	logTestSuccess(t, "Zone changes committed successfully")
}

// TestIntegration_12_DeleteTXTRecord tests deleting the test record (cleanup)
func TestIntegration_12_DeleteTXTRecord(t *testing.T) {
	logTestStart(t, "Delete test TXT record (cleanup)")
	skipIfNoTokens(t)
	skipIfNoZones(t)

	if testRecordID == "" {
		t.Skip("No test record to delete, skipping")
	}

	// Setup cleanup in case this test fails - ensures record is deleted
	t.Cleanup(func() {
		if testRecordID != "" {
			t.Logf("Cleanup: ensuring test record %s is deleted", testRecordID)
			deleteTestRecord(t)
			testRecordID = ""
		}
	})

	deleteTestRecord(t)

	// Commit the deletion
	url := fmt.Sprintf("https://api.nic.ru/dns-master/services/%s/zones/%s/commit",
		testServiceName, testZoneName)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		t.Fatalf("Failed to create commit request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", sharedTokens.AccessToken))

	resp, err := testHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to commit deletion: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := xml.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	if response.Status != "success" {
		t.Fatalf("Expected status=success, got %s: %s", response.Status, response.Errors.Error.Text)
	}

	testRecordID = "" // Clear the ID
	logTestSuccess(t, "Test record deleted and changes committed")
}

// deleteTestRecord helper function to delete test record
func deleteTestRecord(t *testing.T) {
	t.Helper()

	if testRecordID == "" || testServiceName == "" || testZoneName == "" {
		return
	}

	url := fmt.Sprintf("https://api.nic.ru/dns-master/services/%s/zones/%s/records/%s",
		testServiceName, testZoneName, testRecordID)
	t.Logf("DELETE %s", url)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("Failed to create delete request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", sharedTokens.AccessToken))

	resp, err := testHTTPClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete record: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := xml.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	if response.Status != "success" {
		t.Fatalf("Expected status=success, got %s: %s", response.Status, response.Errors.Error.Text)
	}

	t.Logf("Successfully deleted record %s", testRecordID)
}
