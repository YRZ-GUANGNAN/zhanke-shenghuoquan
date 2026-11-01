package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var riderMoneyDatabaseSequence uint64

func TestRiderPayoutQueueAndWithdrawalMigrationsAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedRiderMoneyMySQL(t)
	applyD1SQLFile(t, db, "../../../migrations/028_delivery_fee_and_rider_wallet.sql")
	applyD1SQLFile(t, db, "../../../migrations/050_delivery_rider_payout_queue.sql")
	applyD1SQLFile(t, db, "../../../migrations/051_delivery_rider_withdrawals.sql")

	for _, table := range []string{
		"delivery_rider_wallets",
		"delivery_rider_ledger",
		"delivery_rider_payout_queue",
		"delivery_rider_payout_reservations",
		"delivery_rider_payout_events",
		"delivery_rider_withdrawals",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("migration left required table %s missing", table)
		}
	}

	assertRiderMoneyIndex(t, db, "delivery_rider_payout_queue", "uq_delivery_rider_payout_queue_earning")
	assertRiderMoneyIndex(t, db, "delivery_rider_payout_reservations", "uq_delivery_rider_payout_reservation_earning")
	assertRiderMoneyIndex(t, db, "delivery_rider_withdrawals", "uq_delivery_rider_withdrawal_idempotency")
	t.Log("migration_028_050_051_apply=PASS isolated_database=true")
}

