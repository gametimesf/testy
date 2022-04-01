package testy

import (
	"context"
	"errors"
	"fmt"
)

var ErrNoDB = errors.New("no DB set")

// DB is the interface for something which can save and retrieve test reports.
type DB interface {
	// Save stores the provided TestResult in the data store and returns its unique ID.
	Save(context.Context, TestResult) (string, error)
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