package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io/fs"
	"sort"
	"strings"
)

type Migration struct {
	Module              string
	Version             int
	Name, SQL, Checksum string
}

func LoadMigrations(fsys fs.FS, module, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	out := []Migration{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		var version int
		if _, err := fmt.Sscanf(e.Name(), "%04d_", &version); err != nil {
			return nil, fmt.Errorf("migration %s: %w", e.Name(), err)
		}
		b, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		out = append(out, Migration{module, version, e.Name(), string(b), hex.EncodeToString(sum[:])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1)", int64(0x434f4e4e454354)); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", int64(0x434f4e4e454354))
	if _, err = conn.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS core; CREATE TABLE IF NOT EXISTS core.schema_migrations(module text NOT NULL, version integer NOT NULL, name text NOT NULL, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(module,version))`); err != nil {
		return err
	}
	for _, m := range migrations {
		var existing string
		err = conn.QueryRow(ctx, "SELECT checksum FROM core.schema_migrations WHERE module=$1 AND version=$2", m.Module, m.Version).Scan(&existing)
		if err == nil {
			if existing != m.Checksum {
				return fmt.Errorf("checksum divergente: %s", m.Name)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, m.SQL); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO core.schema_migrations(module,version,name,checksum) VALUES($1,$2,$3,$4)", m.Module, m.Version, m.Name, m.Checksum)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", m.Name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
