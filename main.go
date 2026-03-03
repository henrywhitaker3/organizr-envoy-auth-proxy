package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/henrywhitaker3/organizr-envoy-auth-proxy/internal/config"
	"github.com/henrywhitaker3/organizr-envoy-auth-proxy/internal/http"
)

var (
	version = "dev"
)

func main() {
	ctx, canel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer canel()

	conf, err := config.Parse()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel(conf.LogLevel),
	})))

	slog.Debug("loaded config", "config", conf, "version", version)

	org, err := conf.Organizr.URL()
	if err != nil {
		slog.Error("build organizr url", "error", err)
		os.Exit(1)
	}

	http := http.New(http.Options{
		URL:  org,
		Port: conf.Port,
		UUID: conf.Organizr.UUID,
	})
	go func() {
		slog.Info("starting http server", "port", conf.Port)
		if err := http.Start(); err != nil {
			slog.Error("could not start http server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	http.Shutdown()
}

func logLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "error":
		return slog.LevelError
	case "info":
		fallthrough
	default:
		return slog.LevelInfo
	}
}
