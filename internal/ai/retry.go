package ai

import (
	"context"
	"errors"
	"github.com/reinh2/docmind/internal/domain"
	"math/rand"
	"time"
)

type RetryProvider struct {
	next     Provider
	attempts int
	wait     func(context.Context, time.Duration) error
}

func WithRetries(next Provider, attempts int) *RetryProvider {
	return &RetryProvider{next: next, attempts: attempts, wait: wait}
}
func (r *RetryProvider) Embed(ctx context.Context, text string) ([]float64, error) {
	var out []float64
	err := r.retry(ctx, func(bool) error { var e error; out, e = r.next.Embed(ctx, text); return e })
	return out, err
}
func (r *RetryProvider) RewriteQuery(ctx context.Context, h []domain.Message, q string) (string, error) {
	var out string
	err := r.retry(ctx, func(bool) error { var e error; out, e = r.next.RewriteQuery(ctx, h, q); return e })
	return out, err
}
func (r *RetryProvider) Answer(ctx context.Context, q string, c []domain.Chunk, onDelta func(string)) (string, error) {
	var out string
	// Streams from every attempt, including retries: a transient failure only ever
	// happens before the provider has emitted any deltas, so there is no risk of a
	// partial answer being followed by a second, conflicting one.
	err := r.retry(ctx, func(final bool) error {
		var e error
		out, e = r.next.Answer(ctx, q, c, onDelta)
		return e
	})
	return out, err
}
func (r *RetryProvider) retry(ctx context.Context, fn func(final bool) error) error {
	var err error
	for n := 0; n < r.attempts; n++ {
		if err = fn(n == r.attempts-1); err == nil {
			return nil
		}
		var temporary TemporaryError
		if !errors.As(err, &temporary) {
			return err
		}
		delay := time.Duration(1<<n)*150*time.Millisecond + time.Duration(rand.Intn(80))*time.Millisecond
		if waitErr := r.wait(ctx, delay); waitErr != nil {
			return waitErr
		}
	}
	return err
}
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
