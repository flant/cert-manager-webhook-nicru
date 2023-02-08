package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
)

func (c *nicruDNSProviderSolver) Commit(serviceName, zoneName string) {
	var response Response
	_, accessToken := c.getSecretData()

	url := fmt.Sprintf(urlCommit, serviceName, zoneName)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	header := fmt.Sprintf("Bearer %s", accessToken)
	req.Header.Add("Authorization", header)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("Error: %s", err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	err = xml.Unmarshal(body, &response)
	if err != nil {
		klog.Errorf("Reading body failed: %s", response.Errors.Error.Text)
	}

	if response.Status == "success" {
		klog.Infof("Commit zone success")
	} else {
		klog.Errorf("Commit zone failed: %s", response.Errors.Error.Text)
	}
}
