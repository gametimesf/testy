package testy

import (
	"context"
	"fmt"
	"sync"
)

// CaseExecutor bounds admitted case lifecycles across all packages/runs sharing
// this instance. Share one per hosted worker, not one per package. It is not a
// cross-process limit. The constructor selects worker-wide or package exclusivity.
// A permit covers setup, body, ordered children, cleanup and AfterTest.
type CaseExecutor struct {
	gate    chan struct{}
	slots   chan struct{}
	budget  chan struct{}
	runGate chan struct{}

	mu           sync.Mutex
	packages     map[string]*CaseExecutor
	packageLimit int
}

func NewCaseExecutor(limit int) *CaseExecutor {
	if limit < 1 {
		limit = 1
	}
	return &CaseExecutor{gate: make(chan struct{}, 1), slots: make(chan struct{}, limit)}
}

// NewPackageCaseExecutor preserves overlap between packages while bounding the
// total admitted lifecycles to totalLimit. Within each package, at most
// packageLimit ConcurrentTest cases overlap; ordinary cases and hooks exclude
// other work in that package. Repeated invocations of the same package serialize
// through AfterPackage so their package-scoped setup/teardown cannot interleave.
// Limits below one normalize to one. This is not a cross-package resource lock:
// callers must not run packages with conflicting shared mutations concurrently.
func NewPackageCaseExecutor(totalLimit, packageLimit int) *CaseExecutor {
	if totalLimit < 1 {
		totalLimit = 1
	}
	if packageLimit < 1 {
		packageLimit = 1
	}
	if packageLimit > totalLimit {
		packageLimit = totalLimit
	}
	return &CaseExecutor{
		budget:       make(chan struct{}, totalLimit),
		packages:     make(map[string]*CaseExecutor),
		packageLimit: packageLimit,
	}
}

func (e *CaseExecutor) acquirePackage(ctx context.Context, pkg string) (*CaseExecutor, func(), error) {
	if e == nil || e.packages == nil {
		return e, func() {}, nil
	}
	e.mu.Lock()
	scope := e.packages[pkg]
	if scope == nil {
		scope = NewCaseExecutor(e.packageLimit)
		scope.budget = e.budget
		scope.runGate = make(chan struct{}, 1)
		e.packages[pkg] = scope
	}
	e.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	select {
	case scope.runGate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-scope.runGate
			return nil, nil, err
		}
		return scope, func() { <-scope.runGate }, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
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
		return nil, fmt.Errorf("testy: invalid case executor scope")
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
	budgetAcquired := false
	release := func() {
		if budgetAcquired {
			<-e.budget
		}
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
	// Take one worker permit only after package admission. An exclusive case
	// consumes the package slots, not the entire worker budget.
	if e.budget != nil {
		select {
		case e.budget <- struct{}{}:
			budgetAcquired = true
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
