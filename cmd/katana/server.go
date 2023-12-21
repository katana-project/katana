package main

import (
	"github.com/go-faster/errors"
	"github.com/katana-project/katana/config"
	"github.com/katana-project/katana/server"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
)

// handleServer handles the server sub-command.
func (ac *appContext) handleServer(cCtx *cli.Context) (err error) {
	cfg, err := config.ParseWithDefaults(cCtx.String("config"))
	if err != nil {
		return errors.Wrap(err, "failed to load config")
	}

	handler, err := server.NewConfiguredRouter(cfg, ac.logger)
	if err != nil {
		return errors.Wrap(err, "failed to configure router")
	}
	defer func() {
		if err0 := handler.Close(); err0 != nil {
			err = multierr.Append(err, errors.Wrap(err0, "failed to close handler"))
		}
	}()

	var (
		httpServer = &http.Server{Addr: cfg.HTTP.Host, Handler: handler}
		errorChan  = make(chan error)
	)
	go func() {
		ac.logger.Info("listening for http requests", zap.String("addr", httpServer.Addr))
		errorChan <- httpServer.ListenAndServe()
	}()

	ctx, stop := signal.NotifyContext(cCtx.Context, os.Interrupt)
	defer stop()

	select {
	case <-ctx.Done():
		ac.logger.Info("shutting down gracefully")
		if err = httpServer.Shutdown(ctx); err != nil {
			err = errors.Wrap(err, "failed to shutdown http server")
		}
	case err = <-errorChan:
		err = errors.Wrap(err, "http server errored")
	}

	return err
}
