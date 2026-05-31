package main

import (
	"context"
	"log/slog"
	"os"

	cli2 "github.com/hashmap-kz/gopgmigrate/v2/internal/cli"
	"github.com/urfave/cli/v3"
)

var Version = "dev"

func main() {
	cmd := &cli.Command{
		Name:    "gopgmigrate",
		Usage:   "YAML-manifest-driven PostgreSQL migrations",
		Version: Version,
		Commands: []*cli.Command{
			cli2.CmdUp(),
			cli2.CmdStatus(),
			cli2.CmdValidate(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", slog.Any("err", err))
		os.Exit(1)
	}
}
