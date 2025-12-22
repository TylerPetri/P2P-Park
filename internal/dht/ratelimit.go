package dht

import "time"

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func (b *tokenBucket) allow(now time.Time, rate float64, burst float64, cost float64) bool {
	if b.last.IsZero() {
		b.last = now
		b.tokens = burst
	}
	elapsed := now.Sub(b.last).Seconds()
	b.last = now

	// refill
	b.tokens += elapsed * rate
	if b.tokens > burst {
		b.tokens = burst
	}
	if b.tokens < cost {
		return false
	}
	b.tokens -= cost
	return true
}
