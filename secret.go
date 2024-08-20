package main

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *DNSProviderSolver) getAccessToken() string {
	secret, err := c.client.CoreV1().Secrets(Namespace).Get(context.Background(), nameSecret, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Couldn't find or read the secret in getAccessToken func: %s", err)
	}

	accessToken := string(secret.Data["ACCESS_TOKEN"])

	return accessToken
}

func (c *DNSProviderSolver) getAppSecrets() (string, string) {
	secret, err := c.client.CoreV1().Secrets(Namespace).Get(context.Background(), nameSecret, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Couldn't find or read the secret in getAppSecrets func: %s", err)
	}

	appID := string(secret.Data["APP_ID"])
	appSecret := string(secret.Data["APP_SECRET"])

	return appID, appSecret
}

func (c *DNSProviderSolver) getRefreshToken() string {
	secret, err := c.client.CoreV1().Secrets(Namespace).Get(context.Background(), nameSecret, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Couldn't find or read the secret in getRefreshToken func: %s", err)
	}

	refreshToken := string(secret.Data["REFRESH_TOKEN"])

	return refreshToken
}
