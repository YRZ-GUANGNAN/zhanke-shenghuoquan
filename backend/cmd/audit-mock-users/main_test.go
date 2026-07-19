package main

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestAuditQueriesAreReadOnlyAndCoverRequiredRelations(t *testing.T) {
	queries := strings.Join(auditQueries(), "\n")
	for _, token := range []string{
		`mock_openid\\_%`,
		"posts",
		"products",
		"im_conversations",
		"delivery_orders",
	} {
		if !strings.Contains(queries, token) {
			t.Fatalf("audit queries missing %q", token)
		}
	}
	for _, forbidden := range []string{"DELETE", "UPDATE", "INSERT", "DROP", "TRUNCATE", "ALTER", "REPLACE"} {
		if regexp.MustCompile(`(?i)\b` + forbidden + `\b`).MatchString(queries) {
			t.Fatalf("audit queries contain mutating statement %q", forbidden)
		}
	}
}

func TestParseOptionsOnlyReadsMySQLDSNAndAllowsOutput(t *testing.T) {
	var requested []string
	options, err := parseOptions([]string{"-output", "audit.json"}, func(key string) string {
		requested = append(requested, key)
		if key == "MYSQL_DSN" {
			return "test-user:test-password@tcp(127.0.0.1:3306)/isolated_test"
		}
		return "unexpected"
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.output != "audit.json" || options.dsn == "" {
		t.Fatalf("options=%+v", options)
	}
	if !reflect.DeepEqual(requested, []string{"MYSQL_DSN"}) {
		t.Fatalf("environment keys=%v", requested)
	}

	for _, args := range [][]string{{"-delete"}, {"inline-dsn"}, {"-output", "a.json", "extra"}} {
		if _, err := parseOptions(args, func(string) string { return "configured-dsn" }); err == nil {
			t.Fatalf("unsafe or positional arguments accepted: %v", args)
		}
	}
	if _, err := parseOptions(nil, func(string) string { return "  " }); err == nil {
		t.Fatal("missing MYSQL_DSN was accepted")
	}
}

func TestFinalizeReportMarksOnlyUnreferencedUsersAndOmitsSensitiveFields(t *testing.T) {
	generatedAt := time.Date(2026, 7, 19, 8, 30, 0, 0, time.UTC)
	report := finalizeReport([]auditRow{
		{UserID: 1, Nickname: "unreferenced"},
		{UserID: 2, Nickname: "referenced", Posts: 1, Products: 2, IMRelations: 3, DeliveryOrders: 4, IdentityApplications: 5},
	}, []string{"identity_applications"}, generatedAt)
	if report.Total != 2 || !report.Users[0].SafeToConsiderForCleanup || report.Users[1].SafeToConsiderForCleanup {
		t.Fatalf("report=%+v", report)
	}
	if !reflect.DeepEqual(report.OptionalTablesMissing, []string{"identity_applications"}) {
		t.Fatalf("optional tables=%v", report.OptionalTablesMissing)
	}

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"openId", "open_id", "phone", "token", "messageBody", "message_body"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("sensitive field %q present in report: %s", forbidden, encoded)
		}
	}
}
