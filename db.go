package testy

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/gametimesf/testy/internal/orderedmap"
)

// ErrNoDB indicates no DB has been set via SetDB.
var ErrNoDB = errors.New("no DB set")

// ErrNotFound indicates the provided result ID was not found in the datastore.
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

// Summary is an overview of a TestResult, used to populate the list of past results.
type Summary struct {
	// ID is an opaque unique identifier for a test result. The specific format is defined by the datastore.
	ID string
	// Started indicates when a test run was started.
	Started time.Time
	// Dur is how long the test run took to complete.
	Dur time.Duration
	// Total is the total number of tests that were run.
	Total int
	// Passed is the number of tests that passed.
	Passed int
	// Failed is the number of tests that failed.
	Failed int
}

// TruncatedTimestamp returns the started timestamp truncated to second precision.
func (s Summary) TruncatedTimestamp() time.Time {
	return s.Started.Truncate(time.Second)
}

// SetDB sets the datastore to use for test reports.
// This must be called during application startup.
func SetDB(db DB) {
	instance.db = db
}

// SaveResult saves the provided result to the registered datastore.
// If no datastore has been registered, an error wrapping ErrNoDB is returned.
func SaveResult(ctx context.Context, tr TestResult) (string, error) {
	if instance.db == nil {
		return "", fmt.Errorf("%w", ErrNoDB)
	}

	return instance.db.Save(ctx, tr)
}

// LoadResult loads the specified result from the registered datastore.
// If no datastore has been registered, an error wrapping ErrNoDB is returned.
// If the ID is invalid, an error wrapping ErrNotFound is returned.
func LoadResult(ctx context.Context, id string) (TestResult, error) {
	if instance.db == nil {
		return TestResult{}, fmt.Errorf("%w", ErrNoDB)
	}

	return instance.db.Load(ctx, id)
}

// InMemoryDB is an implementation of DB that is stored in memory, with no persistent storage.
// It should be used for demonstration purposes only.
type InMemoryDB struct {
	nextID int
	store  orderedmap.OrderedMap[string, TestResult]
}

var _ DB = (*InMemoryDB)(nil)

func (db *InMemoryDB) Enumerate(_ context.Context, _ int) (results []Summary, more bool, err error) {
	s := make([]Summary, 0, len(db.store))
	db.store.Iterate(func(id string, r TestResult) bool {
		total, passed, failed := r.SumTestStats()
		s = append(s, Summary{
			ID:      id,
			Started: r.Started,
			Dur:     r.Dur,
			Total:   total,
			Passed:  passed,
			Failed:  failed,
		})
		return true
	})
	return s, false, nil
}

func (db *InMemoryDB) Load(_ context.Context, id string) (TestResult, error) {
	if r, ok := db.store[id]; ok {
		return r, nil
	}
	return TestResult{}, fmt.Errorf("%w: %v", ErrNotFound, id)
}

func (db *InMemoryDB) Save(_ context.Context, result TestResult) (string, error) {
	if db.store == nil {
		db.store = make(orderedmap.OrderedMap[string, TestResult])
	}
	id := strconv.Itoa(db.nextID)
	db.nextID++
	db.store[id] = result
	return id, nil
}
