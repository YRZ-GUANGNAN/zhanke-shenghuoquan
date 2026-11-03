package notification

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

// A stale application timestamp must not extend a database lease. Both
// supported storage clocks must reject the write at database execution time.
func TestNotificationStorageClockAgainstIsolatedMySQL(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	for _, clock := range []struct {
		name string
		loc  *time.Location
		zone string
	}{{"UTC", time.UTC, "+00:00"}, {"Shanghai", shanghai, "+08:00"}} {
		t.Run(clock.name, func(t *testing.T) {
			db := openIsolatedNotificationMySQLWithTimezone(t, clock.loc, clock.zone)
			pool, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			// Hold distinct physical connections so a one-off SET on only the
			// first connection cannot accidentally satisfy this contract.
			connections := make([]*sql.Conn, 0, 3)
			for i := 0; i < 3; i++ {
				connection, err := pool.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				connections = append(connections, connection)
				t.Cleanup(func() { _ = connection.Close() })
				var zone string
				var delta int64
				if err := connection.QueryRowContext(context.Background(), "SELECT @@session.time_zone, TIMESTAMPDIFF(SECOND, ?, CURRENT_TIMESTAMP(3))", time.Now().UTC()).Scan(&zone, &delta); err != nil || zone != clock.zone || delta < -3 || delta > 3 {
					t.Fatalf("pooled connection storage clock mismatch zone=%q delta=%d err=%v", zone, delta, err)
				}
			}
			for _, connection := range connections {
				_ = connection.Close()
			}
			repo, events := NewRepository(db), NewEventRepository(db)
			staleNow := time.Now().UTC().Add(-time.Hour)
			event := isolatedNotificationEvent(t)
			if err := repo.AppendEventTx(db, event); err != nil {
				t.Fatal(err)
			}
			lease, found, err := events.ClaimNextEvent(context.Background(), staleNow, time.Minute)
			if err != nil || !found {
				t.Fatalf("stale event claim found=%v err=%v", found, err)
			}
			if err := events.ExpandAndComplete(context.Background(), lease, nil, nil, EventSkipped, staleNow); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("database-expired event lease accepted with stale now: %v", err)
			}
			if _, err := events.FailEvent(context.Background(), lease, "test_failure", "", staleNow.Add(time.Minute), 3, staleNow); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("database-expired event failure accepted with stale now: %v", err)
			}

			// Event expiry must use the storage clock even while its lease is live.
			liveNow := time.Now().UTC()
			lease, found, err = events.ClaimNextEvent(context.Background(), liveNow, time.Minute)
			if err != nil || !found {
				t.Fatalf("live event claim found=%v err=%v", found, err)
			}
			if err := db.Model(&Event{}).Where("id=?", lease.Event.ID).Update("expires_at", liveNow.Add(-time.Minute)).Error; err != nil {
				t.Fatal(err)
			}
			if err := events.ExpandAndComplete(context.Background(), lease, nil, nil, EventExpanded, liveNow); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("database-expired event expanded: %v", err)
			}
			if err := events.ExpandAndComplete(context.Background(), lease, nil, nil, EventSkipped, time.Now().UTC()); err != nil {
				t.Fatalf("live lease cannot skip expired event: %v", err)
			}

			subscription := isolatedNotificationSubscription(t, db)
			item := isolatedNotificationDelivery(t, db)
			delivery, found, err := repo.ClaimWechatDelivery(context.Background(), staleNow, time.Minute)
			if err != nil || !found {
				t.Fatalf("stale delivery claim found=%v err=%v", found, err)
			}
			if _, err := repo.ReserveForDelivery(context.Background(), delivery, "isolated-app", "isolated-openid", "isolated-template", staleNow); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("database-expired subscribe lease reserved quota with stale now: %v", err)
			}
			var quota Subscription
			if err := db.First(&quota, subscription.ID).Error; err != nil || quota.RemainingQuota != 1 || quota.ReservedQuota != 0 {
				t.Fatalf("rejected stale lease changed subscription quota: %v", err)
			}
			if err := repo.FinishSubscribe(context.Background(), delivery, SubscribeOutcome{Status: DeliverySkipped, QuotaAction: "none"}, staleNow); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("database-expired subscribe lease finished with stale now: %v", err)
			}
			var current DeliveryItem
			if err := db.First(&current, item.ID).Error; err != nil || current.Status != DeliveryProcessing {
				t.Fatalf("rejected stale finish changed delivery: %v", err)
			}

			// A live lease is still usable in either clock, while a worker that
			// loses its lease after reserving consent may not release that consent.
			isolatedNotificationDelivery(t, db)
			liveDelivery := isolatedNotificationClaim(t, repo)
			reserved, err := repo.ReserveForDelivery(context.Background(), liveDelivery, "isolated-app", "isolated-openid", "isolated-template", time.Now().UTC())
			if err != nil {
				t.Fatalf("live lease cannot reserve quota: %v", err)
			}
			if err := db.Model(&DeliveryItem{}).Where("id=?", reserved.Item.ID).Update("lease_expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
				t.Fatal(err)
			}
			if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryBlocked, QuotaAction: "release"}, staleNow); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("stale worker released occupied consent: %v", err)
			}
			if err := db.First(&quota, subscription.ID).Error; err != nil || quota.RemainingQuota != 0 || quota.ReservedQuota != 1 {
				t.Fatalf("stale finish released occupied consent: %v", err)
			}
			if _, found, err := repo.ClaimWechatDelivery(context.Background(), time.Now().UTC(), time.Minute); err != nil || found {
				t.Fatalf("expired occupied delivery was reclaimed: found=%v err=%v", found, err)
			}
			if err := db.First(&quota, subscription.ID).Error; err != nil || quota.Status != "exhausted" || quota.RemainingQuota != 0 || quota.ReservedQuota != 0 {
				t.Fatalf("expired uncertain send did not consume consent: %v", err)
			}
			var expiredResult DeliveryItem
			if err := db.First(&expiredResult, reserved.Item.ID).Error; err != nil || expiredResult.Status != DeliveryUnknown {
				t.Fatalf("expired occupied delivery not terminal unknown: %v", err)
			}

			positiveSubscription := isolatedNotificationSubscription(t, db)
			isolatedNotificationDelivery(t, db)
			positiveLease := isolatedNotificationClaim(t, repo)
			positiveReserved, err := repo.ReserveForDelivery(context.Background(), positiveLease, "isolated-app", "isolated-openid", "isolated-template", time.Now().UTC())
			if err != nil {
				t.Fatalf("fresh positive reservation failed: %v", err)
			}
			if err := repo.FinishSubscribe(context.Background(), positiveReserved, SubscribeOutcome{Status: DeliveryAccepted, QuotaAction: "consume"}, time.Now().UTC()); err != nil {
				t.Fatalf("live finish failed: %v", err)
			}
			var positiveQuota Subscription
			if err := db.First(&positiveQuota, positiveSubscription.ID).Error; err != nil || positiveQuota.Status != "exhausted" || positiveQuota.RemainingQuota != 0 || positiveQuota.ReservedQuota != 0 {
				t.Fatalf("live finish did not consume quota once: %v", err)
			}
		})
	}
}
