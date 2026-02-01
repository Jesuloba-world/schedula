package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"schedula/backend/internal/domain"
	"schedula/backend/internal/store"
)

func TestPostgresIntegration_AppointmentCreateListOverlapAndIdempotency(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("SCHEDULA_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("SCHEDULA_TEST_DATABASE_URL not set")
	}

	db, err := Open(databaseURL, PoolConfig{MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	t.Cleanup(func() {
		_ = Close(db)
	})

	schema := "schedula_test_" + randomHex(t, 8)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = db.NewRaw("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Exec(ctx)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw("CREATE SCHEMA " + schema).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewRaw("SET LOCAL search_path TO " + schema).Exec(ctx); err != nil {
			return err
		}
		if err := applyMigrations(ctx, tx); err != nil {
			return err
		}

		c := calendarTx{tx: tx}

		userID := "u1"
		start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
		end := start.Add(time.Hour)

		a1, err := c.CreateAppointment(ctx, domain.Appointment{
			ID:        uuid.MustParse("00000000-0000-0000-0000-000000000901"),
			UserID:    userID,
			Title:     "t",
			StartTime: start,
			EndTime:   end,
		})
		if err != nil {
			return err
		}

		rows, err := c.ListAppointments(ctx, userID, start.Add(-time.Minute), end.Add(time.Minute))
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return fmt.Errorf("len(rows) = %d, want 1", len(rows))
		}
		if rows[0].ID != a1.ID {
			return fmt.Errorf("listed id = %s, want %s", rows[0].ID, a1.ID)
		}

		_, err = c.CreateAppointment(ctx, domain.Appointment{
			ID:        uuid.MustParse("00000000-0000-0000-0000-000000000902"),
			UserID:    userID,
			Title:     "t2",
			StartTime: start.Add(30 * time.Minute),
			EndTime:   end.Add(30 * time.Minute),
		})
		if err != store.ErrConflict {
			return fmt.Errorf("overlap err = %v, want %v", err, store.ErrConflict)
		}

		a2, err := c.CreateAppointment(ctx, domain.Appointment{
			ID:        uuid.MustParse("00000000-0000-0000-0000-000000000903"),
			UserID:    userID,
			Title:     "t3",
			StartTime: end,
			EndTime:   end.Add(time.Hour),
		})
		if err != nil {
			return err
		}
		if a2.ID == uuid.Nil {
			return fmt.Errorf("expected non-nil id")
		}

		_, err = c.CreateAppointment(ctx, domain.Appointment{
			ID:        a1.ID,
			UserID:    userID,
			Title:     "t",
			StartTime: start,
			EndTime:   end,
		})
		if err != nil {
			return err
		}

		_, err = c.CreateAppointment(ctx, domain.Appointment{
			ID:        a1.ID,
			UserID:    userID,
			Title:     "different",
			StartTime: start,
			EndTime:   end,
		})
		if err != store.ErrIdempotencyConflict {
			return fmt.Errorf("idempotency err = %v, want %v", err, store.ErrIdempotencyConflict)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("tx error: %v", err)
	}
}

func randomHex(t *testing.T, bytesLen int) string {
	t.Helper()
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand.Read error: %v", err)
	}
	return hex.EncodeToString(b)
}

type rawExecutor interface {
	NewRaw(query string, args ...any) *bun.RawQuery
}

func applyMigrations(ctx context.Context, exec rawExecutor) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type mig struct {
		name string
		path string
	}
	migs := make([]mig, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		migs = append(migs, mig{name: e.Name(), path: filepath.Join(dir, e.Name())})
	}
	sort.Slice(migs, func(i, j int) bool { return migs[i].name < migs[j].name })

	for _, m := range migs {
		b, err := os.ReadFile(m.path)
		if err != nil {
			return err
		}
		upSQL, err := extractGooseUp(string(b))
		if err != nil {
			return err
		}
		stmts := splitSQLStatements(upSQL)
		for _, stmt := range stmts {
			if normalized, ok := normalizeExtensionStatement(stmt); ok {
				stmt = normalized
			}
			if _, err := exec.NewRaw(stmt).Exec(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func migrationsDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	base := filepath.Dir(file)
	return filepath.Clean(filepath.Join(base, "..", "..", "..", "migrations")), nil
}

func extractGooseUp(sql string) (string, error) {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"

	upIdx := strings.Index(sql, upMarker)
	if upIdx < 0 {
		return "", fmt.Errorf("missing goose up marker")
	}
	afterUp := sql[upIdx+len(upMarker):]
	afterUp = strings.TrimLeft(afterUp, "\r\n")

	downIdx := strings.Index(afterUp, downMarker)
	if downIdx < 0 {
		return strings.TrimSpace(afterUp), nil
	}
	return strings.TrimSpace(afterUp[:downIdx]), nil
}

func normalizeExtensionStatement(stmt string) (string, bool) {
	s := strings.TrimSpace(stmt)
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "CREATE EXTENSION") {
		return "", false
	}
	if !strings.Contains(upper, "BTREE_GIST") {
		return "", false
	}
	if strings.Contains(upper, " SCHEMA ") {
		return "", false
	}
	return s + " SCHEMA public", true
}

func splitSQLStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
