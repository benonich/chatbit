package domain_test

import (
	"chatbit/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClampTTL(t *testing.T) {
	assert.Equal(t, domain.MaxMessageTTL, domain.ClampTTL(0))
	assert.Equal(t, domain.MaxMessageTTL, domain.ClampTTL(-5))
	assert.Equal(t, domain.MaxMessageTTL, domain.ClampTTL(domain.MaxMessageTTL+1))
	assert.Equal(t, int64(3600), domain.ClampTTL(3600))
}

func TestRoomMessageTTL(t *testing.T) {
	r := &domain.Room{TTL: 3600}

	assert.Equal(t, int64(3600), r.MessageTTL(0), "no own TTL → room TTL")
	assert.Equal(t, int64(300), r.MessageTTL(300), "shorter step is allowed")
	assert.Equal(t, int64(3600), r.MessageTTL(86400), "longer TTL is capped to the room TTL")
	assert.Equal(t, int64(300), r.MessageTTL(1000), "free value is rounded down to a step")
	assert.Equal(t, int64(300), r.MessageTTL(10), "below the smallest step → smallest step")

	week := &domain.Room{TTL: 30 * 24 * 3600}
	assert.Equal(t, int64(24*3600), week.MessageTTL(2*24*3600), "2 days → 1 day")
	assert.Equal(t, int64(7*24*3600), week.MessageTTL(20*24*3600), "20 days → 7 days")

	short := &domain.Room{TTL: 120}
	assert.Equal(t, int64(120), short.MessageTTL(60), "never longer than the room TTL")

	legacy := &domain.Room{}
	assert.Equal(t, domain.MaxMessageTTL, legacy.MessageTTL(0))
}

func TestRoomSetTTLOnce(t *testing.T) {
	r := &domain.Room{}

	assert.Equal(t, int64(3600), r.SetTTLOnce(3600))
	// A second value is ignored — the TTL can only be set once
	assert.Equal(t, int64(3600), r.SetTTLOnce(60))
	assert.Equal(t, int64(3600), r.SetTTLOnce(0))

	// No TTL from the creator means the maximum, which is then fixed as well
	r2 := &domain.Room{}
	assert.Equal(t, domain.MaxMessageTTL, r2.SetTTLOnce(0))
	assert.Equal(t, domain.MaxMessageTTL, r2.SetTTLOnce(60))
}
