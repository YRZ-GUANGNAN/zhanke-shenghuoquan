package migrations

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigration032SourceContract(t *testing.T) {
	source, err := os.ReadFile("032_domain_search_and_community_tags.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, token := range []string{
		"ADD COLUMN tags JSON NULL",
		"CREATE TABLE community_search_histories",
		"CREATE TABLE community_search_events",
		"CREATE TABLE trade_search_histories",
		"CREATE TABLE trade_search_events",
		"CREATE TABLE navigation_search_histories",
		"CREATE TABLE navigation_search_events",
		"uq_community_search_history_user_keyword",
		"idx_community_search_event_keyword_created",
		"uq_trade_search_history_user_keyword",
		"idx_trade_search_event_keyword_created",
		"uq_navigation_search_history_user_keyword",
		"idx_navigation_search_event_keyword_created",
	} {
		if !strings.Contains(text, token) {
			t.Fatalf("migration 032 missing %q", token)
		}
	}
	if strings.Contains(strings.ToLower(text), "scope") {
		t.Fatal("migration 032 must use separate domain tables, not a shared scope column")
	}
	if _, err := os.Stat(filepath.Join("rollback", "032_domain_search_and_community_tags.sql")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration 032 must stay forward-only, rollback stat err=%v", err)
	}
}
