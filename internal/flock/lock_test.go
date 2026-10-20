package flock

// The lock is released and re-acquirable once the holder lets go.

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLockWaitsThenAcquires(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	unlock, err := Lock(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan error, 1)
	go func() {
		u, err := Lock(context.Background(), p)
		if err == nil {
			u()
		}
		got <- err
	}()
	time.Sleep(60 * time.Millisecond)
	select {
	case <-got:
		t.Fatal("second locker must wait while the lock is held")
	default:
	}
	unlock()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second locker never acquired the released lock")
	}
}

// A cancelled waiter returns the context's error instead of blocking.
func TestLockHonoursContext(t *testing.T) {
	p := filepath.Join(t.TempDir(), "y.lock")
	unlock, err := Lock(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if _, err := Lock(ctx, p); err == nil {
		t.Fatal("a cancelled wait must fail")
	}
}
