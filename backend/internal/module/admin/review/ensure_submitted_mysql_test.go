package review

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestEnsureSubmittedPreservesDecisionAgainstMySQL84(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set MYSQL_DSN to run Review EnsureSubmitted MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	const objectType = "ui21b2a_ensure"
	const objectID uint = 4_290_001
	cleanup := func() {
		if err := db.Where("object_type = ? AND object_id = ?", objectType, objectID).Delete(&Task{}).Error; err != nil {
			t.Errorf("cleanup review task: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	service := NewService(NewRepository(db))
	created, err := service.EnsureSubmitted(context.Background(), Submission{
		ObjectType: objectType, ObjectID: objectID, SubmitterID: 41, Risk: "original",
	})
	if err != nil {
		t.Fatal(err)
	}
	created.Status = StatusApproved
	created.Reason = "approved once"
	created.OperatorID = 73
	if err := db.Save(&created).Error; err != nil {
		t.Fatal(err)
	}

	replayed, err := service.EnsureSubmitted(context.Background(), Submission{
		ObjectType: objectType, ObjectID: objectID, SubmitterID: 99, Risk: "must not overwrite",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != created.ID || replayed.Status != StatusApproved || replayed.Reason != "approved once" || replayed.OperatorID != 73 || replayed.SubmitterID != 41 || replayed.Risk != "original" {
		t.Fatalf("replay reset existing decision: %#v", replayed)
	}
	var count int64
	if err := db.Model(&Task{}).Where("object_type = ? AND object_id = ?", objectType, objectID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("review task count=%d want=1", count)
	}
}

func TestEnsureSubmittedConcurrentAgainstMySQL84(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set MYSQL_DSN to run Review EnsureSubmitted MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	const objectType = "ui21b2a_concurrent"
	const objectID uint = 4_290_002
	cleanup := func() {
		if err := db.Where("object_type = ? AND object_id = ?", objectType, objectID).Delete(&Task{}).Error; err != nil {
			t.Errorf("cleanup review task: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	service := NewService(NewRepository(db))
	const workers = 20
	ids := make(chan uint, workers)
	errs := make(chan error, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			task, ensureErr := service.EnsureSubmitted(context.Background(), Submission{
				ObjectType: objectType, ObjectID: objectID, SubmitterID: uint(worker + 1), Risk: "concurrent",
			})
			if ensureErr != nil {
				errs <- ensureErr
				return
			}
			ids <- task.ID
		}(i)
	}
	close(start)
	wg.Wait()
	close(ids)
	close(errs)
	for ensureErr := range errs {
		t.Fatalf("concurrent ensure: %v", ensureErr)
	}
	var firstID uint
	for id := range ids {
		if firstID == 0 {
			firstID = id
		}
		if id == 0 || id != firstID {
			t.Fatalf("concurrent ensure returned inconsistent id=%d first=%d", id, firstID)
		}
	}
	var count int64
	if err := db.Model(&Task{}).Where("object_type = ? AND object_id = ?", objectType, objectID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("review task count=%d want=1", count)
	}
}
