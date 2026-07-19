package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const optionalIdentityApplicationsQuery = `
SELECT COUNT(*)
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_name = 'identity_applications'`

type commandOptions struct {
	dsn    string
	output string
}

type auditRow struct {
	UserID                   uint64 `json:"userId"`
	Nickname                 string `json:"nickname"`
	Posts                    int64  `json:"posts"`
	Products                 int64  `json:"products"`
	IMRelations              int64  `json:"imRelations"`
	DeliveryOrders           int64  `json:"deliveryOrders"`
	IdentityApplications     int64  `json:"identityApplications"`
	SafeToConsiderForCleanup bool   `json:"safeToConsiderForCleanup"`
}

type auditReport struct {
	GeneratedAt           time.Time  `json:"generatedAt"`
	Users                 []auditRow `json:"users"`
	Total                 int        `json:"total"`
	OptionalTablesMissing []string   `json:"optionalTablesMissing"`
}

func main() {
	if err := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) error {
	options, err := parseOptions(args, getenv)
	if err != nil {
		return err
	}
	db, err := gorm.Open(mysql.Open(options.dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return errors.New("connect to audit database: failed")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("open audit database handle: failed")
	}
	defer sqlDB.Close()

	report, err := auditMockUsers(db, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := emitReport(options.output, report, stdout); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "mock users audited: %d\n", report.Total)
	return nil
}

func parseOptions(args []string, getenv func(string) string) (commandOptions, error) {
	flags := flag.NewFlagSet("audit-mock-users", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	output := flags.String("output", "", "write the JSON audit report to this file")
	if err := flags.Parse(args); err != nil {
		return commandOptions{}, errors.New("invalid arguments: only -output is supported")
	}
	if flags.NArg() != 0 {
		return commandOptions{}, errors.New("invalid arguments: positional arguments are not supported")
	}
	dsn := strings.TrimSpace(getenv("MYSQL_DSN"))
	if dsn == "" {
		return commandOptions{}, errors.New("MYSQL_DSN is required")
	}
	return commandOptions{dsn: dsn, output: strings.TrimSpace(*output)}, nil
}

func auditQueries() []string {
	return []string{
		optionalIdentityApplicationsQuery,
		auditQuery(false),
		auditQuery(true),
	}
}

func auditQuery(includeIdentityApplications bool) string {
	identityApplications := "0"
	if includeIdentityApplications {
		identityApplications = `(SELECT COUNT(*) FROM identity_applications ia WHERE ia.user_id = u.id)`
	}
	return fmt.Sprintf(`
SELECT
  u.id AS user_id,
  u.nickname AS nickname,
  (SELECT COUNT(*) FROM posts p WHERE p.user_id = u.id) AS posts,
  (SELECT COUNT(*) FROM products pr WHERE pr.seller_id = u.id) AS products,
  (SELECT COUNT(*) FROM im_conversations ic WHERE ic.buyer_id = u.id OR ic.seller_id = u.id) AS im_relations,
  (SELECT COUNT(*) FROM delivery_orders d WHERE d.merchant_user_id = u.id OR d.rider_user_id = u.id) AS delivery_orders,
  %s AS identity_applications
FROM users u
WHERE u.open_id LIKE 'mock_openid\\_%%'
ORDER BY u.id`, identityApplications)
}

func auditMockUsers(db *gorm.DB, generatedAt time.Time) (auditReport, error) {
	var optionalTableCount int64
	if err := db.Raw(optionalIdentityApplicationsQuery).Scan(&optionalTableCount).Error; err != nil {
		return auditReport{}, errors.New("check optional audit tables: failed")
	}

	missing := make([]string, 0, 1)
	includeIdentityApplications := optionalTableCount > 0
	if !includeIdentityApplications {
		missing = append(missing, "identity_applications")
	}
	rows := make([]auditRow, 0)
	if err := db.Raw(auditQuery(includeIdentityApplications)).Scan(&rows).Error; err != nil {
		return auditReport{}, errors.New("query mock user audit: failed")
	}
	return finalizeReport(rows, missing, generatedAt), nil
}

func finalizeReport(rows []auditRow, optionalTablesMissing []string, generatedAt time.Time) auditReport {
	for index := range rows {
		row := &rows[index]
		row.SafeToConsiderForCleanup = row.Posts == 0 &&
			row.Products == 0 &&
			row.IMRelations == 0 &&
			row.DeliveryOrders == 0 &&
			row.IdentityApplications == 0
	}
	return auditReport{
		GeneratedAt:           generatedAt,
		Users:                 rows,
		Total:                 len(rows),
		OptionalTablesMissing: optionalTablesMissing,
	}
}

func emitReport(output string, report auditReport, stdout io.Writer) error {
	writer := stdout
	var file *os.File
	if output != "" {
		created, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return errors.New("create audit report: failed")
		}
		file = created
		writer = file
		defer file.Close()
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return errors.New("write audit report: failed")
	}
	return nil
}
