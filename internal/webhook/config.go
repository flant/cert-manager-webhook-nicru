// Package webhook implements the cert-manager ACME DNS-01 challenge webhook for NIC.RU.
// It provides a Solver that creates and deletes TXT records in NIC.RU DNS zones
// to validate domain ownership for Let's Encrypt certificates.
package webhook

import (
	"encoding/json"
	"fmt"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Config represents the webhook configuration
type Config struct {
	NicruAPITokenSecretRef cmmeta.SecretKeySelector `json:"nicruTokenSecretRef"`
}

// LoadConfig loads and decodes the webhook configuration
func LoadConfig(cfgJSON *extapi.JSON) (Config, error) {
	cfg := Config{}
	if cfgJSON == nil {
		return cfg, nil
	}

	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to decode config: %w", err)
	}
	return cfg, nil
}
