package deliveryim

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type mysqlGroupPort struct {
	removed     []uint64
	muted       []string
	readOnlyErr error
}

func (*mysqlGroupPort) EnsureWorkGroup(context.Context, string, string, uint64, uint64) error {
	return nil
}
func (p *mysqlGroupPort) RemoveWorkGroupMember(_ context.Context, _ string, rider uint64) error {
	p.removed = append(p.removed, rider)
	return nil
}
func (p *mysqlGroupPort) SetWorkGroupReadOnly(_ context.Context, groupID string, readOnly bool) error {
	if p.readOnlyErr != nil {
		return p.readOnlyErr
	}
	if readOnly {
		p.muted = append(p.muted, groupID)
	}
	return nil
}
func (*mysqlGroupPort) DismissWorkGroup(context.Context, string) error { return nil }

type mysqlMediaPort struct{}

func (*mysqlMediaPort) ValidateOwnedApprovedConversationAsset(context.Context, uint64, string) error {
	return nil
}
func (*mysqlMediaPort) PublishConversationImage(context.Context, string, string, uint16) error {
	return nil
}
func (*mysqlMediaPort) ResolveConversationImage(context.Context, string, uint16, time.Duration) (string, error) {
	return "https://signed.example/private", nil
}

func TestD5ParticipantHTTPAndRiderReplacementAgainstMySQL84(t *testing.T) {
	dsn := os.Getenv("DELIVERY_D5_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("DELIVERY_D1_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set DELIVERY_D5_MYSQL_DSN or DELIVERY_D1_MYSQL_DSN")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&delivery.Conversation{}) {
		t.Fatal("migration 027 is required")
	}
	const merchant uint64 = 9_005_006_001
	riderOne := uint64(9_005_006_002)
	riderTwo := uint64(9_005_006_003)
	orderID := "01J00000000000000000000601"
	cleanupD5HTTP(t, db, merchant)
	t.Cleanup(func() { cleanupD5HTTP(t, db, merchant) })
	now := time.Now().UTC().Truncate(time.Millisecond)
	order := delivery.Order{
		DeliveryOrderPublicID: orderID, MerchantUserID: merchant, RiderUserID: &riderOne,
		PickupContactName: "Pickup", PickupContactPhone: "13100006001", PickupAddress: "Pickup A",
		RecipientName: "Recipient", RecipientPhone: "13100006002", DeliveryAddress: "Delivery B",
		DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50,
		Status: delivery.StatusClaimed, ReadyForClaimAt: &now, ClaimedAt: &now,
		RowVersion: 3, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	repository := delivery.NewRepository(db)
	orders, _ := delivery.NewService(repository, nil)
	groups := &mysqlGroupPort{}
	conversations, err := delivery.NewConversationService(
		repository, orders, groups, &mysqlMediaPort{}, delivery.ConversationServiceConfig{ImagesEnabled: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	first, err := conversations.EnsureForClaim(context.Background(), order)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	codec, _ := NewImageTokenCodec("0123456789abcdef0123456789abcdef")
	NewHandler(conversations, func(c *gin.Context) uint64 {
		value, _ := strconv.ParseUint(c.GetHeader("X-Test-User"), 10, 64)
		return value
	}, codec, true).Register(router.Group("/delivery"))

	assertD5ConversationHTTP(t, router, merchant, orderID, http.StatusOK)
	assertD5ConversationHTTP(t, router, riderOne, orderID, http.StatusOK)
	assertD5ConversationHTTP(t, router, riderOne+99, orderID, http.StatusNotFound)

	if err := conversations.RevokeBeforeRelease(context.Background(), riderOne, orderID); err != nil {
		t.Fatal(err)
	}
	if len(groups.removed) != 1 || groups.removed[0] != riderOne {
		t.Fatalf("old rider was not removed before release: %#v", groups.removed)
	}
	if err := db.Model(&delivery.Order{}).Where("id = ?", order.ID).Updates(map[string]any{
		"rider_user_id": nil, "status": delivery.StatusReadyForClaim,
		"row_version": gorm.Expr("row_version + 1"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	assertD5ConversationHTTP(t, router, riderOne, orderID, http.StatusNotFound)
	if err := db.Model(&delivery.Order{}).Where("id = ?", order.ID).Updates(map[string]any{
		"rider_user_id": riderTwo, "status": delivery.StatusClaimed,
		"row_version": gorm.Expr("row_version + 1"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	order.RiderUserID = &riderTwo
	order.Status = delivery.StatusClaimed
	second, err := conversations.EnsureForClaim(context.Background(), order)
	if err != nil {
		t.Fatal(err)
	}
	if second.AssignmentVersion != first.AssignmentVersion+1 ||
		second.TencentGroupID == first.TencentGroupID {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
	assertD5ConversationHTTP(t, router, riderTwo, orderID, http.StatusOK)
	if _, _, err := conversations.FindForParticipant(
		context.Background(), riderTwo, first.ConversationPublicID,
	); err != delivery.ErrConversationForbidden {
		t.Fatalf("new rider accessed old history: %v", err)
	}
	if err := db.Model(&delivery.Order{}).Where("id = ?", order.ID).Updates(map[string]any{
		"status": delivery.StatusDelivered, "row_version": gorm.Expr("row_version + 1"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	groups.readOnlyErr = errors.New("remote mute unavailable")
	if _, err := conversations.Current(context.Background(), merchant, orderID); !errors.Is(
		err, delivery.ErrConversationUnavailable,
	) {
		t.Fatalf("remote mute failure did not fail closed: %v", err)
	}
	stillActive, err := repository.FindConversation(context.Background(), second.ConversationPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if stillActive.Status != delivery.ConversationActive {
		t.Fatalf("local status advanced before remote mute: %s", stillActive.Status)
	}
	groups.readOnlyErr = nil
	readOnly, err := conversations.Current(context.Background(), merchant, orderID)
	if err != nil {
		t.Fatal(err)
	}
	if readOnly.Status != delivery.ConversationReadOnly ||
		len(groups.muted) != 1 || groups.muted[0] != second.TencentGroupID {
		t.Fatalf("remote read-only barrier not applied: conversation=%#v muted=%#v", readOnly, groups.muted)
	}
	t.Log("real_http=PASS real_mysql=PASS merchant=200 current_rider=200 outsider=404 old_rider=404 new_assignment=PASS remote_read_only_fail_closed=PASS")
}

func assertD5ConversationHTTP(
	t *testing.T,
	router http.Handler,
	userID uint64,
	orderID string,
	want int,
) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/delivery/orders/"+orderID+"/conversation", nil)
	request.Header.Set("X-Test-User", strconv.FormatUint(userID, 10))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != want {
		t.Fatalf("user=%d status=%d want=%d body=%s", userID, recorder.Code, want, recorder.Body.String())
	}
}

func cleanupD5HTTP(t *testing.T, db *gorm.DB, merchant uint64) {
	t.Helper()
	var orderIDs []string
	if err := db.Model(&delivery.Order{}).Where("merchant_user_id = ?", merchant).
		Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
		t.Errorf("list D-5 HTTP orders: %v", err)
		return
	}
	if len(orderIDs) > 0 {
		if err := db.Where("delivery_order_public_id IN ?", orderIDs).
			Delete(&delivery.Conversation{}).Error; err != nil {
			t.Errorf("cleanup D-5 HTTP conversations: %v", err)
		}
	}
	if err := db.Where("merchant_user_id = ?", merchant).Delete(&delivery.Order{}).Error; err != nil {
		t.Errorf("cleanup D-5 HTTP orders: %v", err)
	}
}
