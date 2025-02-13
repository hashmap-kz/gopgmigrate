package cmd

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"gopgmigrate/internal/history"
	"gopgmigrate/internal/history/impl"

	"gopgmigrate/internal/dbms"
)

// TODO: this should be an interface
// TODO: simplify, cleanup
func initRepo(ctx context.Context) (history.MigrateHistoryRepository, *sql.DB) {
	var err error
	var repo history.MigrateHistoryRepository
	var conn *sql.DB

	if cliOptions.dbms == dbmsVendorPostgresql {
		repo = impl.NewMigrateHistoryPostgresRepository(ctx, cliOptions.historyTableName)
		conn, err = dbms.GetDatabaseConnectionPostgres(cliOptions.connStr)
		if err != nil {
			slog.Error("database connection error", slog.String("err", err.Error()))
			os.Exit(1)
		}

		err = repo.CreateHistoryTable(ctx, conn)
		if err != nil {
			slog.Error("cannot create history-table", slog.String("err", err.Error()))
			os.Exit(1)
		}

	} else {
		slog.Error("unknown DBMS vendor", slog.String("name", cliOptions.dbms))
		os.Exit(1)
	}

	return repo, conn
}
