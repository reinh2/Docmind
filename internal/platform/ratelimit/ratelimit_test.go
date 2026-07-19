package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterEnforcesWindow(t *testing.T) {
	l := New(2, time.Minute)
	if !l.Allow("client") || !l.Allow("client") || l.Allow("client") {
		t.Fatal("unexpected rate-limit result")
	}
}
