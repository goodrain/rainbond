package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/furutachiKurea/block-mechanica/internal/config"
	"github.com/furutachiKurea/block-mechanica/internal/k8s"
	"github.com/furutachiKurea/block-mechanica/internal/log"
	"github.com/furutachiKurea/block-mechanica/service"
)

func main() {
	cfg := config.MustLoad()

	log.InitLogger()
	defer log.Sync()

	log.Info("configuration loaded successfully",
		log.String("host", cfg.Host),
		log.String("port", cfg.Port))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGKILL)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal("application exited", log.Err(err))
	}
}

func run(ctx context.Context) error {
	mgr, err := k8s.NewManager()
	if err != nil {
		return fmt.Errorf("create manager: %w", err)
	}

	services := service.New(mgr.GetClient())

	if err := k8s.Setup(ctx, mgr, services); err != nil {
		return fmt.Errorf("setup manager: %w", err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("start manager: %w", err)
	}

	return nil
}
