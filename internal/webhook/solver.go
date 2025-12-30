package webhook

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/flant/cert-manager-webhook-nicru/internal/dns"
	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const dnsName = "nicru-dns"

// Solver implements the ACME webhook interface for NIC.RU DNS
type Solver struct {
	client       *kubernetes.Clientset
	tokenManager *tokenmanager.TokenManager
	dnsClient    *dns.Client
}

// NewSolver creates a new webhook solver
func NewSolver(client *kubernetes.Clientset, tm *tokenmanager.TokenManager, dnsClient *dns.Client) *Solver {
	return &Solver{
		client:       client,
		tokenManager: tm,
		dnsClient:    dnsClient,
	}
}

// Present creates a TXT record for ACME challenge
func (s *Solver) Present(cr *v1alpha1.ChallengeRequest) error {
	zoneName := cr.ResolvedZone[0 : len(cr.ResolvedZone)-1]
	serviceName, err := s.dnsClient.GetServiceName(zoneName)
	if err != nil {
		return fmt.Errorf("failed to get service name for zone %s: %w", zoneName, err)
	}

	klog.Infof("Call function Present: namespace=%s, zone=%s, fqdn=%s",
		cr.ResourceNamespace, cr.ResolvedZone, cr.ResolvedFQDN)

	cfg, err := LoadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Present for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	err = s.dnsClient.CreateTXTRecord(cr.ResolvedFQDN, serviceName, zoneName, cr.Key)
	if err != nil {
		return fmt.Errorf("failed to create record for zone %s: %w", zoneName, err)
	}

	return nil
}

// CleanUp deletes the TXT record after ACME challenge
func (s *Solver) CleanUp(cr *v1alpha1.ChallengeRequest) error {
	zoneName := cr.ResolvedZone[0 : len(cr.ResolvedZone)-1]
	serviceName, err := s.dnsClient.GetServiceName(zoneName)
	if err != nil {
		return fmt.Errorf("failed to get service name for zone %s: %w", zoneName, err)
	}
	domainName := fmt.Sprintf(".%s", cr.ResolvedZone)
	recordName := strings.TrimSuffix(cr.ResolvedFQDN, domainName)

	cfg, err := LoadConfig(cr.Config)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	klog.Infof("Decoded configuration %v", cfg)
	klog.Infof("Delete for entry=%s, domain=%s, key=%s", cr.ResolvedFQDN, cr.ResolvedZone, cr.Key)

	rrID, err := s.dnsClient.GetRecord(serviceName, zoneName, recordName)
	if err != nil {
		// If record not found, it's already deleted - this is OK (idempotent)
		if errors.Is(err, dns.ErrRecordNotFound) {
			klog.Infof("Record %s already deleted, cleanup succeeded (idempotent)", recordName)
			return nil
		}
		return fmt.Errorf("failed to get rrID: %w", err)
	}

	err = s.dnsClient.DeleteRecord(serviceName, zoneName, rrID)
	if err != nil {
		return fmt.Errorf("failed to delete the record: %w", err)
	}

	klog.Infof("Successfully cleaned up challenge record %s", recordName)
	return nil
}

// Name returns the webhook solver name
func (s *Solver) Name() string {
	return dnsName
}

// Initialize initializes the webhook solver with Kubernetes client
func (s *Solver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("k8s clientset: %w", err)
	}

	s.client = cl
	return nil
}
