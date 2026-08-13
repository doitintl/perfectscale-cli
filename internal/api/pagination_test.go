package api

import (
	"errors"
	"testing"
)

func TestFetchAllPagesSinglePage(t *testing.T) {
	calls := 0
	items, err := fetchAllPages(0, func(pageToken *string) ([]int, *string, error) {
		calls++
		if pageToken != nil {
			t.Fatalf("pageToken = %v, want nil on first call", *pageToken)
		}
		return []int{1, 2, 3}, nil, nil
	})
	if err != nil {
		t.Fatalf("fetchAllPages() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if got := len(items); got != 3 {
		t.Fatalf("len(items) = %d, want 3", got)
	}
}

func TestFetchAllPagesEmpty(t *testing.T) {
	items, err := fetchAllPages(0, func(pageToken *string) ([]int, *string, error) {
		return nil, nil, nil
	})
	if err != nil {
		t.Fatalf("fetchAllPages() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func TestFetchAllPagesMultiPageStitching(t *testing.T) {
	pages := [][]int{{1, 2}, {3, 4}, {5}}
	tokens := []*string{strPtr("page-2"), strPtr("page-3"), nil}

	var seenTokens []*string
	items, err := fetchAllPages(0, func(pageToken *string) ([]int, *string, error) {
		seenTokens = append(seenTokens, pageToken)
		idx := len(seenTokens) - 1
		return pages[idx], tokens[idx], nil
	})
	if err != nil {
		t.Fatalf("fetchAllPages() error = %v", err)
	}
	if got := len(items); got != 5 {
		t.Fatalf("len(items) = %d, want 5", got)
	}
	for i, item := range items {
		if item != i+1 {
			t.Fatalf("items[%d] = %d, want %d", i, item, i+1)
		}
	}
	if seenTokens[0] != nil {
		t.Fatalf("seenTokens[0] = %v, want nil", seenTokens[0])
	}
	if seenTokens[1] == nil || *seenTokens[1] != "page-2" {
		t.Fatalf("seenTokens[1] = %v, want page-2", seenTokens[1])
	}
	if seenTokens[2] == nil || *seenTokens[2] != "page-3" {
		t.Fatalf("seenTokens[2] = %v, want page-3", seenTokens[2])
	}
}

func TestFetchAllPagesStopsOnEmptyStringCursor(t *testing.T) {
	calls := 0
	items, err := fetchAllPages(5, func(pageToken *string) ([]int, *string, error) {
		calls++
		empty := ""
		return []int{calls}, &empty, nil
	})
	if err != nil {
		t.Fatalf("fetchAllPages() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (empty-string next must stop, not loop)", calls)
	}
	if got := len(items); got != 1 {
		t.Fatalf("len(items) = %d, want 1", got)
	}
}

func TestFetchAllPagesCapExhausted(t *testing.T) {
	calls := 0
	_, err := fetchAllPages(2, func(pageToken *string) ([]int, *string, error) {
		calls++
		next := "more"
		return []int{calls}, &next, nil
	})
	if err == nil {
		t.Fatal("fetchAllPages() error = nil, want page cap error")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (bounded by pageCap)", calls)
	}
}

func TestFetchAllPagesPropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := fetchAllPages(0, func(pageToken *string) ([]int, *string, error) {
		return nil, nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

func strPtr(value string) *string {
	return &value
}
