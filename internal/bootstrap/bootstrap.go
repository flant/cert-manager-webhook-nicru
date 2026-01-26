// Package bootstrap handles initialization of OAuth tokens before the webhook starts.
// It ensures that valid tokens are available in Kubernetes secrets before serving requests.
package bootstrap

import (
	"fmt"
	"time"

	"github.com/flant/cert-manager-webhook-nicru/internal/metrics"
	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	"k8s.io/klog/v2"
)

// Bootstrap initializes tokens before starting webhook server
func Bootstrap(tm *tokenmanager.TokenManager, m *metrics.Metrics) error {
	klog.Info("Starting bootstrap process...")
	startTime := time.Now()

	// Attempt to ensure valid tokens
	if err := tm.EnsureValidTokens(); err != nil {
		m.BootstrapErrors.Inc()
		return fmt.Errorf("bootstrap failed: %w", err)
	}

	// Record successful bootstrap
	duration := time.Since(startTime).Seconds()
	m.BootstrapDuration.Observe(duration)
	m.BootstrapTotal.Inc()

	klog.Infof("Bootstrap completed successfully in %.2fs", duration)
	return nil
}