func TestRiderWithdrawalConcurrentReservationAndSingleReleaseAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedRiderMoneyMySQL(t)
	applyD1SQLFile(t, db, "../../../migrations/028_delivery_fee_and_rider_wallet.sql")
	applyD1SQLFile(t, db, "../../../migrations/050_delivery_rider_payout_queue.sql")
	applyD1SQLFile(t, db, "../../../migrations/051_delivery_rider_withdrawals.sql")

	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	walletID, err := NewULID()
	if err != nil {
		t.Fatal(err)
	}
	const riderID = uint64(995101)
	if err := db.Create(&RiderWallet{
		WalletPublicID: walletID, RiderUserID: riderID, AvailableBalanceCents: 1000,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	keys := []string{mustRiderMoneyULID(t), mustRiderMoneyULID(t)}
	repository := NewRepository(db)
	type result struct {
		value RiderWithdrawal
		err   error
	}
	results := make(chan result, len(keys))
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, key := range keys {
		key := key
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			value, createErr := repository.CreateRiderWithdrawal(context.Background(), riderID, 700, key, now)
			results <- result{value: value, err: createErr}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	var created RiderWithdrawal
	successes, insufficient := 0, 0
	for value := range results {
		switch {
		case value.err == nil:
			successes++
			created = value.value
		case errors.Is(value.err, ErrWalletInsufficient):
			insufficient++
		default:
			t.Fatalf("concurrent withdrawal returned unexpected error: %v", value.err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent withdrawals successes=%d insufficient=%d, want 1/1", successes, insufficient)
	}
	assertRiderMoneyBalance(t, db, riderID, 300)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawals", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 1)

	operatorID := uint64(995901)
	decisionKey := mustRiderMoneyULID(t)
	rejected, err := repository.RejectRiderWithdrawal(
		context.Background(), created.WithdrawPublicID, created.RowVersion,
		operatorID, decisionKey, "未实际打款，退回余额", now.Add(time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != RiderWithdrawalRejected {
		t.Fatalf("rejected status=%s", rejected.Status)
	}
	assertRiderMoneyBalance(t, db, riderID, 1000)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 2)

	if _, err := repository.RejectRiderWithdrawal(
		context.Background(), created.WithdrawPublicID, created.RowVersion,
		operatorID, decisionKey, "未实际打款，退回余额", now.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("exact rejection replay failed: %v", err)
	}
	assertRiderMoneyBalance(t, db, riderID, 1000)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 2)
	t.Log("rider_withdrawal_concurrency=PASS balance_reservation=true single_release=true")
}

func openIsolatedRiderMoneyMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("DELIVERY_PAYOUT_QUEUE_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("run node scripts/test-rider-money-mysql.cjs for the dedicated isolated MySQL server")
	}
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid isolated rider-money DSN")
	}
	if cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:33318" ||
		!regexp.MustCompile(`^campus_rider_money_runner_[0-9]+$`).MatchString(cfg.DBName) ||
		cfg.User != "root" || cfg.Passwd != "" {
		t.Fatal("requires scripts/test-rider-money-mysql.cjs isolated runner; other servers/databases are forbidden")
	}
	expectedDataDir := os.Getenv("DELIVERY_RIDER_MONEY_TEST_DATADIR")
	if expectedDataDir == "" || !strings.HasPrefix(filepath.Base(expectedDataDir), "campus-rider-money-mysql-") {
		t.Fatal("missing isolated runner data directory")
	}
	// Never use the database named by the caller. The harness must create and
	// later destroy its own isolated database or fail closed.
	cfg.DBName = ""
	cfg.MultiStatements = false
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 3 * time.Second
	cfg.ReadTimeout = 15 * time.Second
	cfg.WriteTimeout = 15 * time.Second
	server, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Ping(); err != nil {
		server.Close()
		t.Fatalf("connect isolated MySQL server: %v", err)
	}
	var actualDataDir string
	if err := server.QueryRow("SELECT @@datadir").Scan(&actualDataDir); err != nil {
		server.Close()
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(actualDataDir), filepath.Clean(expectedDataDir)) {
		server.Close()
		t.Fatal("server data directory does not match the isolated runner")
	}
	var version string
	if err := server.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		server.Close()
		t.Fatal(err)
	}
	if !supportsEnforcedCheckConstraints(version) {
		server.Close()
		t.Fatalf("MySQL 8.0.16+ or 8.4+ required, got %s", version)
	}

	sequence := atomic.AddUint64(&riderMoneyDatabaseSequence, 1)
	databaseName := fmt.Sprintf("delivery_rider_money_%d_%d", time.Now().UnixNano(), sequence)
	if _, err := server.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); err != nil {
		server.Close()
		t.Fatalf("create independent rider-money database (in-place fallback is forbidden): %v", err)
	}
	cfg.DBName = databaseName
	database, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		_, _ = server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		server.Close()
		t.Fatal(err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		_, _ = server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		server.Close()
		t.Fatal(err)
	}
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: database, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		database.Close()
		_, _ = server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close isolated rider-money database: %v", err)
		}
		if _, err := server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop isolated rider-money database: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Errorf("close isolated rider-money server connection: %v", err)
		}
	})
	t.Logf("isolated_mysql_version=%s database=%s", version, databaseName)
	return db
}

