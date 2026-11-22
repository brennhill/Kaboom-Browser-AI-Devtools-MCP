// doc.go -- Package lifecycle provides a typed event bus for capture lifecycle events.
// Why: Extracted from capture package to reduce god-object surface area.
// The Observer supports multiple listeners, unsubscribe by ID, and panic isolation.

package lifecycle
