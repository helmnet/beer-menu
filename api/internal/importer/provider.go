// Package importer is the provider-neutral core of batch import: the Provider boundary, the
// connection and its sealed key, the batch list cache, the rate gate, import, re-sync and the
// scheduled sync. It never sees a provider's field names or statuses (constitution II).
package importer

import (
	"context"
	"errors"
	"time"
)

// Provider reads batches from one connected account. Implementations translate every failure
// into the errors below and never put credentials or response bodies in them.
type Provider interface {
	// Check reports whether the credentials work for batch reads.
	Check(ctx context.Context) error
	// ListBatches returns every batch not archived, all pages.
	ListBatches(ctx context.Context) ([]Batch, error)
	// GetBatch returns one batch, archived or not, or ErrBatchNotFound.
	GetBatch(ctx context.Context, id string) (Batch, error)
}

// Factory builds a Provider for one account's credentials.
type Factory func(userID, apiKey string) Provider

// Batch is a provider batch mapped to beer terms. Values are raw and are cleaned by
// beers.Sanitize before any write.
type Batch struct {
	// Ref is the provider's permanent batch ID, stored as the beer's source_ref.
	Ref    string
	Name   string
	Number *int64
	// Status is one of the four beer statuses (planned, fermenting, conditioning, ready).
	Status string
	// Archived is true when the provider archived the batch; such batches are never imported.
	Archived bool
	Style    *string
	Notes    *string
	// BrewDate is YYYY-MM-DD in UTC.
	BrewDate *string
	ABV      *float64
	IBU      *float64
	SRM      *float64
}

// Errors a Provider returns. They name no credential and carry no response body.
var (
	// ErrBatchNotFound means the provider has no batch with that ID.
	ErrBatchNotFound = errors.New("batch not found")
	// ErrCredentialsRefused means the provider refused the user ID and key (HTTP 401).
	ErrCredentialsRefused = errors.New("provider refused the credentials")
	// ErrNoBatchAccess means the key lacks the scope to read batches (HTTP 403).
	ErrNoBatchAccess = errors.New("credentials cannot read batches")
	// ErrUnreachable means the provider timed out, failed, or sent an unreadable response.
	ErrUnreachable = errors.New("provider unreachable")
)

// RateLimitedError means the provider's rate limit is in force until Until.
type RateLimitedError struct{ Until time.Time }

// Error says the limit holds and when it lifts.
func (e *RateLimitedError) Error() string {
	return "provider rate limit until " + e.Until.UTC().Format(time.RFC3339)
}
