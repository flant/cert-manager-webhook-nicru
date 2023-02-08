package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
)

func (c *nicruDNSProviderSolver) deleteRecord(serviceName, zoneName, rrId string) {
	var response Response
	_, accessToken := c.getSecretData()
	url := fmt.Sprintf(urlDeleteRecord, serviceName, zoneName, rrId)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
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

	err = xml.Unmarshal(body, &response)
	if err != nil {
		klog.Errorf("Failed reading body: %s", response.Errors.Error.Text)
	}
	if response.Status == "success" {
		klog.Infof("Record successfully deleted.")
		c.Commit(serviceName, zoneName)
	} else {
		klog.Errorf("Record has not been deleted")
	}
}
