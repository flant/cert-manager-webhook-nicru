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
}

type nicruDNSProviderConfig struct {
	NicruApiTokenSecretRef cmmeta.SecretKeySelector `json:"nicruTokenSecretRef"`
}

func (c *nicruDNSProviderSolver) Present(challengeRequest *v1alpha1.ChallengeRequest) (string, error) {
	klog.Infof("call function Present: namespace=%s, zone=%s, fqdn=%s", challengeRequest.ResourceNamespace, challengeRequest.ResolvedZone, challengeRequest.ResolvedFQDN)
	cfg, err := loadConfig(challengeRequest.Config)
	if err != nil {
		klog.Errorf("Unable to load config: %v", err)
	}
	klog.Infof("Decoded configuration %v", cfg)

	klog.Infof("Present for entry=%s, domain=%s, key=%s", challengeRequest.ResolvedFQDN, challengeRequest.ResolvedZone, challengeRequest.Key)

	var ZoneName, ServiceName = nicruClient.getZoneInfo(challengeRequest.ResolvedZone)
	rrId := nicruClient.Txt(challengeRequest.ResolvedFQDN, ServiceName, ZoneName)

	return rrId, err
}

func (c *nicruDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {

	return
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