func supportsEnforcedCheckConstraints(version string) bool {
	match := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)`).FindStringSubmatch(version)
	if len(match) != 4 {
		return false
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	return major == 8 && (minor > 0 || patch >= 16)
}

func TestSupportsEnforcedCheckConstraints(t *testing.T) {
	for version, expected := range map[string]bool{
		"8.0.15":                    false,
		"8.0.16":                    true,
		"8.0.46-0ubuntu0.24.04.4":   true,
		"8.4.3":                     true,
		"10.11.6-MariaDB-0+deb12u1": false,
		"unknown":                   false,
	} {
		if actual := supportsEnforcedCheckConstraints(version); actual != expected {
			t.Errorf("supportsEnforcedCheckConstraints(%q)=%t want=%t", version, actual, expected)
		}
	}
}

func assertRiderMoneyIndex(t *testing.T, db *gorm.DB, table, index string) {
	t.Helper()
	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("missing index %s.%s", table, index)
	}
}

func assertRiderMoneyBalance(t *testing.T, db *gorm.DB, riderUserID, expected uint64) {
	t.Helper()
	var wallet RiderWallet
	if err := db.Where("rider_user_id = ?", riderUserID).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableBalanceCents != expected {
		t.Fatalf("rider balance=%d want=%d", wallet.AvailableBalanceCents, expected)
	}
}

func assertRiderMoneyCount(t *testing.T, db *gorm.DB, table string, expected int64) {
	t.Helper()
	var count int64
	if err := db.Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("%s rows=%d want=%d", table, count, expected)
	}
}

func mustRiderMoneyULID(t *testing.T) string {
	t.Helper()
	value, err := NewULID()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func riderMoneySchema(t *testing.T) *gorm.DB {
	t.Helper()
	db := openIsolatedRiderMoneyMySQL(t)
	for _, name := range []string{
		"023_delivery_order.sql", "024_delivery_dispatch.sql", "025_delivery_payment_wallet.sql",
		"028_delivery_fee_and_rider_wallet.sql", "045_delivery_order_source.sql",
		"048_delivery_payment_recovery.sql", "049_delivery_funding_sources.sql",
		"050_delivery_rider_payout_queue.sql", "051_delivery_rider_withdrawals.sql",
		"052_delivery_recharge_cancellation.sql",
	} {
		applyD1SQLFile(t, db, "../../../migrations/"+name)
	}
	return db
}

func seedRiderMoneyWallet(t *testing.T, db *gorm.DB, riderID, amount uint64, now time.Time) RiderWallet {
	t.Helper()
	wallet := RiderWallet{WalletPublicID: mustRiderMoneyULID(t), RiderUserID: riderID,
		AvailableBalanceCents: amount, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	return wallet
}

// These are local test facts, not payment gateway responses or real transfers.
func seedRiderMoneyEarning(t *testing.T, db *gorm.DB, riderID, amount uint64, now time.Time) RiderEarningRecord {
	t.Helper()
	orderID := mustRiderMoneyULID(t)
	order := Order{DeliveryOrderPublicID: orderID, MerchantUserID: 995201, RiderUserID: &riderID,
		SourcePlatform: SourceLocal, DeliveryFeeCents: amount + 50, RiderFeeCents: amount, PlatformCutCents: 50,
		Status: StatusDelivered, RowVersion: 1, CreatedAt: now, UpdatedAt: now, DeliveredAt: &now}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	paymentID := mustRiderMoneyULID(t)
	transactionID := "isolated-test-" + paymentID
	attempt := PaymentAttempt{PaymentPublicID: paymentID, MerchantUserID: order.MerchantUserID,
		Purpose: PaymentOrderFee, DeliveryOrderPublicID: &orderID, Provider: PaymentWechat,
		Status: PaymentPaid, ProviderStatus: "SUCCESS", AmountCents: order.DeliveryFeeCents,
		Currency: "CNY", ClientRequestID: mustRiderMoneyULID(t), OutTradeNo: paymentID,
		WechatTransactionID: &transactionID, PaidAt: &now, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	settlement := FeeSettlement{SettlementPublicID: mustRiderMoneyULID(t), DeliveryOrderPublicID: orderID,
		MerchantUserID: order.MerchantUserID, Source: SettlementWechat, AmountCents: order.DeliveryFeeCents,
		PaymentPublicID: &paymentID, Status: SettlementPaid, PaidAt: now, RowVersion: 1}
	if err := db.Create(&settlement).Error; err != nil {
		t.Fatal(err)
	}
	earning := RiderEarningRecord{EarningPublicID: mustRiderMoneyULID(t), DeliveryOrderPublicID: orderID,
		RiderUserID: riderID, DeliveryFeeCents: order.DeliveryFeeCents, RiderFeeCents: amount,
		PlatformCutCents: 50, SettlementStatus: RiderEarningPending, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&earning).Error; err != nil {
		t.Fatal(err)
	}
	return earning
}

func TestRiderQueueConcurrentDedupCancelAndWithdrawalReservationAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	const riderID = uint64(995102)
	seedRiderMoneyWallet(t, db, riderID, 1000, now)
	earning := seedRiderMoneyEarning(t, db, riderID, 800, now)
	repository := NewRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	type outcome struct {
		value RiderPayoutQueueResult
		err   error
	}
	start := make(chan struct{})
	results := make(chan outcome, 8)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			value, err := repository.ReconcileRiderPayoutQueue(ctx, earning.EarningPublicID, now)
			results <- outcome{value, err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	var queued RiderPayoutQueueResult
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.value.Queue.Status != RiderPayoutQueueQueued || result.value.Reservation == nil {
			t.Fatalf("expected queued reservation: %+v", result.value)
		}
		queued = result.value
	}
	assertRiderMoneyCount(t, db, "delivery_rider_payout_queue", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_payout_reservations", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_payout_events", 1)
	assertRiderMoneyBalance(t, db, riderID, 1000)
	summary, err := repository.ReadRiderPayoutSummary(ctx, riderID)
	if err != nil || summary.ReservedCents != 800 {
		t.Fatalf("reserved=%+v err=%v", summary, err)
	}
	if _, err := repository.CreateRiderWithdrawal(ctx, riderID, 201, mustRiderMoneyULID(t), now); !errors.Is(err, ErrWalletInsufficient) {
		t.Fatalf("withdrawal spent reserved money: %v", err)
	}
	if _, err := repository.MarkRiderEarningPaidOffline(ctx, earning.EarningPublicID, 1, 995901, mustRiderMoneyULID(t), "test", now); !errors.Is(err, ErrRiderPayoutReserved) {
		t.Fatalf("offline payout bypassed own reservation: %v", err)
	}
	withdrawKey := mustRiderMoneyULID(t)
	withdrawal, err := repository.CreateRiderWithdrawal(ctx, riderID, 200, withdrawKey, now)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repository.CreateRiderWithdrawal(ctx, riderID, 200, withdrawKey, now)
	if err != nil || replay.WithdrawPublicID != withdrawal.WithdrawPublicID {
		t.Fatalf("withdrawal replay: %+v %v", replay, err)
	}
	if _, err := repository.CreateRiderWithdrawal(ctx, riderID, 199, withdrawKey, now); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed withdrawal accepted: %v", err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 1)
	assertRiderMoneyBalance(t, db, riderID, 800)

	cancelKey := mustRiderMoneyULID(t)
	cancelled, err := repository.CancelRiderPayoutQueue(ctx, queued.Queue.PayoutQueuePublicID, 1, 995901, cancelKey, "isolated test", now)
	if err != nil || cancelled.Reservation == nil || cancelled.Reservation.Status != RiderPayoutReservationReleased {
		t.Fatalf("cancel: %+v %v", cancelled, err)
	}
	if _, err := repository.CancelRiderPayoutQueue(ctx, queued.Queue.PayoutQueuePublicID, 1, 995901, cancelKey, "isolated test", now); err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_payout_events", 2)
	summary, err = repository.ReadRiderPayoutSummary(ctx, riderID)
	if err != nil || summary.ReservedCents != 0 {
		t.Fatalf("released=%+v err=%v", summary, err)
	}
	assertRiderMoneyBalance(t, db, riderID, 800)
	reconciled, err := repository.ReconcileRiderPayoutQueue(ctx, earning.EarningPublicID, now)
	if err != nil || reconciled.Queue.Status != RiderPayoutQueueCancelled {
		t.Fatalf("cancelled queue was requeued: %+v %v", reconciled, err)
	}
	t.Log("queue_8_worker_dedup=PASS withdrawal_reserved_funds=PASS cancel_release_once=PASS")
}

func TestRiderOfflinePayoutCannotSpendOtherReservationAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	const riderID = uint64(995103)
	seedRiderMoneyWallet(t, db, riderID, 1000, now)
	held := seedRiderMoneyEarning(t, db, riderID, 800, now)
	unheld := seedRiderMoneyEarning(t, db, riderID, 700, now)
	repository := NewRepository(db)
	if _, err := repository.ReconcileRiderPayoutQueue(context.Background(), held.EarningPublicID, now); err != nil {
		t.Fatal(err)
	}
	_, err := repository.MarkRiderEarningPaidOffline(context.Background(), unheld.EarningPublicID, 1, 995901, mustRiderMoneyULID(t), "isolated test", now)
	if !errors.Is(err, ErrRiderPayoutReserved) {
		t.Fatalf("offline payout must not consume another earning's reservation; got %v", err)
	}
	assertRiderMoneyBalance(t, db, riderID, 1000)
	assertRiderMoneyCount(t, db, "delivery_rider_payout_records", 0)
}

func TestRiderWithdrawalProcessedCheckRejectsNullOperatorAndVersionAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	operatorID, version := uint64(995901), uint64(1)
	for _, status := range []RiderWithdrawalStatus{RiderWithdrawalPaid, RiderWithdrawalRejected} {
		for _, field := range []string{"processed_by", "process_version"} {
			t.Run(string(status)+"/"+field, func(t *testing.T) {
				key := mustRiderMoneyULID(t)
				value := RiderWithdrawal{WithdrawPublicID: mustRiderMoneyULID(t), RiderUserID: 995104,
					WalletPublicID: mustRiderMoneyULID(t), AmountCents: 100, Currency: "CNY", Status: status,
					Reason: "isolated test", IdempotencyKey: mustRiderMoneyULID(t), RowVersion: 2,
					ProcessedBy: &operatorID, ProcessKey: &key, ProcessVersion: &version,
					PaymentReference: "isolated-test-reference", ProcessedAt: &now, CreatedAt: now, UpdatedAt: now}
				if field == "processed_by" {
					value.ProcessedBy = nil
				} else {
					value.ProcessVersion = nil
				}
				err := db.Create(&value).Error
				var mysqlError *mysqlDriver.MySQLError
				if !errors.As(err, &mysqlError) || mysqlError.Number != 3819 {
					t.Fatalf("processed CHECK must reject NULL %s: %v", field, err)
				}
			})
		}
	}
}

func TestRiderMoneyEmptyRollbackAndFactGuardsAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	for _, migration := range []string{"051_delivery_rider_withdrawals.sql", "050_delivery_rider_payout_queue.sql"} {
		if err := executeRiderMoneySQL(db, "../../../migrations/rollback/"+migration); err != nil {
			t.Fatal(err)
		}
		applyD1SQLFile(t, db, "../../../migrations/"+migration)
	}
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	const riderID = uint64(995105)
	seedRiderMoneyWallet(t, db, riderID, 1000, now)
	earning := seedRiderMoneyEarning(t, db, riderID, 800, now)
	repository := NewRepository(db)
	if _, err := repository.ReconcileRiderPayoutQueue(context.Background(), earning.EarningPublicID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateRiderWithdrawal(context.Background(), riderID, 100, mustRiderMoneyULID(t), now); err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{"051_delivery_rider_withdrawals.sql", "050_delivery_rider_payout_queue.sql"} {
		err := executeRiderMoneySQL(db, "../../../migrations/rollback/"+migration)
		var mysqlError *mysqlDriver.MySQLError
		if !errors.As(err, &mysqlError) || mysqlError.Number != 1644 || !strings.Contains(mysqlError.Message, "rollback blocked") {
			t.Fatalf("expected rollback fact guard %s, got %v", migration, err)
		}
	}
	assertRiderMoneyCount(t, db, "delivery_rider_payout_queue", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_payout_reservations", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_payout_events", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawals", 1)
	assertRiderMoneyBalance(t, db, riderID, 900)
	t.Log("empty_rollback_reapply=PASS durable_fact_rollback_refused=PASS")
}

func TestRiderQueueAndWithdrawalRaceConservesBalanceAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	for i := uint64(0); i < 5; i++ {
		riderID := uint64(995110) + i
		seedRiderMoneyWallet(t, db, riderID, 1000, now)
		earning := seedRiderMoneyEarning(t, db, riderID, 700, now)
		withdrawKey := mustRiderMoneyULID(t)
		start := make(chan struct{})
		var queued RiderPayoutQueueResult
		var queueErr, withdrawErr error
		var group sync.WaitGroup
		group.Add(2)
		go func() {
			defer group.Done()
			<-start
			queued, queueErr = repository.ReconcileRiderPayoutQueue(context.Background(), earning.EarningPublicID, now)
		}()
		go func() {
			defer group.Done()
			<-start
			_, withdrawErr = repository.CreateRiderWithdrawal(context.Background(), riderID, 700, withdrawKey, now)
		}()
		close(start)
		group.Wait()
		if queueErr != nil {
			t.Fatal(queueErr)
		}
		summary, err := repository.ReadRiderPayoutSummary(context.Background(), riderID)
		if err != nil {
			t.Fatal(err)
		}
		switch queued.Queue.Status {
		case RiderPayoutQueueQueued:
			if !errors.Is(withdrawErr, ErrWalletInsufficient) || summary.ReservedCents != 700 {
				t.Fatalf("queue winner allowed withdrawal: err=%v summary=%+v", withdrawErr, summary)
			}
			assertRiderMoneyBalance(t, db, riderID, 1000)
		case RiderPayoutQueueBlocked:
			if withdrawErr != nil || queued.Queue.ReasonCode != "balance_insufficient" || summary.ReservedCents != 0 {
				t.Fatalf("withdrawal winner inconsistent: err=%v queue=%+v summary=%+v", withdrawErr, queued.Queue, summary)
			}
			assertRiderMoneyBalance(t, db, riderID, 300)
		default:
			t.Fatalf("unexpected race status=%s", queued.Queue.Status)
		}
	}
	t.Log("queue_withdrawal_race_5_runs=PASS reservations_plus_debits_never_exceed_balance=true")
}

func TestRiderWithdrawalSameKeyAndConcurrentRejectionAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	const riderID = uint64(995120)
	seedRiderMoneyWallet(t, db, riderID, 1000, now)
	repository := NewRepository(db)
	key := mustRiderMoneyULID(t)
	start := make(chan struct{})
	type result struct {
		value RiderWithdrawal
		err   error
	}
	results := make(chan result, 8)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			value, err := repository.CreateRiderWithdrawal(context.Background(), riderID, 700, key, now)
			results <- result{value, err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	var withdrawal RiderWithdrawal
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		if withdrawal.WithdrawPublicID != "" && withdrawal.WithdrawPublicID != result.value.WithdrawPublicID {
			t.Fatal("same-key request created different withdrawal")
		}
		withdrawal = result.value
	}
	assertRiderMoneyBalance(t, db, riderID, 300)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 1)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawals", 1)
	decisionKey := mustRiderMoneyULID(t)
	start = make(chan struct{})
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := repository.RejectRiderWithdrawal(context.Background(), withdrawal.WithdrawPublicID, 1, 995901, decisionKey, "isolated duplicate rejection", now)
			errs <- err
		}()
	}
	close(start)
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertRiderMoneyBalance(t, db, riderID, 1000)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 2)
	t.Log("withdrawal_same_key_8_workers=PASS rejection_same_key_8_workers=PASS balance_released_once=true")
}

func executeRiderMoneySQL(db *gorm.DB, filename string) error {
	source, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	delimiter := ";"
	var statement strings.Builder
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.HasPrefix(trimmed, "DELIMITER ") {
			if strings.TrimSpace(statement.String()) != "" {
				return errors.New("delimiter changed in statement")
			}
			delimiter = strings.TrimSpace(strings.TrimPrefix(trimmed, "DELIMITER "))
			continue
		}
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(trimmed, delimiter) {
			continue
		}
		query := strings.TrimSuffix(strings.TrimSpace(statement.String()), delimiter)
		statement.Reset()
		if err := db.Exec(query).Error; err != nil {
			return err
		}
	}
	if strings.TrimSpace(statement.String()) != "" {
		return errors.New("unterminated migration statement")
	}
	return nil
}
