package notification

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This small transactional SQL driver exercises the real GORM adapter without
// network access. It checks affected rows, rollback and persisted states; it
// does not claim MySQL syntax/isolation acceptance.
type eventSQLState struct {
	tables                 map[string][]map[string]driver.Value
	now                    time.Time
	failDeliveryInsert     bool
	failDeliveryUpdate     bool
	failSubscriptionUpdate bool
	queries                []string
}
type eventSQLConnector struct{ state *eventSQLState }

func (c eventSQLConnector) Connect(context.Context) (driver.Conn, error) {
	return &eventSQLConn{state: c.state}, nil
}
func (c eventSQLConnector) Driver() driver.Driver { return eventSQLDriver{} }

type eventSQLDriver struct{}

func (eventSQLDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector required") }

type eventSQLConn struct {
	state   *eventSQLState
	working *eventSQLState
}

func (c *eventSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepared statements unused")
}
func (c *eventSQLConn) Close() error { return nil }
func (c *eventSQLConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *eventSQLConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	if c.working != nil {
		return nil, errors.New("nested transaction")
	}
	c.working = &eventSQLState{tables: map[string][]map[string]driver.Value{}, now: c.state.now, failDeliveryInsert: c.state.failDeliveryInsert, failDeliveryUpdate: c.state.failDeliveryUpdate, failSubscriptionUpdate: c.state.failSubscriptionUpdate}
	for table, rows := range c.state.tables {
		for _, row := range rows {
			copyRow := map[string]driver.Value{}
			for k, v := range row {
				copyRow[k] = v
			}
			c.working.tables[table] = append(c.working.tables[table], copyRow)
		}
	}
	return eventSQLTx{c}, nil
}

type eventSQLTx struct{ conn *eventSQLConn }

func (tx eventSQLTx) Commit() error {
	tx.conn.state.tables = tx.conn.working.tables
	tx.conn.working = nil
	return nil
}
func (tx eventSQLTx) Rollback() error { tx.conn.working = nil; return nil }
func (c *eventSQLConn) active() *eventSQLState {
	if c.working != nil {
		return c.working
	}
	return c.state
}

type eventSQLResult struct{ id, count int64 }

func (r eventSQLResult) LastInsertId() (int64, error) { return r.id, nil }
func (r eventSQLResult) RowsAffected() (int64, error) { return r.count, nil }

var eventInsertPattern = regexp.MustCompile("^INSERT INTO `([^`]+)` \\(([^)]+)\\)")
var eventUpdatePattern = regexp.MustCompile("^UPDATE `([^`]+)` SET (.+) WHERE (.+)$")
var eventFromPattern = regexp.MustCompile(" FROM `([^`]+)`")
var eventConditionPattern = regexp.MustCompile("(RIGHT\\(authorization_nonce,32\\)|`?[a-z_]+`?) (=|>|<>) \\?")
var eventLiteralCondition = regexp.MustCompile("([a-z_]+) = ([0-9]+)")

func (c *eventSQLConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	s := c.active()
	c.state.queries = append(c.state.queries, query)
	if match := eventInsertPattern.FindStringSubmatch(query); len(match) > 0 {
		table := match[1]
		if table == "delivery_notification_deliveries" && s.failDeliveryInsert {
			return nil, errors.New("simulated delivery insert failure")
		}
		columns := strings.Split(match[2], ",")
		row := map[string]driver.Value{"id": int64(len(s.tables[table]) + 1)}
		for i, name := range columns {
			row[strings.Trim(name, "`")] = args[i].Value
		}
		for _, existing := range s.tables[table] {
			keys := []string{"event_public_id", "recipient_user_id", "purpose"}
			if table == "delivery_notification_deliveries" {
				keys = append(keys, "channel")
			}
			match := true
			for _, key := range keys {
				if fmt.Sprint(existing[key]) != fmt.Sprint(row[key]) {
					match = false
				}
			}
			if match {
				return nil, &mysqlDriver.MySQLError{Number: 1062, Message: "duplicate coordinate"}
			}
		}
		s.tables[table] = append(s.tables[table], row)
		return eventSQLResult{id: row["id"].(int64), count: 1}, nil
	}
	if match := eventUpdatePattern.FindStringSubmatch(query); len(match) > 0 {
		table, assignment, where := match[1], match[2], match[3]
		if table == "delivery_notification_deliveries" && s.failDeliveryUpdate {
			return nil, errors.New("simulated delivery update failure")
		}
		if table == "notification_subscriptions" && s.failSubscriptionUpdate {
			return nil, errors.New("simulated subscription update failure")
		}
		if table == "delivery_notification_events" && strings.Contains(assignment, "`last_error`") {
			return nil, errors.New("events has no last_error column")
		}
		setArgs := strings.Count(assignment, "?")
		var count int64
		for _, row := range s.tables[table] {
			if !eventSQLMatches(row, where, args[setArgs:], s.now) {
				continue
			}
			index := 0
			for _, set := range strings.Split(assignment, ",") {
				parts := strings.SplitN(set, "=", 2)
				key := strings.TrimSpace(strings.Trim(parts[0], "` "))
				if strings.Contains(parts[1], "?") {
					row[key] = args[index].Value
					index++
				} else if strings.Contains(parts[1], "+ 1") {
					n, _ := strconv.ParseInt(fmt.Sprint(row[key]), 10, 64)
					row[key] = n + 1
				} else {
					return nil, fmt.Errorf("unexpected assignment %s", set)
				}
			}
			count++
		}
		return eventSQLResult{count: count}, nil
	}
	return nil, fmt.Errorf("unexpected exec %s", query)
}

func eventSQLMatches(row map[string]driver.Value, where string, args []driver.NamedValue, now time.Time) bool {
	for i, match := range eventConditionPattern.FindAllStringSubmatch(where, -1) {
		value := args[i].Value
		column := strings.Trim(match[1], "`")
		if column == "RIGHT(authorization_nonce,32)" {
			if !strings.HasSuffix(fmt.Sprint(row["authorization_nonce"]), fmt.Sprint(value)) {
				return false
			}
			continue
		}
		if match[2] == "<>" {
			if fmt.Sprint(row[column]) == fmt.Sprint(value) {
				return false
			}
			continue
		}
		if match[2] == "=" {
			if fmt.Sprint(row[column]) != fmt.Sprint(value) {
				return false
			}
		} else {
			if column == "expires_at" && row[column] == nil && strings.Contains(where, "expires_at IS NULL OR expires_at > ?") {
				continue
			}
			left, ok := row[column].(time.Time)
			right, ok2 := value.(time.Time)
			if !ok || !ok2 || !left.After(right) {
				return false
			}
		}
	}
	for _, match := range eventLiteralCondition.FindAllStringSubmatch(where, -1) {
		if fmt.Sprint(row[match[1]]) != match[2] {
			return false
		}
	}
	if strings.Contains(where, "reservation_nonce IS NULL") && row["reservation_nonce"] != nil {
		return false
	}
	if strings.Contains(where, "lease_expires_at > UTC_TIMESTAMP(3)") {
		at, ok := row["lease_expires_at"].(time.Time)
		if !ok || !at.After(now) {
			return false
		}
	}
	if strings.Contains(where, "expires_at IS NULL OR expires_at > UTC_TIMESTAMP(3)") && row["expires_at"] != nil {
		at, ok := row["expires_at"].(time.Time)
		if !ok || !at.After(now) {
			return false
		}
	}
	return true
}

func (c *eventSQLConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	match := eventFromPattern.FindStringSubmatch(query)
	if len(match) == 0 {
		return nil, fmt.Errorf("unexpected query %s", query)
	}
	s := c.active()
	c.state.queries = append(c.state.queries, query)
	rows := s.tables[match[1]]
	found := []map[string]driver.Value{}
	for _, row := range rows {
		if eventSQLMatches(row, query, args, s.now) {
			found = append(found, row)
		}
	}
	columns := []string{}
	if len(found) > 0 {
		for key := range found[0] {
			columns = append(columns, key)
		}
	}
	sort.Strings(columns)
	values := [][]driver.Value{}
	for _, row := range found {
		item := []driver.Value{}
		for _, column := range columns {
			item = append(item, row[column])
		}
		values = append(values, item)
	}
	return &eventSQLRows{columns: columns, values: values}, nil
}

type eventSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *eventSQLRows) Columns() []string { return r.columns }
func (r *eventSQLRows) Close() error      { return nil }
func (r *eventSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func newEventSQLRepository(t *testing.T, lease EventLease) (*GormEventRepository, *eventSQLState) {
	t.Helper()
	now := time.Now().UTC()
	state := &eventSQLState{now: now, tables: map[string][]map[string]driver.Value{"delivery_notification_events": {{"id": int64(lease.Event.ID), "status": string(EventProcessing), "row_version": int64(lease.Event.RowVersion), "lease_token": lease.LeaseToken, "lease_expires_at": *lease.Event.LeaseExpiresAt, "expires_at": nil}}}}
	conn := sql.OpenDB(eventSQLConnector{state: state})
	t.Cleanup(func() { conn.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	return NewEventRepository(db), state
}

func expansionTestRows(t *testing.T, lease EventLease) ([]InboxItem, []DeliveryItem) {
	t.Helper()
	r := Recipient{UserID: 9, Purpose: PurposeOrder, Title: "订单更新", Summary: "请查看最新进度", AggregateType: lease.Event.AggregateType, AggregatePublicID: lease.Event.AggregatePublicID, AggregateVersion: lease.Event.AggregateVersion, TargetPath: "/pages/merchant-order-detail/index"}
	inbox, deliveries, err := (&Expander{}).build(lease.Event, []Recipient{r}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return inbox, deliveries
}

func TestEventRepositoryAtomicallyDeliversInboxAndPreservesReadOnReplay(t *testing.T) {
	lease := testLease(t)
	repo, state := newEventSQLRepository(t, lease)
	inbox, deliveries := expansionTestRows(t, lease)
	if err := repo.ExpandAndComplete(context.Background(), lease, inbox, deliveries, EventExpanded, state.now); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"delivery_notification_inbox", "delivery_notification_deliveries"} {
		if len(state.tables[table]) != 1 || state.tables[table][0]["status"] != string(DeliveryDelivered) {
			t.Fatalf("not delivered: %s %+v", table, state.tables[table])
		}
	}
	row := state.tables["delivery_notification_inbox"][0]
	readAt := state.now
	row["status"], row["read_at"] = string(DeliveryRead), readAt
	event := state.tables["delivery_notification_events"][0]
	event["status"], event["row_version"], event["lease_token"], event["lease_expires_at"] = string(EventProcessing), int64(lease.Event.RowVersion), lease.LeaseToken, *lease.Event.LeaseExpiresAt
	if err := repo.ExpandAndComplete(context.Background(), lease, inbox, deliveries, EventExpanded, state.now); err != nil {
		t.Fatal(err)
	}
	row = state.tables["delivery_notification_inbox"][0]
	if row["status"] != string(DeliveryRead) || row["read_at"] != readAt {
		t.Fatalf("read rolled back: %+v", row)
	}
}

func TestEventRepositoryRollsBackChannelRowsWhenCommitFails(t *testing.T) {
	lease := testLease(t)
	repo, state := newEventSQLRepository(t, lease)
	inbox, deliveries := expansionTestRows(t, lease)
	state.failDeliveryInsert = true
	if err := repo.ExpandAndComplete(context.Background(), lease, inbox, deliveries, EventExpanded, state.now); err == nil {
		t.Fatal("expected insertion failure")
	}
	if len(state.tables["delivery_notification_inbox"]) != 0 || state.tables["delivery_notification_events"][0]["status"] != string(EventProcessing) {
		t.Fatalf("partial commit: %+v", state.tables)
	}
}

func TestEventRepositoryDatabaseLeaseFenceRollsBackStaleExpansion(t *testing.T) {
	lease := testLease(t)
	repo, state := newEventSQLRepository(t, lease)
	inbox, deliveries := expansionTestRows(t, lease)
	state.now = lease.Event.LeaseExpiresAt.Add(time.Second)
	// Client timestamp is stale but the database's independent clock fences it.
	if err := repo.ExpandAndComplete(context.Background(), lease, inbox, deliveries, EventExpanded, time.Now().UTC()); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("err=%v", err)
	}
	if len(state.tables["delivery_notification_inbox"]) != 0 {
		t.Fatal("stale lease created visible inbox")
	}
}

func TestEventRepositoryFailureStoresOnlyCodeAndAppliesRetryLimit(t *testing.T) {
	lease := testLease(t)
	repo, state := newEventSQLRepository(t, lease)
	status, err := repo.FailEvent(context.Background(), lease, "recipient_resolve_failed", "credential-must-not-persist", state.now.Add(time.Minute), 1, state.now)
	if err != nil || status != EventDeadletter {
		t.Fatalf("status=%s err=%v", status, err)
	}
	row := state.tables["delivery_notification_events"][0]
	if row["last_error_code"] != "recipient_resolve_failed" || row["last_error"] != nil || row["next_attempt_at"] != nil {
		t.Fatalf("bad failure row %+v", row)
	}
}
