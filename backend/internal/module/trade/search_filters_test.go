package trade

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestTradeSearchFiltersValidation(t *testing.T) {
	zero, maximum, negative := int64(0), int64(9900), int64(-1)
	for _, value := range []SearchFilters{{}, {Sort: "relevance"}, {Sort: "recent", Category: "其他"}, {Sort: "price_asc", MinPrice: &zero, MaxPrice: &maximum}, {Sort: "price_desc", MaxPrice: &zero}} {
		if err := validateSearchFilters(value); err != nil {
			t.Fatalf("valid filters %+v: %v", value, err)
		}
	}
	for _, value := range []SearchFilters{{Sort: "price; DROP TABLE products"}, {MinPrice: &negative}, {MinPrice: &maximum, MaxPrice: &zero}, {Category: strings.Repeat("长", 65)}} {
		if err := validateSearchFilters(value); err == nil {
			t.Fatalf("accepted invalid filters %+v", value)
		}
	}
}

func TestTradeSearchFiltersSQLKeepsVisibilityAndCents(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	minimum, maximum := int64(0), int64(1001)
	filters := SearchFilters{Sort: "price_asc", Category: "书籍资料", MinPrice: &minimum, MaxPrice: &maximum}
	query := func() *gorm.DB { return r.searchProductsQuery(context.Background(), 73, "校园' OR 1=1", filters) }
	var products []Product
	statement := query().Offset(20).Limit(20).Find(&products).Statement
	sql := statement.SQL.String()
	for _, want := range []string{"review_status = ? AND status = ?", "ub.blocked_id = products.seller_id", "category = ?", "price >= ?", "price <= ?", "`products`.`deleted_at` IS NULL", "ORDER BY price ASC, created_at DESC, id DESC", "LIMIT ? OFFSET ?"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("missing %q in %s", want, sql)
		}
	}
	if strings.Contains(sql, "书籍资料") || strings.Contains(sql, "校园") {
		t.Fatalf("input interpolated into SQL: %s", sql)
	}
	hasZero, hasCents := false, false
	for _, value := range statement.Vars {
		if value == minimum {
			hasZero = true
		}
		if value == maximum {
			hasCents = true
		}
	}
	if !hasZero || !hasCents {
		t.Fatalf("price bounds not bound in cents: %v", statement.Vars)
	}
	var total int64
	count := query().Count(&total).Statement.SQL.String()
	if !strings.Contains(count, "price >= ?") || !strings.Contains(count, "category = ?") || strings.Contains(count, "LIMIT") {
		t.Fatalf("count=%s", count)
	}
	for sortKey, column := range map[string]string{"": "CASE", "recent": "created_at DESC, id DESC", "price_desc": "price DESC, created_at DESC, id DESC"} {
		ordered := r.searchProductsQuery(context.Background(), 73, "校园", SearchFilters{Sort: sortKey}).Find(&products).Statement.SQL.String()
		if !strings.Contains(ordered, "ORDER BY "+column) {
			t.Fatalf("sort %s: %s", sortKey, ordered)
		}
	}
}
