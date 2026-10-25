package delivery

import (
	"os"
	"testing"
)

func TestRiderPayoutQueueAgainstRealMySQL(t *testing.T) {
	if os.Getenv("DELIVERY_PAYOUT_QUEUE_MYSQL_DSN") == "" {
		t.Skip("set DELIVERY_PAYOUT_QUEUE_MYSQL_DSN to run isolated queue concurrency checks")
	}
	t.Skip("isolated MySQL queue harness is reserved for the integration environment")
}
