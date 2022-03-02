package main

import (
	"context"
	"github.com/aibotsoft/crypto-collector/pkg/config"
	"github.com/aibotsoft/crypto-collector/pkg/logger"
	"github.com/aibotsoft/crypto-collector/pkg/signals"
	"github.com/aibotsoft/crypto-collector/pkg/version"
	"github.com/aibotsoft/crypto-collector/services/collector"
	"go.uber.org/zap"
)

func main() {
	cfg := config.NewConfig()
	log, err := logger.NewLogger(cfg)
	if err != nil {
		panic(err)
	}
	log.Info("start_service", zap.String("version", version.Version), zap.String("build_date", version.BuildDate), zap.Any("config", cfg))

	ctx, cancel := context.WithCancel(context.Background())
	c := collector.NewCollector(cfg, log, ctx)
	errCh := make(chan error)
	go func() {
		errCh <- c.Run()
	}()
	defer func() {
		log.Info("closing_services...")
		cancel()
		err := c.Close()
		if err != nil {
			log.Error("close_collector_error", zap.Error(err))
		}
		_ = log.Sync()
	}()

	stopCh := signals.SetupSignalHandler()
	select {
	case err := <-errCh:
		log.Error("stop_service_by_error", zap.Error(err))
	case sig := <-stopCh:
		log.Info("stop_service_by_os_signal", zap.String("signal", sig.String()))
	}
}
