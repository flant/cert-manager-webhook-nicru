package main

import (
	"context"
	"encoding/json"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

func (c *nicruDNSProviderSolver) patchSecret(newRefreshToken, newAccessToken string) {
	updateSecret := v1.Secret{
		Data: map[string][]byte{
			"REFRESH_TOKEN": []byte(newRefreshToken),
			"ACCESS_TOKEN":  []byte(newAccessToken),
		},
	}

	payload, err := json.Marshal(updateSecret)
	if err != nil {
		panic(err.Error())
	}
	_, err = c.client.CoreV1().Secrets(NAMESPACE).Patch(context.TODO(), nameSecret, types.StrategicMergePatchType, payload, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Couldn't patch the secret: %s", err)
	}
}
