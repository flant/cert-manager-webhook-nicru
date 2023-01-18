package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
)

func (c *NicruClient) getZoneInfo(zoneName string) (name string, service string) {
	var zone Zone
	url := fmt.Sprintf("%szones/?token=%s", urlApi, c.token)
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
			id := zone.Data.Zone[i].ID
			name = zone.Data.Zone[i].Name
			service = zone.Data.Zone[i].Service
			klog.Infof("ZoneId=%s", id)
			klog.Infof("ZoneName=%s", name)
			klog.Infof("Service=%s", service)
		}
	}
	return
}
