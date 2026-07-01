package delivery

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

const d5ConversationMerchantID uint64 = 9_005_005_001

func TestD5MigrationApplyRollbackApplyAgainstMySQL84(t *testing.T) {
	db := openD1MySQL(t)
	assertD1MySQL84(t, db)
	if db.Migrator().HasTable(&Conversation{}) {
		var count int64
		if err := db.Model(&Conversation{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refuse to replace nonempty delivery_im_conversations: count=%d", count)
		}
		if err := executeD5SQLFile(db, "../../../migrations/rollback/027_delivery_im.sql"); err != nil {
			t.Fatalf("reset existing 027: %v", err)
		}
	}
	if err := executeD5SQLFile(db, "../../../migrations/027_delivery_im.sql"); err != nil {
		t.Fatalf("apply 027: %v", err)
	}
	assertD5ConversationSchema(t, db)

	now := time.Now().UTC().Truncate(time.Millisecond)
	fact := Conversation{
		ConversationPublicID:  "01J00000000000000000000591",
		DeliveryOrderPublicID: "01J00000000000000000000592",
		AssignmentVersion:     1, MerchantUserID: d5ConversationMerchantID,
		RiderUserID:    d5ConversationMerchantID + 1,
		TencentGroupID: "d5_01J00000000000000000000591",
		Status:         ConversationProvisioning, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&fact).Error; err != nil {
		t.Fatal(err)
	}
	if err := executeD5SQLFile(db, "../../../migrations/rollback/027_delivery_im.sql"); err == nil ||
		!strings.Contains(err.Error(), "still contains facts") {
		t.Fatalf("rollback with fact must fail closed, err=%v", err)
	}
	if err := db.Where("conversation_public_id = ?", fact.ConversationPublicID).Delete(&Conversation{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := executeD5SQLFile(db, "../../../migrations/rollback/027_delivery_im.sql"); err != nil {
		t.Fatalf("rollback 027: %v", err)
	}
	if db.Migrator().HasTable(&Conversation{}) {
		t.Fatal("rollback left delivery_im_conversations")
	}
	if err := executeD5SQLFile(db, "../../../migrations/027_delivery_im.sql"); err != nil {
		t.Fatalf("re-apply 027: %v", err)
	}
	assertD5ConversationSchema(t, db)
	t.Log("migration_027_apply=PASS rollback=PASS fail_closed=PASS reapply=PASS")
}

func TestD5ConversationAssignmentParticipantLeaseAndRevocationAgainstMySQL84(t *testing.T) {
	db := openD1MySQL(t)
	assertD1MySQL84(t, db)
	for _, table := range []any{&Order{}, &Conversation{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required D-5 migration table missing for %T", table)
		}
	}
	cleanupD5ConversationMySQL(t, db)
	t.Cleanup(func() { cleanupD5ConversationMySQL(t, db) })

	now := time.Now().UTC().Truncate(time.Millisecond)
	riderOne := uint64(9_005_005_002)
	order := d5ConversationOrder(now, riderOne)
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	conversation, loadedOrder, created, err := repository.EnsureConversationForCurrentAssignment(
		context.Background(), order.DeliveryOrderPublicID, riderOne, now,
	)
	if err != nil || !created || loadedOrder.ID != order.ID || conversation.AssignmentVersion != 1 {
		t.Fatalf("conversation=%#v order=%#v created=%v err=%v", conversation, loadedOrder, created, err)
	}
	idempotent, _, created, err := repository.EnsureConversationForCurrentAssignment(
		context.Background(), order.DeliveryOrderPublicID, riderOne, now,
	)
	if err != nil || created || idempotent.ID != conversation.ID {
		t.Fatalf("idempotent=%#v created=%v err=%v", idempotent, created, err)
	}
	if _, _, err := repository.FindConversationForParticipant(
		context.Background(), conversation.ConversationPublicID, d5ConversationMerchantID,
	); err != nil {
		t.Fatalf("merchant participant: %v", err)
	}
	if _, _, err := repository.FindConversationForParticipant(
		context.Background(), conversation.ConversationPublicID, riderOne,
	); err != nil {
		t.Fatalf("rider participant: %v", err)
	}
	if _, _, err := repository.FindConversationForParticipant(
		context.Background(), conversation.ConversationPublicID, riderOne+99,
	); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("nonparticipant err=%v", err)
	}

	lease, found, err := repository.ClaimConversation(
		context.Background(), conversation.ConversationPublicID, now.Add(time.Second), time.Minute,
	)
	if err != nil || !found || lease.Conversation.AttemptCount != 1 {
		t.Fatalf("lease=%#v found=%v err=%v", lease, found, err)
	}
	if err := repository.CompleteConversationProvision(context.Background(), lease, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	active, err := repository.FindConversation(context.Background(), conversation.ConversationPublicID)
	if err != nil || active.Status != ConversationActive {
		t.Fatalf("active=%#v err=%v", active, err)
	}
	revoking, err := repository.BeginConversationRevocation(
		context.Background(), active.ID, active.Status, active.RowVersion, now.Add(3*time.Second),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteConversationRevocation(
		context.Background(), revoking.ID, revoking.RowVersion, now.Add(4*time.Second),
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Order{}).Where("id = ?", order.ID).Updates(map[string]any{
		"rider_user_id": nil, "status": StatusReadyForClaim, "row_version": gorm.Expr("row_version + 1"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.FindConversationForParticipant(
		context.Background(), conversation.ConversationPublicID, riderOne,
	); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("released rider err=%v", err)
	}
	riderTwo := uint64(9_005_005_003)
	if err := db.Model(&Order{}).Where("id = ?", order.ID).Updates(map[string]any{
		"rider_user_id": riderTwo, "status": StatusClaimed, "row_version": gorm.Expr("row_version + 1"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	next, _, created, err := repository.EnsureConversationForCurrentAssignment(
		context.Background(), order.DeliveryOrderPublicID, riderTwo, now.Add(5*time.Second),
	)
	if err != nil || !created || next.AssignmentVersion != 2 || next.TencentGroupID == conversation.TencentGroupID {
		t.Fatalf("next=%#v created=%v err=%v", next, created, err)
	}
	if _, _, err := repository.FindConversationForParticipant(
		context.Background(), conversation.ConversationPublicID, riderTwo,
	); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("new rider saw old conversation err=%v", err)
	}
	t.Log("d5_assignment_generation=PASS participant_fence=PASS old_rider_404=PASS lease=PASS revocation=PASS")
}

func d5ConversationOrder(now time.Time, riderID uint64) Order {
	return Order{
		DeliveryOrderPublicID: "01J00000000000000000000501",
		MerchantUserID:        d5ConversationMerchantID, RiderUserID: &riderID,
		PickupContactName: "Pickup", PickupContactPhone: "13100005001", PickupAddress: "Pickup A",
		RecipientName: "Recipient", RecipientPhone: "13100005002", DeliveryAddress: "Delivery B",
		DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50,
		Status: StatusClaimed, ReadyForClaimAt: &now, ClaimedAt: &now,
		RowVersion: 3, CreatedAt: now, UpdatedAt: now,
	}
}

func cleanupD5ConversationMySQL(t *testing.T, db *gorm.DB) {
	t.Helper()
	var orderIDs []string
	if err := db.Model(&Order{}).Where("merchant_user_id = ?", d5ConversationMerchantID).
		Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
		t.Errorf("list D-5 orders: %v", err)
		return
	}
	if len(orderIDs) > 0 {
		if err := db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&Conversation{}).Error; err != nil {
			t.Errorf("cleanup D-5 conversations: %v", err)
		}
	}
	if err := db.Where("merchant_user_id = ?", d5ConversationMerchantID).Delete(&Order{}).Error; err != nil {
		t.Errorf("cleanup D-5 orders: %v", err)
	}
}

func assertD5ConversationSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&Conversation{}) {
		t.Fatal("delivery_im_conversations missing")
	}
	var foreignKeys int64
	if err := db.Raw(`
		SELECT COUNT(*) FROM information_schema.referential_constraints
		WHERE constraint_schema = DATABASE() AND table_name = 'delivery_im_conversations'
	`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("delivery_im_conversations foreign keys=%d want=0", foreignKeys)
	}
	var checkClause string
	if err := db.Raw(`
		SELECT check_clause FROM information_schema.check_constraints
		WHERE constraint_schema=DATABASE() AND constraint_name='chk_media_reference_type'
	`).Scan(&checkClause).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(checkClause, "delivery_im") {
		t.Fatalf("media reference CHECK missing delivery_im: %s", checkClause)
	}
}

func executeD5SQLFile(db *gorm.DB, relativePath string) error {
	file, err := os.Open(filepath.Clean(relativePath))
	if err != nil {
		return err
	}
	defer file.Close()
	delimiter := ";"
	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			delimiter = strings.TrimSpace(trimmed[len("DELIMITER "):])
			continue
		}
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(trimmed, delimiter) {
			continue
		}
		query := strings.TrimSpace(statement.String())
		query = strings.TrimSpace(strings.TrimSuffix(query, delimiter))
		statement.Reset()
		if query != "" {
			if err := db.Exec(query).Error; err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(statement.String()) != "" {
		return errors.New("unterminated SQL statement")
	}
	return nil
}
