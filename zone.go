package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const dnsName = "nicru-dns"

type DNSProviderSolver struct {
	client *kubernetes.Clientset
}

type nicruDNSProviderConfig struct {
	NicruApiTokenSecretRef cmmeta.SecretKeySelector `json:"nicruTokenSecretRef"`
}

func (c *DNSProviderSolver) getServiceName(zoneName string) (string, error) {
	var zone Zone
	var service string

	accessToken := c.getAccessToken()
	url := fmt.Sprintf(urlGetZoneInfo, accessToken)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}

	err = xml.Unmarshal(body, &zone)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal body: %w", err)
	}
	for i := 0; i < len(zone.Data.Zone); i++ {
		if zone.Data.Zone[i].Name == zoneName {
			service = zone.Data.Zone[i].Service
		}
	}

	klog.Infof("Service name: %s", service)
	return service, nil
}

func (c *DNSProviderSolver) Present(cr *v1alpha1.ChallengeRequest) error {
	zoneName := (cr.ResolvedZone[0 : len(cr.ResolvedZone)-1])
	ServiceName, err := c.getServiceName(zoneName)
	if err != nil {
		return fmt.Errorf("failed to get service name for zone %s: %w", zoneName, err)
	}

	klog.Infof("Call function Present: namespace=%s, zone=%s, fqdn=%s",
		cr.ResourceNamespace, cr.ResolvedZone, cr.ResolvedFQDN)

	cfg, err := loadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Present for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	err = c.Txt(cr.ResolvedFQDN, ServiceName, zoneName, cr.Key)
	if err != nil {
		return fmt.Errorf("failed to create record for zone %s: %w", zoneName, err)
	}

	return nil
}

func (c *DNSProviderSolver) CleanUp(cr *v1alpha1.ChallengeRequest) error {
	zoneName := (cr.ResolvedZone[0 : len(cr.ResolvedZone)-1])
	var ServiceName, _ = c.getServiceName(zoneName)
	domainName := fmt.Sprintf(".%s", cr.ResolvedZone)
	recordName := strings.TrimSuffix(cr.ResolvedFQDN, domainName)

	rrID, err := c.getRecord(ServiceName, zoneName, recordName)
	if err != nil {
		return fmt.Errorf("failed to get rrID: %w", err)
	}

	cfg, err := loadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Delete for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	err = c.deleteRecord(ServiceName, zoneName, rrID)
	if err != nil {
		return fmt.Errorf("failed to delete the record: %w", err)
	}

	return nil
}

func (c *DNSProviderSolver) Name() string {
	return dnsName
}

func (c *DNSProviderSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("k8s clientset: %w", err)
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
		return cfg, fmt.Errorf("failed to  decode config: %w", err)
	}
	return cfg, nil
}
