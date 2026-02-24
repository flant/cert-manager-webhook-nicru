package nicru

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	s.logger.Info("solver_initialize", "namespace", s.namespace, "secret", s.secretName)

	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("create k8s clientset: %w", err)
	}
	s.k8s = cl

	s.logger.Info("solver_initialized", "namespace", s.namespace)
	go s.runTokenManager(stopCh)
	return nil
}

func (s *Solver) Present(ch *v1alpha1.ChallengeRequest) error {
	start := time.Now()
	zoneName := strings.TrimSuffix(ch.ResolvedZone, ".")
	recordName := extractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)

	s.logger.Info("present_started",
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
	s.logger.Info("present_token_ok", "uid", string(ch.UID))

	service, err := s.resolveService(token, zoneName)
	if err != nil {
		return fmt.Errorf("resolve service for zone %q: %w", zoneName, err)
	}
	s.logger.Info("present_service_resolved",
		"uid", string(ch.UID),
		"zone", zoneName,
		"service", service,
	)

	s.logger.Info("present_creating_record",
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

	s.logger.Info("present_completed",
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

	s.logger.Info("cleanup_started",
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
	s.logger.Info("cleanup_token_ok", "uid", string(ch.UID))

	service, err := s.resolveService(token, zoneName)
	if err != nil {
		return fmt.Errorf("resolve service for zone %q: %w", zoneName, err)
	}

	s.logger.Info("cleanup_searching_record",
		"uid", string(ch.UID),
		"service", service,
		"zone", zoneName,
		"record", recordName,
	)

	rrID, err := s.api.FindTXTRecord(token, service, zoneName, recordName, ch.Key)
	if err != nil {
		return fmt.Errorf("find TXT record in zone %q: %w", zoneName, err)
	}

	s.logger.Info("cleanup_record_found",
		"uid", string(ch.UID),
		"rr_id", rrID,
		"record", recordName,
	)

	s.logger.Info("cleanup_deleting_record",
		"uid", string(ch.UID),
		"rr_id", rrID,
		"zone", zoneName,
	)

	if err := s.api.DeleteRecord(token, service, zoneName, rrID); err != nil {
		return fmt.Errorf("delete record %s in zone %q: %w", rrID, zoneName, err)
	}

	s.logger.Info("cleanup_completed",
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
		s.logger.Info("service_cache_hit", "zone", zoneName, "service", cached.(string))
		return cached.(string), nil
	}

	s.logger.Info("service_cache_miss", "zone", zoneName)
	service, err := s.api.GetServiceForZone(token, zoneName)
	if err != nil {
		return "", err
	}

	s.serviceCache.Store(zoneName, service)
	s.logger.Info("service_resolved", "zone", zoneName, "service", service)
	return service, nil
}

func extractRecordName(fqdn, zone string) string {
	name := strings.TrimSuffix(fqdn, ".")
	zoneName := strings.TrimSuffix(zone, ".")
	name = strings.TrimSuffix(name, "."+zoneName)
	return name
}

func loadConfig(cfgJSON *extapi.JSON) (solverConfig, error) {
	var cfg solverConfig
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("decode solver config: %w", err)
	}
	return cfg, nil
}
