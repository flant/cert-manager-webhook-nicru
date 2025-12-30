// Package metrics provides Prometheus metrics collection for the NIC.RU webhook.
// It tracks token refresh operations, validation status, and bootstrap performance.
package metrics

import (
	"github.com/flant/cert-manager-webhook-nicru/pkg/tokenmanager"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the webhook
type Metrics struct {
	TokenRefreshTotal       prometheus.Counter
	TokenRefreshErrors      prometheus.Counter
	TokenValidationTotal    prometheus.Counter
	LastTokenRefreshTime    prometheus.Gauge
	LastTokenValidationTime prometheus.Gauge
	TokenValidationStatus   prometheus.Gauge
	BootstrapDuration       prometheus.Histogram
	BootstrapTotal          prometheus.Counter
	BootstrapErrors         prometheus.Counter
}

// New creates and registers all Prometheus metrics
func New() *Metrics {
	return &Metrics{
		TokenRefreshTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nicru_webhook_token_refresh_total",
			Help: "Total number of successful token refreshes",
		}),
		TokenRefreshErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nicru_webhook_token_refresh_errors_total",
			Help: "Total number of token refresh errors",
		}),
		TokenValidationTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nicru_webhook_token_validation_total",
			Help: "Total number of token validations performed",
		}),
		LastTokenRefreshTime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nicru_webhook_last_token_refresh_timestamp",
			Help: "Unix timestamp of last successful token refresh",
		}),
		LastTokenValidationTime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nicru_webhook_last_token_validation_timestamp",
			Help: "Unix timestamp of last token validation",
		}),
		TokenValidationStatus: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "nicru_webhook_token_validation_status",
			Help: "Status of last token validation (1=valid, 0=invalid)",
		}),
		BootstrapDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "nicru_webhook_bootstrap_duration_seconds",
			Help:    "Time taken for bootstrap process in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		BootstrapTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nicru_webhook_bootstrap_total",
			Help: "Total number of bootstrap attempts",
		}),
		BootstrapErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "nicru_webhook_bootstrap_errors_total",
			Help: "Total number of bootstrap failures",
		}),
	}
}

// GetTokenManagerCallbacks returns callbacks for TokenManager metrics
func (m *Metrics) GetTokenManagerCallbacks() tokenmanager.MetricsCallbacks {
	return tokenmanager.MetricsCallbacks{
		IncTokenRefreshTotal:       func() { m.TokenRefreshTotal.Inc() },
		IncTokenRefreshErrors:      func() { m.TokenRefreshErrors.Inc() },
		IncTokenValidationTotal:    func() { m.TokenValidationTotal.Inc() },
		SetLastTokenRefreshTime:    func() { m.LastTokenRefreshTime.SetToCurrentTime() },
		SetLastTokenValidationTime: func() { m.LastTokenValidationTime.SetToCurrentTime() },
		SetTokenValidationStatus:   func(v float64) { m.TokenValidationStatus.Set(v) },
	}
}
