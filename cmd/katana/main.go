package main

import (
	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
	"os"
)

// main is the application entrypoint.
func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	appCtx := &appContext{
		logger: logger,
	}
	app := &cli.App{
		Name:  "katana",
		Usage: "CLI interface for the Katana server",
		Commands: []*cli.Command{
			{
				Name:  "server",
				Usage: "launches the server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "the configuration path, defaults to config.toml",
						Value:   "config.toml",
					},
				},
				Action: appCtx.handleServer,
			},
			{
				Name:  "config",
				Usage: "generates an example configuration file",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
						Usage:   "the configuration path, defaults to config.toml",
						Value:   "config.toml",
					},
				},
				Action: appCtx.handleConfig,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		logger.Fatal("failed to run cli", zap.Error(err))
	}
}
