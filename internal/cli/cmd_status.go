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

			fmt.Printf("%-8s %-12s %-10s %s\n", "APPLIED", "KIND", "CHECKSUM", "PATH")
			fmt.Println("---------------------------------------------------------------")
			for _, s := range statuses {
				applied := "no"
				if s.Applied {
					applied = "yes"
				}
				checksum := s.Checksum
				if len(checksum) > 8 {
					checksum = checksum[:8]
				}
				fmt.Printf("%-8s %-12s %-10s %s\n", applied, s.Kind, checksum+"...", s.Path)
				if s.Description != "" {
					fmt.Printf("         desc: %s\n", s.Description)
				}
			}
			return nil
		},
	}
}
