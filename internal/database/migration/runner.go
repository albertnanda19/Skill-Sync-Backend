package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Runner struct {
	Dir string
}

func (r Runner) Run(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("nil db")
	}

	dir, err := resolveDir(r.Dir)
	if err != nil {
		return err
	}

	migs, err := loadMigrations(dir)
	if err != nil {
		return err
	}

	if len(migs) == 0 {
		return nil
	}

	if err := ensureSchemaMigrations(ctx, db); err != nil {
		return err
	}

	if err := advisoryLock(ctx, db, 746295114); err != nil {
		return err
	}
	defer func() {
		_ = advisoryUnlock(context.Background(), db, 746295114)
	}()

	applied, err := getApplied(ctx, db)
	if err != nil {
		return err
	}

	for _, m := range migs {
		if a, ok := applied[m.Version]; ok {
			if a.Checksum != m.Checksum {
				return fmt.Errorf("migration checksum mismatch: version=%d name=%s", m.Version, m.Name)
			}
			continue
		}

		if err := applyOne(ctx, db, m); err != nil {
			return err
		}
	}

	return nil
}

type Migration struct {
	Version  int64
	Name     string
	Filename string
	SQL      string
	Checksum string
}

type appliedMigration struct {
	Version  int64
	Checksum string
}

var fileRe = regexp.MustCompile(`^V(\d+)__([A-Za-z0-9_.-]+)\.sql$`)

func resolveDir(dir string) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return dir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "migrations"), nil
}

func loadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	migs := make([]Migration, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		m := fileRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		v, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid migration version: %s", name)
		}

		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		sqlText := strings.TrimSpace(string(b))
		if sqlText == "" {
			return nil, fmt.Errorf("empty migration file: %s", name)
		}

		h := sha256.Sum256([]byte(sqlText))
		migs = append(migs, Migration{
			Version:  v,
			Name:     m[2],
			Filename: name,
			SQL:      sqlText,
			Checksum: hex.EncodeToString(h[:]),
		})
	}

	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	for i := 1; i < len(migs); i++ {
		if migs[i].Version == migs[i-1].Version {
			return nil, fmt.Errorf("duplicate migration version: %d", migs[i].Version)
		}
	}

	return migs, nil
}

func ensureSchemaMigrations(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version BIGINT PRIMARY KEY,
	name TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	return err
}

func advisoryLock(ctx context.Context, db *sql.DB, key int64) error {
	_, err := db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, key)
	return err
}

func advisoryUnlock(ctx context.Context, db *sql.DB, key int64) error {
	_, err := db.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, key)
	return err
}

func getApplied(ctx context.Context, db *sql.DB) (map[int64]appliedMigration, error) {
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]appliedMigration{}
	for rows.Next() {
		var v int64
		var c string
		if err := rows.Scan(&v, &c); err != nil {
			return nil, err
		}
		out[v] = appliedMigration{Version: v, Checksum: c}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func applyOne(ctx context.Context, db *sql.DB, m Migration) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fmt.Errorf("apply migration failed: version=%d file=%s: %w", m.Version, m.Filename, err)
	}

	appliedAt := time.Now().UTC()
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES ($1, $2, $3, $4)`,
		m.Version,
		m.Name,
		m.Checksum,
		appliedAt,
	)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
