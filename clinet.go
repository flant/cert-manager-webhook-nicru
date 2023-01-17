package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"log"
	"net/http"
	"os"
	"strings"
)

const (
	urlOauth  = "https://api.nic.ru/oauth/token"
	urlRecord = ""
)

var (
	nicru       = NicruClient{os.Getenv("TOKEN")}
	nicruClient = NewNicruClient(nicru.token)
)

func NewNicruClient(token string) *NicruClient {
	return &NicruClient{
		token: token,
	}
}

func get_tokens() {
	var tokens NicruTokens

	Rtoken := ""
	ID := ""
	Secret := ""
	s := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s", Rtoken, ID, Secret)
	payload := strings.NewReader(s)
	req, _ := http.NewRequest("POST", urlOauth, payload)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, _ := http.DefaultClient.Do(req)

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	err := json.Unmarshal(body, &tokens)
	if err != nil {
		log.Printf("Reading body failed: %s", err)
		return
	}

	log.Printf("Access token = %s", tokens.AccessToken)
	log.Printf("Refresh token = %s", tokens.RefreshToken)
}

func (c *NicruClient) get_records() {

	var error Response

	req, err := http.NewRequest("GET", urlRecord, nil)
	if err != nil {
		klog.Errorf("Code: %s, Message: %s", error.Errors.Error.Code, error.Errors.Error.Text)
	}

	header := fmt.Sprintf("Bearer %s", c.token)

	req.Header.Add("Authorization", header)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("Message: %s", err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	err = xml.Unmarshal(body, &error)
	if err != nil {
		if error.Errors.Error.Code == "" && error.Errors.Error.Text == "" {
			klog.Errorf("Failed to get records. Reading body failed. Message: %s", err)
		} else {
			klog.Errorf("Failed to get records. Code: %s, Message: %s", error.Errors.Error.Code, error.Errors.Error.Text)
		}
	} else {
		bodyStr := string(body)
		klog.Infof("Bode %s", bodyStr)

	}

}

func (c *NicruClient) getZoneName(zoneName string) (id string, name string, service string) {
	var zone Zone
	url := fmt.Sprintf("https://api.nic.ru/dns-master/zones/?token=%s", c.token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	err = xml.Unmarshal(body, &zone)
	if err != nil {
		klog.Errorf("Reading body failed: %s", err)
	}
	for i := 0; i < len(zone.Data.Zone); i++ {
		if zone.Data.Zone[i].Name == zoneName {
			id = zone.Data.Zone[i].ID
			name = zone.Data.Zone[i].Name
			service = zone.Data.Zone[i].Service
			klog.Infof("ZoneId=%s", id)
			klog.Infof("ZoneName=%s", name)
			klog.Infof("Service=%s", service)
		}
	}
	return
}

func (c *NicruClient) createTxt(request *Request) {

	var error Response
	serviceName := ""
	zoneName := ""
	url := fmt.Sprintf("https://api.nic.ru/dns-master/services/%s/zones/%s/records", serviceName, zoneName)

	payload, err := xml.MarshalIndent(request, "", "")
	if err != nil {
		klog.Errorf("Failed marshal XML: %s", err)
	}
	bodyPayload := bytes.NewBuffer([]byte(payload))
	klog.Infof("Payload: %s", bodyPayload)

	req, err := http.NewRequest("PUT", url, bodyPayload)

	if err != nil {
		klog.Errorf("Error: %s", err)
	}
	header := fmt.Sprintf("Bearer %s", c.token)

	req.Header.Add("Authorization", header)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	err = xml.Unmarshal(body, &error)
	if err != nil {
		klog.Errorf("Failed marshal xml: %s", err)
	}

	err = xml.Unmarshal(body, &error)
	if err != nil {
		klog.Errorf("Code: %s, Message: %s", error.Errors.Error.Code, error.Errors.Error.Text)
	}

	klog.Infof("Body: %s", body)
	klog.Infof("rrId=%s", request.RrList.Rr[0].ID)
}

func (c *NicruClient) deleteTxt() {

	url := fmt.Sprintf("")

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	header := fmt.Sprintf("Bearer %s", c.token)
	req.Header.Add("Authorization", header)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		klog.Errorf("Failed reading body: %s", err)
	}

	klog.Infof("Body: %s", body)
}

func main() {
	nicruClient.get_records()
	var zoneName = ""
	var recName = ""
	nicruClient.getZoneName(zoneName)
	nicruClient.Txt(recName)
	nicruClient.get_records()

}
