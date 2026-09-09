package postgres

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jamersom/b3-data-hub/internal/infra/config"
	"github.com/jamersom/b3-data-hub/internal/infra/database"
	"github.com/joho/godotenv"
)

func TestObservedCalendarMigration(t *testing.T) {
	if os.Getenv("CALENDAR_INTEGRATION_TEST") != "1" {
		t.Skip("set CALENDAR_INTEGRATION_TEST=1 for isolated transactional fixtures")
	}
	_ = godotenv.Load("../../../../.env")
	cfg, err := config.LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	schema := pgx.Identifier{fmt.Sprintf("calendar_test_%d", time.Now().UnixNano())}.Sanitize()
	if _, err = tx.Exec(ctx, "CREATE SCHEMA "+schema+"; SET LOCAL search_path TO "+schema); err != nil {
		t.Fatal(err)
	}
	// Isolated minimal source tables: no writes to published production data.
	_, err = tx.Exec(ctx, `CREATE TABLE historical_imports(id bigint PRIMARY KEY, reference_year int, status text, total_records bigint, file_sha256 text, parser_version text, layout_version text, published_at timestamptz);
CREATE TABLE historical_quotes(id bigint PRIMARY KEY, import_id bigint, market_type int, trading_date date, ticker text, trade_count int);
INSERT INTO historical_imports VALUES
(1,2020,'published',4,'hash1','1.2','layout','2021-01-01Z'),
(2,2021,'failed',1,'hash2','1.2','layout','2022-01-01Z'),
(3,2022,'published',2,'hash3','1.2','layout','2023-01-01Z');
INSERT INTO historical_quotes VALUES
(1,1,10,'2020-01-02','PETR4',1),(2,1,10,'2020-01-02','VALE3',1),
(3,1,10,'2020-01-03','VALE3',1),(4,1,20,'2020-01-06','OTHER',1),
(5,2,10,'2021-01-04','PETR4',1),(6,3,10,'2022-01-03','PETR4',1);`)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(name string) {
		t.Helper()
		raw, err := os.ReadFile("../../../../migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		sql := strings.ReplaceAll(strings.ReplaceAll(string(raw), "BEGIN;", ""), "COMMIT;", "")
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	apply("003_observed_trading_calendar.up.sql")
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_trading_sessions WHERE market_type=10 AND reference_year=2020").Scan(&count); err != nil || count != 2 {
		t.Fatalf("sessions=%d err=%v", count, err)
	}
	var valid, official bool
	var version string
	if err = tx.QueryRow(ctx, "SELECT import_integrity_validated, official_calendar_verified, calendar_version FROM observed_calendar_coverage WHERE import_id=1 AND market_type=10").Scan(&valid, &official, &version); err != nil || !valid || official {
		t.Fatalf("coverage valid=%v official=%v err=%v", valid, official, err)
	}
	if err = tx.QueryRow(ctx, "SELECT import_integrity_validated FROM observed_calendar_coverage WHERE import_id=3").Scan(&valid); err != nil || valid {
		t.Fatalf("incomplete import accepted: %v", err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_calendar_coverage WHERE import_id=2").Scan(&count); err != nil || count != 0 {
		t.Fatal("failed import exposed")
	}
	_, err = tx.Exec(ctx, "UPDATE historical_quotes SET trading_date='2020-01-07' WHERE id=3")
	if err != nil {
		t.Fatal(err)
	}
	var updated string
	if err = tx.QueryRow(ctx, "SELECT calendar_version FROM observed_calendar_coverage WHERE import_id=1 AND market_type=10").Scan(&updated); err != nil || updated == version {
		t.Fatal("version did not reflect changed session")
	}
	_, err = tx.Exec(ctx, "UPDATE historical_imports SET status='superseded' WHERE id=1")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_trading_sessions WHERE import_id=1").Scan(&count); err != nil || count != 0 {
		t.Fatal("superseded sessions exposed")
	}
	apply("003_observed_trading_calendar.up.sql") // Repeatable migration.
	apply("004_consolidated_trading_calendar.up.sql")
	// Cache has exactly the same rows as the source aggregations after backfill.
	var different bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS (
 (SELECT * FROM observed_calendar_coverage EXCEPT SELECT * FROM observed_calendar_coverage_source)
 UNION ALL
 (SELECT * FROM observed_calendar_coverage_source EXCEPT SELECT * FROM observed_calendar_coverage)
 )`).Scan(&different); err != nil || different {
		t.Fatalf("backfill mismatch: %v", err)
	}
	if _, err = tx.Exec(ctx, `SAVEPOINT publication;
 UPDATE historical_imports SET status='processing' WHERE id=3;
 INSERT INTO historical_quotes VALUES(7,3,10,'2022-01-04','VALE3',1);
 UPDATE historical_imports SET status='published', total_records=2 WHERE id=3;`); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_trading_sessions WHERE import_id=3").Scan(&count); err != nil || count != 2 {
		t.Fatalf("publication not refreshed: count=%d err=%v", count, err)
	}
	if err = tx.QueryRow(ctx, "SELECT import_integrity_validated FROM observed_calendar_coverage WHERE import_id=3").Scan(&valid); err != nil || !valid {
		t.Fatal("coverage not refreshed")
	}
	if _, err = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT publication"); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_trading_sessions WHERE import_id=3").Scan(&count); err != nil || count != 1 {
		t.Fatal("calendar did not roll back atomically")
	}
	if _, err = tx.Exec(ctx, "UPDATE historical_imports SET status='superseded' WHERE id=3"); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_trading_sessions WHERE import_id=3").Scan(&count); err != nil || count != 0 {
		t.Fatal("superseded cache not removed")
	}
	if _, err = tx.Exec(ctx, "INSERT INTO historical_imports VALUES(4,2023,'published',0,'hash4','1.2','layout','2024-01-01Z')"); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_calendar_coverage WHERE import_id=4").Scan(&count); err != nil || count != 1 {
		t.Fatal("insert publication not represented")
	}
	if _, err = tx.Exec(ctx, "DELETE FROM historical_imports WHERE id=4"); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM observed_calendar_coverage WHERE import_id=4").Scan(&count); err != nil || count != 0 {
		t.Fatal("deleted cache not removed")
	}
	apply("004_consolidated_trading_calendar.up.sql")
	apply("004_consolidated_trading_calendar.down.sql")
	apply("003_observed_trading_calendar.down.sql")
	var absent bool
	if err = tx.QueryRow(ctx, "SELECT to_regclass('observed_trading_sessions') IS NULL").Scan(&absent); err != nil || !absent {
		t.Fatal("down migration failed")
	}
}
