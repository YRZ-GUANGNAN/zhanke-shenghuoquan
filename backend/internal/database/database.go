package database

import (
	"errors"
	"fmt"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/config"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	if cfg.EnableUserNotifications {
		if err := validateNotificationClockDSN(cfg.MySQLDSN, time.Now()); err != nil {
			return nil, err
		}
	}
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}

// Notification leases compare DATETIME columns with CURRENT_TIMESTAMP at SQL
// execution time. The driver's loc controls DATETIME serialization; time_zone
// controls the database clock and must match on every pooled connection. Do not
// infer or change a legacy storage convention during deployment.
func validateNotificationClockDSN(dsn string, now time.Time) error {
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return errors.New("notifications require a valid MySQL DSN; connection details omitted")
	}
	if !cfg.ParseTime || cfg.Loc == nil || now.IsZero() {
		return errors.New("notifications require parseTime=true and a known MySQL loc")
	}
	localNow := now.In(cfg.Loc)
	_, end := localNow.ZoneBounds()
	if !end.IsZero() {
		return errors.New("notifications do not support MySQL loc with future timezone transitions; use a verified fixed-offset storage clock")
	}
	_, offset := localNow.Zone()
	if offset%60 != 0 || offset < -(13*60+59)*60 || offset > 14*60*60 {
		return errors.New("notifications require a supported whole-minute MySQL timezone offset")
	}
	sign := "+"
	if offset < 0 {
		sign, offset = "-", -offset
	}
	expected := fmt.Sprintf("'%s%02d:%02d'", sign, offset/3600, offset%3600/60)
	if cfg.Params["time_zone"] != expected {
		return errors.New("notifications require an explicit fixed-offset MySQL time_zone matching loc; verify existing storage before updating configuration")
	}
	return nil
}
