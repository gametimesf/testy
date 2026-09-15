package testy

import (
	"context"
	"fmt"
)

// CaseExecutor bounds admitted case lifecycles across all packages/runs sharing
// this instance. Share one per hosted worker, not one per package. It is not a
// cross-process limit. Non-ConcurrentTest cases and package hooks are exclusive.
// A permit covers setup, body, ordered children, cleanup and AfterTest.
type CaseExecutor struct {
	gate  chan struct{}
	slots chan struct{}
}

func NewCaseExecutor(limit int) *CaseExecutor {
	if limit < 1 {
		limit = 1
	}
	return &CaseExecutor{gate: make(chan struct{}, 1), slots: make(chan struct{}, limit)}
}

// Serialize admission, not execution, so two exclusive waiters cannot each hold
// half the permits and deadlock. Cancellation rolls back partial acquisition.
func (e *CaseExecutor) acquire(ctx context.Context, concurrent bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e == nil {
		return func() {}, nil
	}
	if cap(e.slots) == 0 || cap(e.gate) != 1 {
		return nil, fmt.Errorf("testy: CaseExecutor must be constructed with NewCaseExecutor")
	}
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-e.gate }()
	count := 1
	if !concurrent {
		count = cap(e.slots)
	}
	acquired := 0
	release := func() {
		for i := 0; i < acquired; i++ {
			<-e.slots
		}
	}
	for acquired < count {
		select {
		case e.slots <- struct{}{}:
			acquired++
		case <-ctx.Done():
			release()
			return nil, ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	return release, nil
}
