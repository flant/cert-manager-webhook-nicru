package main

import (
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"k8s.io/klog/v2"
)

const (
	nameSecret      = "nicru-tokens"
	apiUrl          = `https://api.nic.ru/`
	oauthUrl        = apiUrl + `oauth/token`
	urlCommit       = apiUrl + `dns-master/services/%s/zones/%s/commit`
	urlCreateRecord = apiUrl + `dns-master/services/%s/zones/%s/records`
	urlDeleteRecord = apiUrl + `dns-master/services/%s/zones/%s/records/%s`
	urlGetRecord    = apiUrl + `dns-master/services/%s/zones/%s/records`
	urlGetZoneInfo  = apiUrl + `dns-master/zones/?token=%s`
)

var (
	GroupName = os.Getenv("GROUP_NAME")
	Namespace = os.Getenv("NAMESPACE")
)

func main() {
	if GroupName == "" {
		klog.Fatal("group name must be specified")
	}

	c := DNSProviderSolver{}
	go c.cronUpdateToken()

	cmd.RunWebhookServer(GroupName, &c)
}
