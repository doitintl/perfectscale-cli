package api

import "fmt"

const defaultPageCap = 50

// fetchAllPages walks a single opaque forward cursor until the server returns
// no next token, collecting every page's items into one slice.
//
// This is the pagination contract used by the newer public API endpoints
// (node-groups, unevictable-pods): a single direction-encoded pageToken, with
// a nil or empty-string next signaling the last page — unlike the older
// automation audit-logs endpoint's separate after/before/has_next shape (see
// automation.go).
//
// pageCap bounds the number of pages fetched as a safety net (set <=0 to use
// the default of 50).
func fetchAllPages[T any](pageCap int, fetch func(pageToken *string) ([]T, *string, error)) ([]T, error) {
	if pageCap <= 0 {
		pageCap = defaultPageCap
	}

	var (
		all   []T
		token *string
	)

	for page := 0; page < pageCap; page++ {
		items, next, err := fetch(token)
		if err != nil {
			return nil, err
		}

		all = append(all, items...)
		if next == nil || *next == "" {
			return all, nil
		}
		token = next
	}

	return all, fmt.Errorf("page cap %d reached; refine filters or pass a higher page cap to lift the limit", pageCap)
}
