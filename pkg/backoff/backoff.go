package backoff

import (
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

// Backoff returns whether to retry or abort, and how long to backoff for.
func (b *Backoff) Backoff() (time.Duration, bool) {
	if b.retries != 0 && b.attempts > b.retries {
		return 0, false
	}
	b.attempts++

	backoff := b.nextWait()
	b.lastBackoff = backoff

	return backoff, true
}

func (b *Backoff) nextWait() time.Duration {
	var backoff time.Duration
	if b.lastBackoff == 0 {
		backoff = b.minBackoff
	} else {
		backoff = b.lastBackoff * 2
	}
	if backoff > b.maxBackoff {
		backoff = b.maxBackoff
	}

	jitterMultipler := 1.0 + (rand.Float64() * 0.1)
	return time.Duration(float64(backoff) * jitterMultipler)
}
