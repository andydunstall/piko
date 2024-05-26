package backoff

import (
	"context"
	"math/rand"
	"time"
)

// Backoff implements exponential backoff with jitter.
type Backoff struct {
	// retries is the maximum number of attempts.
	retries    int
	minBackoff time.Duration
	maxBackoff time.Duration

	// attempts is the number of attempts so far.
	attempts    int
	lastBackoff time.Duration
}

// New creates a new backoff.
//
// Set 'retries' to zero to retry forever.
func New(retries int, minBackoff time.Duration, maxBackoff time.Duration) *Backoff {
	return &Backoff{
		retries:    retries,
		minBackoff: minBackoff,
		maxBackoff: maxBackoff,
		attempts:   0,
	}
}

// Wait blocks until the next retry. Returns false if the number of retries has
// been reached so the client should stop.
func (b *Backoff) Wait(ctx context.Context) bool {
	if b.retries != 0 && b.attempts > b.retries {
		return false
	}
	b.attempts++

	backoff := b.nextWait()
	b.lastBackoff = backoff

	select {
	case <-time.After(b.lastBackoff):
		return true
	case <-ctx.Done():
		return false
	}
}

func (b *Backoff) nextWait() time.Duration {
	var backoff time.Duration
	if b.lastBackoff == 0 {
		backoff = b.minBackoff
	} else {
		backoff = b.lastBackoff * 2
	}

	jitterMultipler := 1.0 + (rand.Float64() * 0.1)
	return time.Duration(float64(backoff) * jitterMultipler)
}
