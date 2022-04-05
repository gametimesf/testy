package testy

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrNoDB = errors.New("no DB set")
var ErrNotFound = errors.New("not found")

// DB is the interface for something which can save and retrieve test reports.
type DB interface {
	// Enumerate lists the test results for the given page. The datastore determines the page size.
	Enumerate(ctx context.Context, page int) (results []Summary, more bool, err error)
	// Load retrieves the specified test result from the datastore.
	// If the ID is invalid, ErrNotFound should be returned.
	Load(ctx context.Context, id string) (TestResult, error)
	// Save stores the provided TestResult in the data store and returns its unique ID.
	Save(context.Context, TestResult) (string, error)
}

type Summary struct {
	ID      string
	Started time.Time
	Dur     time.Duration
	Total   int
	Passed  int
}

// SetDB sets the datastore to use for test reports.
func SetDB(db DB) {
	instance.db = db
}

// SaveResult saves the provided result to the registered datastore. If no datastore has been registered, an error
// wrapping ErrNoDB is returned.
func SaveResult(ctx context.Context, tr TestResult) (string, error) {
	if instance.db == nil {
		return "", fmt.Errorf("%w", ErrNoDB)
	}

	return instance.db.Save(ctx, tr)
}

// LoadResult loads the specified result from the registered datastore. If no datastore has been registered, an error
// wrapping ErrNoDB is returned. If the ID is invalid, an error wrapping ErrNotFound is returned.
func LoadResult(ctx context.Context, id string) (TestResult, error) {
	if instance.db == nil {
		return TestResult{}, fmt.Errorf("%w", ErrNoDB)
	}

	return instance.db.Load(ctx, id)
}
