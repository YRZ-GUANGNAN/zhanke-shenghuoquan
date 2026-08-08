package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/community"
	"campus-circle/backend/internal/module/trade"
	"campus-circle/backend/internal/module/user"
)

const g6b1Fence = "g6b1-content-takedown"

type g6b1Envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestContentTakedownRealHTTPAndMySQL84(t *testing.T) {
	db := openG6B1MySQL(t)
	assertG6B1MySQL84(t, db)
	assertG6B1ReviewStatusSchema(t, db)
	cleanupG6B1(t, db)
	t.Cleanup(func() { cleanupG6B1(t, db) })

	ctx := context.Background()
	reviews := review.NewService(review.NewRepository(db))
	users := user.NewService(user.NewRepository(db), "g6b1-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)
	communityService := community.NewService(community.NewRepository(db), users, reviews)
	tradeService := trade.NewService(trade.NewRepository(db), users, reviews, nil, nil)
	adminService := NewService(NewRepository(db), reviews)

	author, err := users.WechatLogin(ctx, user.LoginInput{Code: g6b1Fence + "-author", Nickname: "G6B1 Author"})
	if err != nil {
		t.Fatal(err)
	}
	seller, err := users.WechatLogin(ctx, user.LoginInput{Code: g6b1Fence + "-seller", Nickname: "G6B1 Seller"})
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := users.WechatLogin(ctx, user.LoginInput{Code: g6b1Fence + "-ordinary", Nickname: "G6B1 Ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := users.WechatLogin(ctx, user.LoginInput{Code: g6b1Fence + "-admin", Nickname: "G6B1 Admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	post := community.Post{
		UserID: author.User.ID, Topic: g6b1Fence, Title: g6b1Fence + " post", Content: "visible before admin takedown",
		ReviewStatus: review.StatusApproved, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatal(err)
	}
	product := trade.Product{
		SellerID: seller.User.ID, Title: g6b1Fence + " product", Description: "visible before admin takedown",
		Category: g6b1Fence, Price: 199, Campus: "campus", Status: trade.StatusSelling,
		ReviewStatus: review.StatusApproved, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatal(err)
	}
	var reviewTasksBefore int64
	if err := db.Model(&review.Task{}).Where("(object_type = ? AND object_id = ?) OR (object_type = ? AND object_id = ?)", "post", post.ID, "product", product.ID).Count(&reviewTasksBefore).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	community.NewHandler(communityService, users, nil).Register(api.Group("/community"))
	trade.NewHandler(tradeService, users, nil).Register(api.Group("/trade"))
	NewHandler(adminService, reviews, users, communityService, tradeService, nil).Register(api)
	server := httptest.NewServer(router)
	defer server.Close()

	status, body := g6b1HTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/community/posts?keyword="+url.QueryEscape(post.Title), "", nil)
	assertG6B1PageContains(t, status, body, "id", fmt.Sprint(post.ID))
	status, body = g6b1HTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/trade/products?category="+product.Category, "", nil)
	assertG6B1PageContains(t, status, body, "id", fmt.Sprint(product.ID))

	takedownPostURL := fmt.Sprintf("%s/api/v1/admin/community/posts/%d/takedown", server.URL, post.ID)
	status, _ = g6b1HTTPJSON(t, server.Client(), http.MethodPatch, takedownPostURL, "", map[string]any{"reason": "admin policy enforcement"})
	if status != http.StatusUnauthorized {
		t.Fatalf("post takedown without token status=%d want=401", status)
	}
	status, _ = g6b1HTTPJSON(t, server.Client(), http.MethodPatch, takedownPostURL, ordinary.Token, map[string]any{"reason": "admin policy enforcement"})
	if status != http.StatusForbidden {
		t.Fatalf("post takedown ordinary user status=%d want=403", status)
	}
	status, _ = g6b1HTTPJSON(t, server.Client(), http.MethodPatch, takedownPostURL, administrator.Token, map[string]any{"reason": "admin policy enforcement"})
	if status != http.StatusOK {
		t.Fatalf("post takedown admin status=%d", status)
	}
	status, _ = g6b1HTTPJSON(t, server.Client(), http.MethodPatch, takedownPostURL, administrator.Token, map[string]any{"reason": "repeat takedown idempotency"})
	if status != http.StatusOK {
		t.Fatalf("post repeated takedown status=%d", status)
	}
	var loadedPost community.Post
	if err := db.First(&loadedPost, post.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loadedPost.ReviewStatus != community.ReviewStatusTakenDown {
		t.Fatalf("post review_status=%s want=%s", loadedPost.ReviewStatus, community.ReviewStatusTakenDown)
	}
	status, body = g6b1HTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/community/posts?keyword="+url.QueryEscape(post.Title), "", nil)
	assertG6B1PageMissing(t, status, body, "id", fmt.Sprint(post.ID))

	takedownProductURL := fmt.Sprintf("%s/api/v1/admin/trade/products/%d/takedown", server.URL, product.ID)
	status, _ = g6b1HTTPJSON(t, server.Client(), http.MethodPatch, takedownProductURL, administrator.Token, map[string]any{"reason": "admin policy enforcement"})
	if status != http.StatusOK {
		t.Fatalf("product takedown admin status=%d", status)
	}
	status, _ = g6b1HTTPJSON(t, server.Client(), http.MethodPatch, takedownProductURL, administrator.Token, map[string]any{"reason": "repeat takedown idempotency"})
	if status != http.StatusOK {
		t.Fatalf("product repeated takedown status=%d", status)
	}
	var loadedProduct trade.Product
	if err := db.First(&loadedProduct, product.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loadedProduct.ReviewStatus != trade.ReviewStatusTakenDown {
		t.Fatalf("product review_status=%s want=%s", loadedProduct.ReviewStatus, trade.ReviewStatusTakenDown)
	}
	if loadedProduct.Status != trade.StatusSelling {
		t.Fatalf("product status changed to %s; want preserved selling", loadedProduct.Status)
	}
	status, body = g6b1HTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/trade/products?category="+product.Category, "", nil)
	assertG6B1PageMissing(t, status, body, "id", fmt.Sprint(product.ID))

	var searchDeletes int64
	if err := db.Model(&trade.SearchOutbox{}).Where("product_id = ? AND operation = ?", product.ID, trade.SearchDelete).Count(&searchDeletes).Error; err != nil {
		t.Fatal(err)
	}
	if searchDeletes != 1 {
		t.Fatalf("search delete events=%d want=1 idempotent event", searchDeletes)
	}
	var reviewTasksAfter int64
	if err := db.Model(&review.Task{}).Where("(object_type = ? AND object_id = ?) OR (object_type = ? AND object_id = ?)", "post", post.ID, "product", product.ID).Count(&reviewTasksAfter).Error; err != nil {
		t.Fatal(err)
	}
	if reviewTasksAfter != reviewTasksBefore {
		t.Fatalf("F review tasks changed before=%d after=%d", reviewTasksBefore, reviewTasksAfter)
	}
	t.Log("g6b1_real_http=PASS auth=401/403/200 post_taken_down_hidden=PASS product_taken_down_hidden=PASS product_status_preserved=PASS idempotent=PASS f_review_unchanged=PASS search_delete_event_idempotent=PASS")
}

func TestZZContentTakedownResidueAgainstMySQL84(t *testing.T) {
	db := openG6B1MySQL(t)
	cleanupG6B1(t, db)
	assertG6B1Residue(t, db)
}

func openG6B1MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := strings.TrimSpace(os.Getenv("G6B1_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set G6B1_MYSQL_DSN or MYSQL_DSN for G-6b-1 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG6B1MySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("real MySQL version=%s want 8.4.x", version)
	}
	t.Logf("real_mysql_version=%s", version)
}

func assertG6B1ReviewStatusSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	type column struct {
		TableName     string
		ColumnType    string
		DataType      string
		IsNullable    string
		ColumnDefault string
	}
	var columns []column
	rows, err := db.Raw(`SELECT table_name,column_type,data_type,is_nullable,COALESCE(column_default,'') AS column_default
		FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name IN ('posts','products') AND column_name='review_status'
		ORDER BY table_name`).Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var col column
		if err := rows.Scan(&col.TableName, &col.ColumnType, &col.DataType, &col.IsNullable, &col.ColumnDefault); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, col)
	}
	if len(columns) != 2 {
		t.Fatalf("review_status columns=%d want=2: %+v", len(columns), columns)
	}
	for _, col := range columns {
		if col.ColumnType != "varchar(24)" || col.DataType != "varchar" || col.IsNullable != "NO" || col.ColumnDefault != "submitted" {
			t.Fatalf("unexpected %s.review_status schema: %+v", col.TableName, col)
		}
	}
	var checks int64
	if err := db.Raw(`SELECT COUNT(*)
		FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc
		  ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE()
		  AND tc.table_name IN ('posts','products')
		  AND tc.constraint_type='CHECK'
		  AND LOWER(cc.check_clause) LIKE '%review_status%'`).Scan(&checks).Error; err != nil {
		t.Fatal(err)
	}
	if checks != 0 {
		t.Fatalf("posts/products review_status CHECK constraints=%d want=0", checks)
	}
	t.Log("schema_check=PASS posts.review_status/products.review_status varchar(24) no review_status CHECK/ENUM; migration_required=false")
}

func g6b1HTTPJSON(t *testing.T, client *http.Client, method, url, token string, payload any) (int, g6b1Envelope) {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var envelope g6b1Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope failed status=%d raw=%s err=%v", response.StatusCode, raw, err)
	}
	return response.StatusCode, envelope
}

func assertG6B1PageContains(t *testing.T, status int, body g6b1Envelope, field, expected string) {
	t.Helper()
	items := decodeG6B1Page(t, status, body)
	for _, item := range items {
		if fmt.Sprint(item[field]) == expected {
			return
		}
	}
	t.Fatalf("page missing %s=%s: %s", field, expected, body.Data)
}

func assertG6B1PageMissing(t *testing.T, status int, body g6b1Envelope, field, expected string) {
	t.Helper()
	items := decodeG6B1Page(t, status, body)
	for _, item := range items {
		if fmt.Sprint(item[field]) == expected {
			t.Fatalf("page still contains %s=%s after takedown: %s", field, expected, body.Data)
		}
	}
}

func decodeG6B1Page(t *testing.T, status int, body g6b1Envelope) []map[string]any {
	t.Helper()
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("page status=%d body=%+v", status, body)
	}
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body.Data, &page); err != nil {
		t.Fatal(err)
	}
	return page.Items
}

