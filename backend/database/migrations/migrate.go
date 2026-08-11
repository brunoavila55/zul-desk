package migrations

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.up.sql
var files embed.FS

func Run(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	entries, err := files.ReadDir(".")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	var appliedCount int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&appliedCount); err != nil {
		return err
	}
	if appliedCount == 0 {
		baseline, err := detectBaseline(ctx, db)
		if err != nil {
			return err
		}
		for _, name := range names {
			version, parseErr := migrationVersion(name)
			if parseErr != nil {
				return parseErr
			}
			if version <= baseline {
				if _, err := db.Exec(ctx, "INSERT INTO schema_migrations(version,name) VALUES($1,$2) ON CONFLICT DO NOTHING", version, name); err != nil {
					return err
				}
			}
		}
	}

	for _, name := range names {
		version, err := migrationVersion(name)
		if err != nil {
			return err
		}
		var exists bool
		if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := files.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version,name) VALUES($1,$2)", version, name)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("invalid migration name %q", name)
	}
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("invalid migration version %q: %w", name, err)
	}
	return version, nil
}

func detectBaseline(ctx context.Context, db *pgxpool.Pool) (int, error) {
	checks := []struct {
		version int
		query   string
	}{
		{6, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='app_settings' AND column_name='whatsapp_verify_token_encrypted')`},
		{5, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='messages' AND column_name='media_path')`},
		{4, `SELECT to_regclass('public.user_groups') IS NOT NULL`},
		{3, `SELECT to_regclass('public.app_settings') IS NOT NULL`},
		{2, `SELECT to_regclass('public.whatsapp_accounts') IS NOT NULL`},
		{1, `SELECT to_regclass('public.users') IS NOT NULL`},
	}
	for _, check := range checks {
		var exists bool
		if err := db.QueryRow(ctx, check.query).Scan(&exists); err != nil {
			return 0, err
		}
		if exists {
			return check.version, nil
		}
	}
	return 0, nil
}
