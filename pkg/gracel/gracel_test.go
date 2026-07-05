package gracel

import (
	"context"
	"errors"
	"testing"
	"time"
)

// serviceFunc adapts a plain function to the Service interface.
type serviceFunc func(ctx context.Context) error

func (f serviceFunc) Run(ctx context.Context) error { return f(ctx) }

func TestNewGracelWaitTime(t *testing.T) {
	svc := serviceFunc(func(context.Context) error { return nil })

	if g := NewGracel(svc, nil); g.waitTime != defaultWaitTime {
		t.Errorf("nil opts: waitTime = %v, want %v", g.waitTime, defaultWaitTime)
	}
	if g := NewGracel(svc, &Options{}); g.waitTime != defaultWaitTime {
		t.Errorf("zero WaitTime: waitTime = %v, want %v", g.waitTime, defaultWaitTime)
	}
	if g := NewGracel(svc, &Options{WaitTime: 3 * time.Second}); g.waitTime != 3*time.Second {
		t.Errorf("custom WaitTime = %v, want 3s", g.waitTime)
	}
}

func TestGracelRunServiceReturns(t *testing.T) {
	// Service finishes on its own (ctx never cancelled) → Run returns its result.
	if err := NewGracel(serviceFunc(func(context.Context) error { return nil }), nil).
		Run(context.Background()); err != nil {
		t.Errorf("Run = %v, want nil", err)
	}

	wantErr := errors.New("boom")
	if err := NewGracel(serviceFunc(func(context.Context) error { return wantErr }), nil).
		Run(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("Run = %v, want %v", err, wantErr)
	}
}

func TestGracelRunGracefulShutdown(t *testing.T) {
	// ctx cancelled and the service stops within the wait window → its error wins.
	wantErr := errors.New("clean stop")
	svc := serviceFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return wantErr
	})
	g := NewGracel(svc, &Options{WaitTime: time.Second})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Run(ctx); !errors.Is(err, wantErr) {
		t.Errorf("Run = %v, want %v", err, wantErr)
	}
}

func TestGracelRunWaitTimeout(t *testing.T) {
	// ctx cancelled but the service doesn't stop in time → timeout error.
	svc := serviceFunc(func(ctx context.Context) error {
		<-ctx.Done()
		time.Sleep(60 * time.Millisecond) // outlives the wait window
		return nil
	})
	g := NewGracel(svc, &Options{WaitTime: 10 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := g.Run(ctx); err == nil || err.Error() != "shutdown wait time is over" {
		t.Errorf("Run = %v, want shutdown-timeout error", err)
	}
}
