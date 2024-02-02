package ddrv

// Simple version of https://github.com/diamondburned/arikawa/blob/v3/api/rate/rate.go
import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

const ExtraDelay = 250 * time.Millisecond

type Limiter struct {
	lock    sync.Mutex
	global  time.Time
	buckets map[string]*bucket
}

type bucket struct {
	lock      sync.Mutex
	reset     time.Time
	remaining uint64
}

func NewLimiter() *Limiter {
	return &Limiter{buckets: map[string]*bucket{}}
}

func (l *Limiter) getBucket(path string, store bool) *bucket {
	l.lock.Lock()
	defer l.lock.Unlock()

	b, ok := l.buckets[path]
	if !ok && store {
		b = &bucket{remaining: 1}
		l.buckets[path] = b
	}
	return b
}

func (l *Limiter) Acquire(path string) {
	now := time.Now()
	// Check global rate limit
	if l.global.After(now) {
		time.Sleep(l.global.Sub(now) + ExtraDelay)
	}

	b := l.getBucket(path, true)

	// Lock bucket until released
	b.lock.Lock()
	// Check bucket-specific rate limit
	if b.remaining == 0 && b.reset.After(now) {
		time.Sleep(b.reset.Sub(now) + ExtraDelay)
	}

	if b.remaining > 0 {
		b.remaining--
	}
}

func (l *Limiter) Release(path string, headers http.Header) {
	b := l.getBucket(path, false)
	// Continue if no specific bucket was found
	if b == nil {
		return
	}
	// Release bucket for next request
	defer b.lock.Unlock()
	// Request failed due to network error
	if headers == nil {
		return
	}
	var (
		global     = headers.Get("X-RateLimit-Global") // boolean
		remaining  = headers.Get("X-RateLimit-Remaining")
		reset      = headers.Get("X-RateLimit-Reset") // float
		retryAfter = headers.Get("Retry-After")
	)

	switch {
	case retryAfter != "":
		i, err := strconv.Atoi(retryAfter)
		if err != nil {
			return
		}

		at := time.Now().Add(time.Duration(i) * time.Second)

		// probably "true"
		if global != "" {
			l.global = at
		} else {
			b.reset = at
		}

	case reset != "":
		unix, err := strconv.ParseFloat(reset, 64)
		if err != nil {
			return
		}
		sec := int64(unix)
		nsec := int64((unix - float64(sec)) * float64(time.Second))
		b.reset = time.Unix(sec, nsec).Add(ExtraDelay)
	}

	if remaining != "" {
		u, err := strconv.ParseUint(remaining, 10, 64)
		if err != nil {
			return
		}
		b.remaining = u
	}

	return
}
