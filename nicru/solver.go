package nicru

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	solverName        = "nicru-dns"
	DefaultSecretName = "nicru-tokens"
	defaultTTL        = 60
)

type Solver struct {
	k8s          *kubernetes.Clientset
	api          *Client
	logger       *slog.Logger
	namespace    string
	secretName   string
	serviceCache sync.Map
}

type solverConfig struct {
	NicruTokenSecretRef struct {
		Name string `json:"name"`
		Key  string `json:"key"`
	} `json:"nicruTokenSecretRef"`
}

func NewSolver(namespace, secretName string, logger *slog.Logger) *Solver {
	return &Solver{
		namespace:  namespace,
		secretName: secretName,
		api:        NewClient(logger),
		logger:     logger,
	}
}

func (s *Solver) Name() string {
	return solverName
}

func (s *Solver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	s.logger.Info("initializing webhook solver",
		"namespace", s.namespace,
		"secret", s.secretName,
	)

	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("create k8s clientset: %w", err)
	}
	s.k8s = cl

	s.logger.Info("webhook solver initialized, starting token manager")
	go s.runTokenManager(stopCh)
	return nil
}

func (s *Solver) Present(ch *v1alpha1.ChallengeRequest) error {
	start := time.Now()
	zoneName := strings.TrimSuffix(ch.ResolvedZone, ".")
	recordName := extractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)

	s.logger.Info("received ACME challenge, need to create TXT record",
		"uid", string(ch.UID),
		"dns_name", ch.DNSName,
		"zone", zoneName,
		"fqdn", ch.ResolvedFQDN,
		"record", recordName,
		"resource_namespace", ch.ResourceNamespace,
	)

	token, err := s.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	service, err := s.resolveService(token, zoneName)
	if err != nil {
		return fmt.Errorf("resolve service for zone %q: %w", zoneName, err)
	}

	s.logger.Info("creating TXT record for ACME challenge",
		"uid", string(ch.UID),
		"service", service,
		"zone", zoneName,
		"record", recordName,
		"ttl", defaultTTL,
	)

	rrID, err := s.api.CreateTXTRecord(token, service, zoneName, recordName, ch.Key, defaultTTL)
	if err != nil {
		return fmt.Errorf("create TXT record in zone %q: %w", zoneName, err)
	}

	s.logger.Info("ACME challenge TXT record created successfully",
		"uid", string(ch.UID),
		"zone", zoneName,
		"record", recordName,
		"rr_id", rrID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (s *Solver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	start := time.Now()
	zoneName := strings.TrimSuffix(ch.ResolvedZone, ".")
	recordName := extractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)

	s.logger.Info("received cleanup request, need to delete TXT record",
		"uid", string(ch.UID),
		"dns_name", ch.DNSName,
		"zone", zoneName,
		"fqdn", ch.ResolvedFQDN,
		"record", recordName,
		"resource_namespace", ch.ResourceNamespace,
	)

	token, err := s.getAccessToken()
	if err != nil {
		return fmt.Errorf("get access token: %w", err)
	}

	service, err := s.resolveService(token, zoneName)
	if err != nil {
		return fmt.Errorf("resolve service for zone %q: %w", zoneName, err)
	}

	s.logger.Info("searching for TXT record to delete",
		"uid", string(ch.UID),
		"service", service,
		"zone", zoneName,
		"record", recordName,
	)

	rrID, err := s.api.FindTXTRecord(token, service, zoneName, recordName, ch.Key)
	if err != nil {
		return fmt.Errorf("find TXT record in zone %q: %w", zoneName, err)
	}

	s.logger.Info("found TXT record, deleting",
		"uid", string(ch.UID),
		"rr_id", rrID,
		"record", recordName,
	)

	if err := s.api.DeleteRecord(token, service, zoneName, rrID); err != nil {
		return fmt.Errorf("delete record %s in zone %q: %w", rrID, zoneName, err)
	}

	s.logger.Info("ACME challenge TXT record deleted successfully",
		"uid", string(ch.UID),
		"zone", zoneName,
		"record", recordName,
		"rr_id", rrID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

func (s *Solver) resolveService(token, zoneName string) (string, error) {
	if cached, ok := s.serviceCache.Load(zoneName); ok {
		s.logger.Info("using cached nic.ru service for zone",
			"zone", zoneName,
			"service", cached.(string),
		)
		return cached.(string), nil
	}

	s.logger.Info("looking up nic.ru service for zone (not in cache)",
		"zone", zoneName,
	)
	service, err := s.api.GetServiceForZone(token, zoneName)
	if err != nil {
		return "", err
	}

	s.serviceCache.Store(zoneName, service)
	s.logger.Info("resolved nic.ru service for zone",
		"zone", zoneName,
		"service", service,
	)
	return service, nil
}

func extractRecordName(fqdn, zone string) string {
	name := strings.TrimSuffix(fqdn, ".")
	zoneName := strings.TrimSuffix(zone, ".")
	name = strings.TrimSuffix(name, "."+zoneName)
	return name
}
