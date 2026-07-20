package migrate

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var files embed.FS

func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(9463321)`); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(9463321)`)
	}()
	entries, err := files.ReadDir("sql")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		n := strings.SplitN(name, "_", 2)[0]
		version, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return fmt.Errorf("parse migration %s: %w", name, err)
		}
		var exists bool
		err = conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists)
		// 001 creates schema_migrations, therefore it has to be tried once before this check.
		if err != nil && version != 1 {
			return err
		}
		if exists {
			continue
		}
		body, err := files.ReadFile("sql/" + name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(body)); err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1) ON CONFLICT DO NOTHING`, version)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
