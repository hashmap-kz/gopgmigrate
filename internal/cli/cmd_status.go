package cli

import (
	"context"
	"fmt"

	"github.com/hashmap-kz/gopgmigrate/v2/pkg/migrator"
	"github.com/urfave/cli/v3"
)

func CmdStatus() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "show applied/pending state of all manifest entries",
		Flags: []cli.Flag{
			flagDSN(),
			flagManifest(),
			flagTable(),
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			m, err := migrator.NewWithDSN(cmd.String("dsn"), migrator.Config{
				ManifestPath: cmd.String("manifest"),
				Table:        cmd.String("table"),
			})
			if err != nil {
				return err
			}
			defer m.Close()

			statuses, err := m.Status(ctx)
			if err != nil {
				return err
			}

			pathW := len("PATH")
			kindW := len("KIND")
			for _, s := range statuses {
				if len(s.Path) > pathW {
					pathW = len(s.Path)
				}
				if len(s.Kind) > kindW {
					kindW = len(s.Kind)
				}
			}

			fmt.Printf("%-*s  %-*s  %s\n", pathW, "PATH", kindW, "KIND", "APPLIED_AT")
			for _, s := range statuses {
				appliedAt := "-"
				if s.Applied && !s.AppliedAt.IsZero() {
					appliedAt = s.AppliedAt.UTC().Format("2006-01-02 15:04:05")
				}
				fmt.Printf("%-*s  %-*s  %s\n", pathW, s.Path, kindW, s.Kind, appliedAt)
			}
			return nil
		},
	}
}