func cleanupG6B1(t *testing.T, db *gorm.DB) {
	t.Helper()
	var productIDs []uint
	db.Model(&trade.Product{}).Unscoped().Where("title LIKE ?", g6b1Fence+"%").Pluck("id", &productIDs)
	if len(productIDs) > 0 {
		db.Where("product_id IN ?", productIDs).Delete(&trade.ProductImage{})
		db.Where("product_id IN ?", productIDs).Delete(&trade.ProductFavorite{})
		db.Where("product_id IN ?", productIDs).Delete(&trade.SearchOutbox{})
		db.Unscoped().Where("id IN ?", productIDs).Delete(&trade.Product{})
		db.Where("(object_type = ? AND object_id IN ?) OR (object_type = ? AND object_id IN ?)", "product", productIDs, "trade_product", productIDs).Delete(&review.Task{})
	}
	var postIDs []uint
	db.Model(&community.Post{}).Unscoped().Where("title LIKE ?", g6b1Fence+"%").Pluck("id", &postIDs)
	if len(postIDs) > 0 {
		db.Where("post_id IN ?", postIDs).Delete(&community.PostImage{})
		db.Where("post_id IN ?", postIDs).Delete(&community.PostLike{})
		db.Where("post_id IN ?", postIDs).Delete(&community.PostFavorite{})
		db.Where("post_id IN ?", postIDs).Delete(&community.Comment{})
		db.Where("post_id IN ?", postIDs).Delete(&community.Notification{})
		db.Unscoped().Where("id IN ?", postIDs).Delete(&community.Post{})
		db.Where("(object_type = ? AND object_id IN ?) OR (object_type = ? AND object_id IN ?)", "post", postIDs, "community_post", postIDs).Delete(&review.Task{})
	}
	db.Unscoped().Where("open_id IN ?", []string{
		"mock_openid_" + g6b1Fence + "-author",
		"mock_openid_" + g6b1Fence + "-seller",
		"mock_openid_" + g6b1Fence + "-ordinary",
		"mock_openid_" + g6b1Fence + "-admin",
	}).Delete(&user.User{})
}

func assertG6B1Residue(t *testing.T, db *gorm.DB) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		query string
	}{
		{"posts", &community.Post{}, "title LIKE '" + g6b1Fence + "%'"},
		{"products", &trade.Product{}, "title LIKE '" + g6b1Fence + "%'"},
		{"search_outbox", &trade.SearchOutbox{}, "product_id IN (SELECT id FROM products WHERE title LIKE '" + g6b1Fence + "%')"},
	}
	for _, check := range checks {
		var count int64
		if err := db.Unscoped().Model(check.model).Where(check.query).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s residue=%d", check.name, count)
		}
	}
	var users int64
	if err := db.Unscoped().Model(&user.User{}).Where("open_id LIKE ?", "mock_openid_"+g6b1Fence+"%").Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Errorf("users residue=%d", users)
	}
	t.Log("g6b1_residue posts=0 products=0 users=0 search_outbox=0")
}
