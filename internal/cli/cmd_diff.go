package cli

import (
	"context"
	"os"

	"github.com/hashmap-kz/gopgmigrate/v2/internal/differ"
	cliv3 "github.com/urfave/cli/v3"
)

func CmdDiff() *cliv3.Command {
	return &cliv3.Command{
		Name:  "diff",
		Usage: "show schema changes introduced by pending migrations",
		Description: `Spins up a temporary Docker PostgreSQL container, replays applied and
pending migrations, dumps schema state at each checkpoint, and prints
a colored diff of the changes the pending migrations would introduce.

Requires Docker and git to be installed and available on PATH.

Examples:
   gopgmigrate diff --dsn $DSN
   gopgmigrate diff --dsn $DSN --no-color
   gopgmigrate diff --dsn $DSN --out-dir ./schema-snapshots`,
		Flags: []cliv3.Flag{
			flagDSN(),
			flagDir(),
			flagTable(),
			&cliv3.BoolFlag{
				Name:  "no-color",
				Usage: "disable colored output (also respected via NO_COLOR env var)",
			},
			&cliv3.StringFlag{
				Name:  "out-dir",
				Usage: "directory to write dump files into (default: temp dir)",
			},
		},
		Action: func(ctx context.Context, cmd *cliv3.Command) error {
			noColor := cmd.Bool("no-color") || os.Getenv("NO_COLOR") != ""
			return differ.Run(ctx, differ.Options{
				DSN:     cmd.String("dsn"),
				Dir:     cmd.String("dir"),
				Table:   cmd.String("table"),
				NoColor: noColor,
				OutDir:  cmd.String("out-dir"),
			})
		},
	}
}
