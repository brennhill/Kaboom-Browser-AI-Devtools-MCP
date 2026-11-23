// cursor.go — Shared cursor pagination for observe modes that list buffered entries newest-first.
// Docs: docs/features/feature/observe/index.md

package core

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/observation/pagination"
)

// CursorArgs are the cursor parameters every paginated observe mode accepts.
type CursorArgs struct {
	AfterCursor       string `json:"after_cursor"`
	BeforeCursor      string `json:"before_cursor"`
	SinceCursor       string `json:"since_cursor"`
	RestartOnEviction bool   `json:"restart_on_eviction"`
}

// PageNewestFirst applies cursor pagination to oldest-first entries and returns
// the page newest-first, matching how these modes list entries without a cursor.
func PageNewestFirst[T pagination.Sequenced](entries []T, args CursorArgs, limit int) ([]T, *pagination.CursorPaginationMetadata, error) {
	page, meta, err := pagination.ApplyCursorPagination(entries, pagination.CursorParams{
		AfterCursor: args.AfterCursor, BeforeCursor: args.BeforeCursor, SinceCursor: args.SinceCursor,
		Limit: limit, RestartOnEviction: args.RestartOnEviction,
	})
	if err != nil {
		return nil, nil, err
	}
	for i, j := 0, len(page)-1; i < j; i, j = i+1, j-1 {
		page[i], page[j] = page[j], page[i]
	}
	return page, meta, nil
}
