// Package main is the entry point for the NIC.RU cert-manager webhook.
// This webhook implements ACME DNS-01 challenge validation for domains
// managed by NIC.RU DNS service.
package main

import (
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/flant/cert-manager-webhook-nicru/internal/bootstrap"
	"github.com/flant/cert-manager-webhook-nicru/internal/config"
	"github.com/flant/cert-manager-webhook-nicru/internal/cron"
	"github.com/flant/cert-manager-webhook-nicru/internal/dns"
	"github.com/flant/cert-manager-webhook-nicru/internal/metrics"
	"github.com/flant/cert-manager-webhook-nicru/internal/webhook"
	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func main() {
	if config.GroupName == "" {
		klog.Fatal("group name must be specified")
	}

	if config.Namespace == "" {
		klog.Fatal("namespace must be specified")
	}

	// Create Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("Failed to create in-cluster config: %v", err)
	}

	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		klog.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Initialize metrics
	m := metrics.New()

	// Create TokenManager configuration
	tmConfig := tokenmanager.Config{
		HTTPClient:       config.HTTPClient,
		OAuthURL:         config.OAuthURL,
		ZoneInfoURL:      config.URLGetZoneInfo,
		MetricsCallbacks: m.GetTokenManagerCallbacks(),
	}

	// Create TokenManager
	tm := tokenmanager.NewTokenManager(client, config.Namespace, tmConfig)

	// Run bootstrap to ensure tokens are available before starting webhook
	if err := bootstrap.Bootstrap(tm, m); err != nil {
		klog.Fatalf("Bootstrap failed: %v", err)
	}

	// Create DNS client
	dnsClient := dns.NewClient(config.HTTPClient, tm)

	// Create webhook solver
	solver := webhook.NewSolver(client, tm, dnsClient)

	// Start token refresh cron job in background
	refresher := cron.NewTokenRefresher(tm, m)
	go refresher.Start()

	// Start webhook server
	cmd.RunWebhookServer(config.GroupName, solver)
}
