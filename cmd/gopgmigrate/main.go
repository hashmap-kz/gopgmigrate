package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"

	gopgmigratecli "github.com/hashmap-kz/gopgmigrate/v2/internal/cli"
	"github.com/hashmap-kz/gopgmigrate/v2/internal/manifest"
	cliv3 "github.com/urfave/cli/v3"
)

var Version = "dev"

func main() {
	cmd := &cliv3.Command{
		Name:    "gopgmigrate",
		Usage:   "SQL-first PostgreSQL migrations",
		Version: Version,
		Flags: []cliv3.Flag{
			&cliv3.StringFlag{
				Name:  "log-level",
				Usage: "log level (debug, info, warn, error)",
				Value: "warn",
			},
		},
		Before: func(ctx context.Context, cmd *cliv3.Command) (context.Context, error) {
			configureLogging(cmd.String("log-level"))
			return ctx, nil
		},
		Commands: []*cliv3.Command{
			gopgmigratecli.CmdApply(),
			gopgmigratecli.CmdPlan(),
			gopgmigratecli.CmdStatus(),
			gopgmigratecli.CmdValidate(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", slog.Any("err", err))
		var stray *manifest.StrayFilesError
		if errors.As(err, &stray) {
			os.Exit(3)
		}
		os.Exit(1)
	}
}

func configureLogging(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelWarn
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}
