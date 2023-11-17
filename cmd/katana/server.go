package main

import (
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/config"
	"github.com/katana-project/katana/server"
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
)

func (ac *appContext) handleServer(cCtx *cli.Context) error {
	cfg, err := config.Parse(cCtx.String("config"))
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	handler, err := server.NewConfiguredRouter(cfg, ac.logger)
	if err != nil {
		return errors.Wrap(err, "failed to configure router")
	}

	var (
		httpServer = &http.Server{Addr: cfg.HTTP.Host, Handler: handler}
		errorChan  = make(chan error)
	)
	go func() {
		ac.logger.Info("listening for http requests", zap.String("address", httpServer.Addr))
		errorChan <- httpServer.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(cCtx.Context, os.Interrupt)
	defer stop()

	select {
	case <-ctx.Done():
		ac.logger.Info("shutting down gracefully")
		if err := httpServer.Shutdown(ctx); err != nil {
			return errors.Wrap(err, "failed to shutdown http server")
		}
	case err := <-errorChan:
		return errors.Wrap(err, "http server errored")
	}

	return nil
}
