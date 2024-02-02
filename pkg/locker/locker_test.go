package locker_test

import (
	"sync"
	"testing"

	"github.com/forscht/ddrv/pkg/locker"
)

func TestLocker(t *testing.T) {
	l := &locker.Locker{}

	testID := "test_id"

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Acquire(testID)
			l.Release(testID)
		}()
	}

	wg.Wait()
}

func TestLocker_MultipleIDs(t *testing.T) {
	l := &locker.Locker{}

	testIDs := []string{"id1", "id2", "id3"}

	var wg sync.WaitGroup
	for _, id := range testIDs {
		wg.Add(1)
		go func(testID string) {
			defer wg.Done()
			l.Acquire(testID)
			l.Release(testID)
		}(id)
	}

	wg.Wait()
}
