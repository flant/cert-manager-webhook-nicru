package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
)

func (c *NicruClient) getRecord(serviceName, zoneName, recordName string) string {
	var zone Zone
	urlRecord := fmt.Sprintf("%sservices/%s/zones/&s/records", urlApi, serviceName, zoneName)
	req, err := http.NewRequest("GET", urlRecord, nil)
	if err != nil {
		klog.Errorf("Message: %s", err)
	}

	header := fmt.Sprintf("Bearer %s", c.token)

	req.Header.Add("Authorization", header)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		klog.Errorf("Message: %s", err)
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	err = xml.Unmarshal(body, &zone)
	if err != nil {
		klog.Errorf("Failed to get record. Reading body failed. Message: %s", err)

	} else {
		for i := 0; i < len(zone.Data.Zone); i++ {
			for j := 0; j < len(zone.Data.Zone[i].RR); j++ {
				if zone.Data.Zone[i].RR[j].Name == recordName {
					rrId := zone.Data.Zone[i].RR[j].ID
					klog.Infof("rrId = %v", rrId)
					return rrId
				}
			}
		}
	}
	return ""
}
