package main

import (
	"log/slog"
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/flant/cert-manager-webhook-nicru/nicru"
	_ "k8s.io/component-base/logs/json/register"
)

var appVersion string

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	version := appVersion
	if version == "" {
		version = "dev"
	}
	logger.Info("starting cert-manager-webhook-nicru", "version", version)

	groupName := os.Getenv("GROUP_NAME")
	if groupName == "" {
		logger.Error("GROUP_NAME environment variable is required")
		os.Exit(1)
	}

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		logger.Error("NAMESPACE environment variable is required")
		os.Exit(1)
	}

	secretName := os.Getenv("SECRET_NAME")
	if secretName == "" {
		secretName = nicru.DefaultSecretName
	}

	os.Args = append(os.Args, "--logging-format=json")

	solver := nicru.NewSolver(namespace, secretName, logger)

	cmd.RunWebhookServer(groupName, solver)
}
