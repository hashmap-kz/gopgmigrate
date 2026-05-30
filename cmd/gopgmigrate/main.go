package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "gopgmigrate",
		Usage: "YAML-manifest-driven PostgreSQL migrations",
		Commands: []*cli.Command{
			cmdUp(),
			cmdStatus(),
			cmdValidate(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
