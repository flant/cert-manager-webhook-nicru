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
)

const (
	urlApi = "https://api.nic.ru/dns-master/"
)

var (
	nicru       = NicruClient{os.Getenv("TOKEN")}
	nicruClient = NewNicruClient(nicru.token)
	GroupName   = os.Getenv("GROUP_NAME")
)

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	cmd.RunWebhookServer(GroupName,
		&nicruDNSProviderSolver{},
	)
}

type nicruDNSProviderSolver struct {
	client *kubernetes.Clientset
	RrId   string
}

type nicruDNSProviderConfig struct {
	NicruApiTokenSecretRef cmmeta.SecretKeySelector `json:"nicruTokenSecretRef"`
}

func (c *nicruDNSProviderSolver) Present(cr *v1alpha1.ChallengeRequest) error {
	var ZoneName, ServiceName = nicruClient.getZoneInfo(cr.ResolvedZone)

	klog.Infof("Call function Present: namespace=%s, zone=%s, fqdn=%s", cr.ResourceNamespace, cr.ResolvedZone, cr.ResolvedFQDN)

	cfg, err := loadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("Unable to load config: %v", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Present for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	nicruClient.Txt(cr.ResolvedFQDN, ServiceName, ZoneName)

	return nil
}

func (c *nicruDNSProviderSolver) CleanUp(cr *v1alpha1.ChallengeRequest) error {
	var ZoneName, ServiceName = nicruClient.getZoneInfo(cr.ResolvedZone)
	rrId := nicruClient.getRecord(ServiceName, ZoneName, cr.ResolvedFQDN)

	klog.Infof("Call function CleanUp: namespace=%s, zone=%s, fqdn=%s", cr.ResourceNamespace, cr.ResolvedZone, cr.ResolvedFQDN)

	cfg, err := loadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("Error: %s", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Delete for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	nicruClient.deleteRecord(ServiceName, ZoneName, rrId)

	return nil
}

func (c *nicruDNSProviderSolver) Name() string {
	return "nicru-dns"
}

func (c *nicruDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	klog.Infof("call function Initialize")
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("unable to get k8s client: %v", err)
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
