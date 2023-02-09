package main

import (
	"encoding/json"
	"fmt"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"os"
	"strings"
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
	GroupName  = os.Getenv("GROUP_NAME")
	APP_ID     = os.Getenv("APP_ID")
	APP_SECRET = os.Getenv("APP_SECRET")
	NAMESPACE  = os.Getenv("NAMESPACE")
)

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	c := nicruDNSProviderSolver{}
	go c.cronUpdateToken()

	cmd.RunWebhookServer(GroupName, &c)
}

type nicruDNSProviderSolver struct {
	client *kubernetes.Clientset
}

type nicruDNSProviderConfig struct {
	NicruApiTokenSecretRef cmmeta.SecretKeySelector `json:"nicruTokenSecretRef"`
}

func (c *nicruDNSProviderSolver) Present(cr *v1alpha1.ChallengeRequest) error {
	targetZone := (cr.ResolvedZone[0 : len(cr.ResolvedZone)-1])
	var ZoneName, ServiceName = c.getZoneInfo(targetZone)

	klog.Infof("Call function Present: namespace=%s, zone=%s, fqdn=%s", cr.ResourceNamespace, cr.ResolvedZone, cr.ResolvedFQDN)

	cfg, err := loadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("Unable to load config: %v", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Present for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	c.Txt(cr.ResolvedFQDN, ServiceName, ZoneName, cr.Key)

	return nil
}

func (c *nicruDNSProviderSolver) CleanUp(cr *v1alpha1.ChallengeRequest) error {
	targetZone := (cr.ResolvedZone[0 : len(cr.ResolvedZone)-1])
	var ZoneName, ServiceName = c.getZoneInfo(targetZone)
	domainName := fmt.Sprintf(".%s", cr.ResolvedZone)
	recordName := strings.TrimSuffix(cr.ResolvedFQDN, domainName)
	rrId := c.getRecord(ServiceName, ZoneName, recordName)

	klog.Infof("Call function CleanUp: namespace=%s, zone=%s, fqdn=%s", cr.ResourceNamespace, cr.ResolvedZone, cr.ResolvedFQDN)

	cfg, err := loadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("Error: %s", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Delete for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	c.deleteRecord(ServiceName, ZoneName, rrId)

	return nil
}

func (c *nicruDNSProviderSolver) Name() string {
	return "nicru-dns"
}

func (c *nicruDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	klog.Infof("call function Initialize")
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("Unable to get k8s client: %v", err)
	}
	c.client = cl
	return nil
}

func loadConfig(cfgJSON *extapi.JSON) (nicruDNSProviderConfig, error) {
	cfg := nicruDNSProviderConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}

	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		klog.Errorf("error decoding solver config: %v", err)
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}
	return cfg, nil
}
