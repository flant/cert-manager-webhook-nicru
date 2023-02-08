package main

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *nicruDNSProviderSolver) getSecretData() (refreshToken, accessToken string) {

	secret, err := c.client.CoreV1().Secrets(NAMESPACE).Get(context.TODO(), nameSecret, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Сouldn't find or read the secret: %s", err)
	}

	refreshToken = string(secret.Data["REFRESH_TOKEN"])
	accessToken = string(secret.Data["ACCESS_TOKEN"])

	return refreshToken, accessToken

}
