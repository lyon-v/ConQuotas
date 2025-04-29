package main

import (
	"RootfsQuota/pkg/handler"
	"RootfsQuota/pkg/log"
	"flag"
	"os"

	"go.uber.org/zap"
)

func main() {
	log.Info("RootfsQuota is starting...")

	configPath := flag.String("config", "/etc/containerd-quota/config.json", "Path to configuration file")
	flag.Parse()

	quota, err := handler.NewRFSQuota(*configPath)
	if err != nil {
		log.Error("Failed to initialize RFSQuota", zap.Error(err))
		os.Exit(1)
	}

	if err := quota.Run(); err != nil {
		log.Error("Service exited with error", zap.Error(err))
		os.Exit(1)
	}
	log.Info("RootfsQuota shutdown gracefully")
}
