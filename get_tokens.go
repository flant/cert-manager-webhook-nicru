package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"net/http"
	"strings"
)

func (c *nicruDNSProviderSolver) getNewTokens() {
	var token NicruTokens

	currentRefreshToken, _ := c.getSecretData()

	params := fmt.Sprintf("grant_type=refresh_token&refresh_token=%s&client_id=%s&client_secret=%s", currentRefreshToken, APP_ID, APP_SECRET)
	payload := strings.NewReader(params)

	req, _ := http.NewRequest("POST", oauthUrl, payload)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, _ := http.DefaultClient.Do(req)

	body, _ := ioutil.ReadAll(res.Body)

	defer res.Body.Close()

	err := json.Unmarshal(body, &token)
	if err != nil {
		klog.Errorf("Reading body failed: %s", err)
	}

	c.patchSecret(token.RefreshToken, token.AccessToken)

}
