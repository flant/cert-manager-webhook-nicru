package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
)

func (c *NicruClient) createRecord(request *Request, serviceName, zoneName string) string {

	var zone Zone

	url := fmt.Sprintf("%sservices/%s/zones/%s/records", urlApi, serviceName, zoneName)

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

	err = xml.Unmarshal(body, &zone)
	if err != nil {
		klog.Errorf("Failed marshal xml: %s", err)
	}

	rrId := zone.Data.Zone[0].RR[0].ID
	if zone.Status == "success" {
		klog.Infof("Record successfully added. rrId=%s", rrId)
	} else {
		klog.Error("Record has not created")
	}

	return rrId

}
