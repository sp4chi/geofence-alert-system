package httpapi

import (
	"fmt"
	"net/url"
	"strconv"

	"geofencing/internal/store"
)

const maxPageLimit = 500

// parsePage reads the optional ?limit=&offset= query parameters shared by every list endpoint.
// Without limit, defaultLimit applies (0 = return everything, as the README's list endpoints do).
// A limit above 500 is clamped to 500 rather than rejected (documented decision).
func parsePage(query url.Values, defaultLimit int) (store.Page, []string) {
	var problems []string
	page := store.Page{Limit: defaultLimit}

	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		switch {
		case err != nil || limit < 1:
			problems = append(problems, fmt.Sprintf("limit %q must be a positive integer (max %d)", raw, maxPageLimit))
		case limit > maxPageLimit:
			page.Limit = maxPageLimit
		default:
			page.Limit = limit
		}
	}

	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			problems = append(problems, fmt.Sprintf("offset %q must be a non-negative integer", raw))
		} else {
			page.Offset = offset
		}
	}
	return page, problems
}

// totalIfPaginated returns a pointer for an omitempty "total_count" field: present only when the
// client asked for a page, so unpaginated responses keep exactly the README's shape.
func totalIfPaginated(page store.Page, total int) *int {
	if !page.Paginated() {
		return nil
	}
	return &total
}
