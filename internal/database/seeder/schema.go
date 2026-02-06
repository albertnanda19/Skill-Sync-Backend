package seeder

import (
	"context"
	"fmt"

	"skill-sync/internal/database"
)

func EnsureTableColumns(ctx context.Context, db database.DB, table string, columns ...string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if table == "" {
		return fmt.Errorf("empty table")
	}
	for _, col := range columns {
		if col == "" {
			return fmt.Errorf("empty column")
		}
	}

	rows, err := db.Query(
		ctx,
		`SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name=$1`,
		table,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := map[string]struct{}{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return err
		}
		existing[c] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, col := range columns {
		if _, ok := existing[col]; !ok {
			return fmt.Errorf("schema mismatch: missing column %s.%s", table, col)
		}
	}
	return nil
}
