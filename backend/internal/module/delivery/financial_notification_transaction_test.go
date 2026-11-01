package delivery

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// A narrow test-only SQL connector, following notification/event_repository_test.go.
// The production GORM repository executes every statement and transaction; no
// database or network is used, and this is not MySQL isolation acceptance.
type financialSQLStep struct {
	kind, contains string
	columns        []string
	rows           [][]driver.Value
	err            error
}

type financialSQLScript struct {
	steps                         []financialSQLStep
	active                        bool
	stagedWrites, committedWrites int
	commits, rollbacks, calls     int
	unexpected                    error
}

func (s *financialSQLScript) next(kind, query string) (financialSQLStep, error) {
	if len(s.steps) == 0 || s.steps[0].kind != kind || !strings.Contains(query, s.steps[0].contains) {
		s.unexpected = fmt.Errorf("unexpected %s: %s", kind, query)
		return financialSQLStep{}, s.unexpected
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	return step, step.err
}

type financialSQLConnector struct{ script *financialSQLScript }

func (c financialSQLConnector) Connect(context.Context) (driver.Conn, error) {
	return financialSQLConn{c.script}, nil
}
func (c financialSQLConnector) Driver() driver.Driver { return financialSQLDriver{} }

type financialSQLDriver struct{}

func (financialSQLDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("connector required")
}

type financialSQLConn struct{ script *financialSQLScript }

func (c financialSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}
func (c financialSQLConn) Close() error { return nil }
func (c financialSQLConn) Begin() (driver.Tx, error) {
	if _, err := c.script.next("begin", ""); err != nil {
		return nil, err
	}
	if c.script.active {
		return nil, errors.New("nested transaction")
	}
	c.script.active = true
	return financialSQLTx{c.script}, nil
}
func (c financialSQLConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return c.Begin()
}
func (c financialSQLConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if !c.script.active {
		return nil, errors.New("write escaped transaction")
	}
	if _, err := c.script.next("exec", query); err != nil {
		return nil, err
	}
	c.script.stagedWrites++
	return financialSQLResult{}, nil
}
func (c financialSQLConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if !c.script.active {
		return nil, errors.New("query escaped transaction")
	}
	step, err := c.script.next("query", query)
	if err != nil {
		return nil, err
	}
	return &financialSQLRows{columns: step.columns, rows: step.rows}, nil
}

type financialSQLTx struct{ script *financialSQLScript }

func (tx financialSQLTx) Commit() error {
	if _, err := tx.script.next("commit", ""); err != nil {
		return err
	}
	tx.script.commits++
	tx.script.committedWrites += tx.script.stagedWrites
	tx.script.stagedWrites, tx.script.active = 0, false
	return nil
}
func (tx financialSQLTx) Rollback() error {
	if _, err := tx.script.next("rollback", ""); err != nil {
		return err
	}
	tx.script.rollbacks++
	tx.script.stagedWrites, tx.script.active = 0, false
	return nil
}

type financialSQLResult struct{}

func (financialSQLResult) LastInsertId() (int64, error) { return 10, nil }
func (financialSQLResult) RowsAffected() (int64, error) { return 1, nil }

type financialSQLRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *financialSQLRows) Columns() []string { return r.columns }
func (r *financialSQLRows) Close() error      { return nil }
func (r *financialSQLRows) Next(dest []driver.Value) error {
	if r.index == len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

const financialWithdrawID = "01J00000000000000000000911"
const financialRequestKey = "01J00000000000000000000912"
const financialDecisionKey = "01J00000000000000000000913"
const financialWalletID = "01J00000000000000000000914"

func financialWithdrawalStep(paid bool) financialSQLStep {
	status, version := "processing", int64(1)
	var processedBy, processKey, processVersion driver.Value
	reference := ""
	if paid {
		status, version, processedBy, processKey, processVersion, reference = "paid", 2, int64(91), financialDecisionKey, int64(1), "private-payment-reference"
	}
	return financialSQLStep{kind: "query", contains: "FROM `delivery_rider_withdrawals`",
		columns: []string{"id", "withdraw_public_id", "rider_user_id", "wallet_public_id", "amount_cents", "status", "row_version", "reason", "idempotency_key", "processed_by", "process_key", "process_version", "payment_reference"},
		rows:    [][]driver.Value{{int64(7), financialWithdrawID, int64(61), financialWalletID, int64(200), status, version, "", financialRequestKey, processedBy, processKey, processVersion, reference}}}
}

func financialWalletStep() financialSQLStep {
	return financialSQLStep{kind: "query", contains: "FROM `delivery_rider_wallets`",
		columns: []string{"id", "wallet_public_id", "rider_user_id", "available_balance_cents", "row_version"},
		rows:    [][]driver.Value{{int64(5), financialWalletID, int64(61), int64(1000), int64(1)}}}
}

func newFinancialSQLRepository(t *testing.T, steps []financialSQLStep) (*GormRepository, *financialSQLScript) {
	t.Helper()
	script := &financialSQLScript{steps: steps}
	conn := sql.OpenDB(financialSQLConnector{script})
	t.Cleanup(func() { _ = conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	appender := func(tx *gorm.DB, event NotificationEvent) error {
		script.calls++
		var fact RiderWithdrawalNotificationPayload
		if json.Unmarshal(event.Payload, &fact) != nil || event.EventType != EventRiderWithdrawalChanged ||
			fact.RiderUserID != 61 || fact.AmountCents != 200 || !ValidULID(fact.WithdrawPublicID) || event.AggregatePublicID != fact.WithdrawPublicID {
			t.Fatal("invalid event appended from real repository")
		}
		if strings.Contains(string(event.Payload), "private-payment-reference") {
			t.Fatal("payment reference leaked")
		}
		return tx.Exec("INSERT INTO financial_notification_test_events (event_public_id) VALUES (?)", event.EventPublicID).Error
	}
	t.Cleanup(func() {
		if len(script.steps) != 0 || script.unexpected != nil || script.active || script.stagedWrites != 0 {
			t.Errorf("SQL transaction expectations unmet: remaining=%d active=%t staged=%d err=%v", len(script.steps), script.active, script.stagedWrites, script.unexpected)
		}
	})
	return NewRepository(db, WithNotificationEvents(appender)), script
}

func TestRiderWithdrawalNotificationInsertFailureRollsBackRealRepositoryTransaction(t *testing.T) {
	want := errors.New("notification insert unavailable")
	now := time.Now().UTC()
	for _, operation := range []string{"create", "approve"} {
		t.Run(operation, func(t *testing.T) {
			steps := []financialSQLStep{{kind: "begin"}}
			if operation == "create" {
				steps = append(steps, financialWalletStep(), financialSQLStep{kind: "query", contains: "FROM `delivery_rider_withdrawals`", columns: []string{"id"}},
					financialSQLStep{kind: "query", contains: "FROM `delivery_rider_payout_reservations`", columns: []string{"amount"}, rows: [][]driver.Value{{int64(0)}}},
					financialSQLStep{kind: "exec", contains: "UPDATE `delivery_rider_wallets`"}, financialSQLStep{kind: "exec", contains: "INSERT INTO `delivery_rider_withdrawals`"},
					financialSQLStep{kind: "exec", contains: "INSERT INTO `delivery_rider_ledger`"})
			} else {
				steps = append(steps, financialWithdrawalStep(false), financialSQLStep{kind: "exec", contains: "UPDATE `delivery_rider_withdrawals`"})
			}
			steps = append(steps, financialSQLStep{kind: "exec", contains: "INSERT INTO financial_notification_test_events", err: want}, financialSQLStep{kind: "rollback"})
			repo, script := newFinancialSQLRepository(t, steps)
			var err error
			if operation == "create" {
				_, err = repo.CreateRiderWithdrawal(context.Background(), 61, 200, financialRequestKey, now)
			} else {
				_, err = repo.ApproveRiderWithdrawal(context.Background(), financialWithdrawID, 1, 91, financialDecisionKey, "private-payment-reference", now)
			}
			if !errors.Is(err, want) || script.calls != 1 || script.commits != 0 || script.rollbacks != 1 || script.committedWrites != 0 {
				t.Fatalf("partial commit or missing notification failure: calls=%d commits=%d rollbacks=%d writes=%d err=%v", script.calls, script.commits, script.rollbacks, script.committedWrites, err)
			}
		})
	}
}

func TestRiderWithdrawalIdempotentReplayDoesNotAppendNotification(t *testing.T) {
	for _, operation := range []string{"create", "approve"} {
		t.Run(operation, func(t *testing.T) {
			steps := []financialSQLStep{{kind: "begin"}}
			if operation == "create" {
				steps = append(steps, financialWalletStep())
			}
			steps = append(steps, financialWithdrawalStep(operation == "approve"), financialSQLStep{kind: "commit"})
			repo, script := newFinancialSQLRepository(t, steps)
			var value RiderWithdrawal
			var err error
			if operation == "create" {
				value, err = repo.CreateRiderWithdrawal(context.Background(), 61, 200, financialRequestKey, time.Now().UTC())
			} else {
				value, err = repo.ApproveRiderWithdrawal(context.Background(), financialWithdrawID, 1, 91, financialDecisionKey, "private-payment-reference", time.Now().UTC())
			}
			if err != nil || value.WithdrawPublicID != financialWithdrawID || script.calls != 0 || script.commits != 1 || script.rollbacks != 0 || script.committedWrites != 0 {
				t.Fatalf("replay appended or modified money: calls=%d commits=%d writes=%d err=%v", script.calls, script.commits, script.committedWrites, err)
			}
		})
	}
}
