package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
)

func (c *nicruDNSProviderSolver) getZoneInfo(zoneName string) (name string, service string) {
	var zone Zone
	_, accessToken := c.getSecretData()
	url := fmt.Sprintf(urlGetZoneInfo, accessToken)

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
			name = zone.Data.Zone[i].Name
			service = zone.Data.Zone[i].Service
		}
	}
	return
}
