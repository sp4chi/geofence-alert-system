package store

// Page is an optional slice of a list. Limit 0 means "everything": the README's list endpoints
// have no paging, so callers that don't ask for it still get the full list, as the contract says.
// Queries use `LIMIT NULLIF(limit, 0)`; Postgres treats LIMIT NULL as no limit.
type Page struct {
	Limit  int
	Offset int
}

// Paginated reports whether the caller asked for a slice (and therefore wants a total count).
func (p Page) Paginated() bool {
	return p.Limit > 0 || p.Offset > 0
}
