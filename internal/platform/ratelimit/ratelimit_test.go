package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterEnforcesWindow(t *testing.T) {
	l := New(2, time.Minute)
	if !l.Allow("client") {
		t.Fatal("first request should be allowed")
	}
	if !l.Allow("client") {
		t.Fatal("second request should be allowed")
	}
	if l.Allow("client") {
		t.Fatal("third request should be blocked")
	}
}
