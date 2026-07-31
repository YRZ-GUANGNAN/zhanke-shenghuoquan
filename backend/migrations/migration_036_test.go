package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migration036File = "036_trade_comments_auto_review.sql"

func readMigration036(t *testing.T, path ...string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(path...))
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

func TestMigration036DefinesTradeCommentsAndReviewAudit(t *testing.T) {
	source := readMigration036(t, migration036File)
	for _, token := range []string{
		"CREATE TABLE product_comments",
		"parent_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"reply_to_user_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"client_request_id CHAR(26) NOT NULL",
		"request_hash BINARY(32) NOT NULL",
		"UNIQUE INDEX uq_product_comments_user_request",
		"idx_product_comments_public",
		"idx_product_comment_replies",
		"idx_product_comments_user_recent",
		"ADD COLUMN product_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"ADD COLUMN product_comment_id BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"ADD COLUMN decision_source VARCHAR(24) NOT NULL DEFAULT 'manual'",
		"CHECK (decision_source IN ('manual','automatic','manual_override'))",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 036 missing %q", token)
		}
	}
}

func TestMigration036RebuildsNotificationUniqueness(t *testing.T) {
	source := readMigration036(t, migration036File)
	for _, token := range []string{
		"idx_notification_event",
		"recipient_id, actor_id, type, post_id, comment_id, product_id, product_comment_id",
		"GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')",
		"recipient_id,actor_id,type,post_id,comment_id",
		"recipient_id,actor_id,type,post_id,comment_id,product_id,product_comment_id",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 036 notification contract missing %q", token)
		}
	}
}

func TestMigration036IsGuardedForRepeatApplication(t *testing.T) {
	source := readMigration036(t, migration036File)
	for _, token := range []string{
		"CREATE PROCEDURE migrate_trade_comments_auto_review",
		"GET_LOCK('campus_circle:migration:036'",
		"RELEASE_LOCK('campus_circle:migration:036')",
		"information_schema.tables",
		"information_schema.columns",
		"information_schema.statistics",
		"information_schema.table_constraints",
		"CALL migrate_trade_comments_auto_review()",
		"DROP PROCEDURE migrate_trade_comments_auto_review",
		"SIGNAL SQLSTATE '45000'",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 036 repeat-application guard missing %q", token)
		}
	}
}

func TestMigration036RollbackIsSymmetricAndScoped(t *testing.T) {
	source := readMigration036(t, filepath.Join("rollback", migration036File))
	for _, token := range []string{
		"CREATE PROCEDURE rollback_trade_comments_auto_review",
		"GET_LOCK('campus_circle:rollback:036'",
		"DROP INDEX idx_notification_event",
		"ADD UNIQUE INDEX idx_notification_event (recipient_id, actor_id, type, post_id, comment_id)",
		"DROP COLUMN product_comment_id",
		"DROP COLUMN product_id",
		"DROP TABLE IF EXISTS product_comments",
		"DROP COLUMN decision_source",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 036 rollback missing %q", token)
		}
	}

	restoreIndex := strings.Index(source, "ADD UNIQUE INDEX idx_notification_event (recipient_id, actor_id, type, post_id, comment_id)")
	dropProductCommentID := strings.Index(source, "DROP COLUMN product_comment_id")
	dropProductID := strings.Index(source, "DROP COLUMN product_id")
	dropComments := strings.Index(source, "DROP TABLE IF EXISTS product_comments")
	dropDecisionSource := strings.Index(source, "DROP COLUMN decision_source")
	if restoreIndex < 0 || dropProductCommentID < 0 || dropProductID < 0 || dropComments < 0 || dropDecisionSource < 0 {
		t.Fatal("migration 036 rollback order tokens missing")
	}
	if restoreIndex > dropProductCommentID || restoreIndex > dropProductID {
		t.Fatal("migration 036 rollback must restore notification uniqueness before dropping product coordinates")
	}
	if dropComments < dropProductCommentID || dropComments < dropProductID {
		t.Fatal("migration 036 rollback must drop product_comments after notification coordinates")
	}
	if dropDecisionSource < dropProductCommentID || dropDecisionSource < dropProductID {
		t.Fatal("migration 036 rollback must drop decision_source after notification coordinates")
	}

	for _, forbidden := range []string{
		"DROP TABLE posts",
		"DROP TABLE products",
		"DROP TABLE identity_applications",
		"DROP TABLE im_conversations",
		"DROP TABLE delivery_orders",
	} {
		if strings.Contains(strings.ToUpper(source), strings.ToUpper(forbidden)) {
			t.Fatalf("migration 036 rollback must not remove existing business data: %q", forbidden)
		}
	}
}
