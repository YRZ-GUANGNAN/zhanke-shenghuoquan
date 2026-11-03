package database

import (
	"strings"
	"testing"
	"time"
)

func TestNotificationClockDSN(t *testing.T) {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	previousLocal := time.Local
	time.Local = shanghai
	t.Cleanup(func() { time.Local = previousLocal })
	for _, test := range []struct {
		name, query string
		valid       bool
	}{
		{"utc", "parseTime=true&loc=UTC&time_zone=%27%2B00%3A00%27", true},
		{"shanghai", "parseTime=true&loc=Asia%2FShanghai&time_zone=%27%2B08%3A00%27", true},
		{"local_shanghai", "parseTime=true&loc=Local&time_zone=%27%2B08%3A00%27", true},
		{"missing", "parseTime=true&loc=UTC", false},
		{"mismatch", "parseTime=true&loc=Asia%2FShanghai&time_zone=%27%2B00%3A00%27", false},
		{"system", "parseTime=true&loc=Local&time_zone=%27SYSTEM%27", false},
		{"named", "parseTime=true&loc=Asia%2FShanghai&time_zone=%27Asia%2FShanghai%27", false},
		{"unparsed", "parseTime=false&loc=UTC&time_zone=%27%2B00%3A00%27", false},
		{"dst", "parseTime=true&loc=America%2FNew_York&time_zone=%27-05%3A00%27", false},
		{"invalid", "parseTime=not_bool", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := validateNotificationClockDSN("private_user:private_password@tcp(private_host:3306)/private_db?"+test.query, now)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if err != nil && strings.Contains(err.Error(), "private_") {
				t.Fatal("DSN details escaped into validation error")
			}
		})
	}
}
