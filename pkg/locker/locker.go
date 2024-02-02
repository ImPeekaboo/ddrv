// Package locker provides a concurrency control mechanism to manage
// locks based on unique identifiers. It allows multiple goroutines
// to safely acquire and release locks associated with specific IDs,
// ensuring that resources tied to these IDs are not accessed concurrently.
// The package uses reference counting to clean up unused locks, optimizing memory usage.
package locker

import (
	"sync"
)

// Locker is a struct that manages a collection of locks.
// Each lock is associated with a unique ID and is reference counted.
type Locker struct {
	mutexes sync.Map // A concurrent map to store mutexes for each id
}

type lockRef struct {
	mu    sync.Mutex
	count int
}

func New() *Locker {
	return new(Locker)
}

// Acquire obtains a lock for the specified ID. If the lock does not exist,
// it is created. Acquire increments the reference count for the lock,
// indicating that it is in use.
func (l *Locker) Acquire(id string) {
	ref, _ := l.mutexes.LoadOrStore(id, &lockRef{})
	lock := ref.(*lockRef)

	lock.mu.Lock()
	lock.count++
	lock.mu.Unlock()
}

// Release releases the lock for the specified ID. It decrements the reference
// count for the lock, and if the count reaches zero, the lock is removed from
// the Locker. This frees up resources for locks that are no longer in use.
func (l *Locker) Release(id string) {
	ref, ok := l.mutexes.Load(id)
	if !ok {
		return
	}
	lock := ref.(*lockRef)

	lock.mu.Lock()
	if lock.count > 0 {
		lock.count--
		if lock.count == 0 {
			l.mutexes.Delete(id)
		}
	}
	lock.mu.Unlock()
}
