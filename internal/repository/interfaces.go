// Package repository defines the data-access contracts the service
// layer depends on. Each *_interfaces.go file in this package owns one
// aggregate (or a small cluster of tightly-related aggregates) and
// declares the interface(s) the corresponding postgres adapter
// satisfies in internal/repository/postgres/.
//
// This file holds only the shared pagination primitives. Sentinel
// errors and aggregate-specific interfaces live alongside their
// aggregate (e.g. ErrCurrencyDuplicate is in
// gamification_interfaces.go, ErrPseudonymTaken is in
// course_interfaces.go).
package repository

// PaginationParams is the input shape every paginated list endpoint
// accepts. Page is 1-indexed; PerPage is capped by the handler layer.
type PaginationParams struct {
	Page    int
	PerPage int
}

// PaginatedResult wraps a list response with the count + window
// metadata the API serializes for pagination headers.
type PaginatedResult[T any] struct {
	Items      []T
	TotalCount int64
	Page       int
	PerPage    int
}
