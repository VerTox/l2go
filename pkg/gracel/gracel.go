package gracel

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultWaitTime = 10 * time.Second
)

type Service interface {
	Run(ctx context.Context) error
}

type Gracel struct {
	m sync.Mutex

	closed bool

	service  Service
	waitTime time.Duration
}

func (g *Gracel) Run(ctx context.Context) error {
	errCh := make(chan error)

	defer func() {
		g.m.Lock()
		if !g.closed {
			close(errCh)
		}

		g.closed = true
		g.m.Unlock()
	}()

	go func() {
		err := g.service.Run(ctx)

		g.m.Lock()
		if !g.closed {
			errCh <- err
		}
		g.m.Unlock()
	}()

	select {
	case <-ctx.Done():
		select {
		case serviceErr := <-errCh:
			return serviceErr
		case <-time.After(g.waitTime):
			return errors.New("shutdown wait time is over")
		}
	case serviceErr := <-errCh:
		return serviceErr
	}
}

type Options struct {
	WaitTime time.Duration
}

func NewGracel(service Service, opts *Options) *Gracel {
	if opts == nil {
		opts = &Options{}
	}

	if opts.WaitTime == 0 {
		opts.WaitTime = defaultWaitTime
	}

	return &Gracel{
		service:  service,
		waitTime: opts.WaitTime,
	}
}
