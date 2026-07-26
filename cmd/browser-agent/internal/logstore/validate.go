// validate.go — Log-entry validation rules to enforce ingest contract bounds.
// Why: Keeps validation policy separate from persistence and async logging mechanics.

package logstore

import "encoding/json"

// validLevels defines accepted log level values.
var validLevels = map[string]bool{
	"error": true,
	"warn":  true,
	"info":  true,
	"debug": true,
	"log":   true,
}

// MaxEntrySize is the maximum serialized size of a single log entry (1MB).
const MaxEntrySize = 1024 * 1024

// ValidateEntry checks if a log entry meets the contract requirements.
// Returns true if the entry is valid, false otherwise.
func ValidateEntry(entry Entry) bool {
	// Required: level field must exist and be a known value
	level, ok := entry["level"].(string)
	if !ok || !validLevels[level] {
		return false
	}

	// Fast path: if total string content is under half the limit,
	// the entry can't exceed MaxEntrySize even with JSON escaping overhead
	var stringBytes int
	for _, v := range entry {
		if s, ok := v.(string); ok {
			stringBytes += len(s)
		}
	}
	if stringBytes < MaxEntrySize/2 {
		return true
	}

	// Slow path: might be large — check precisely via marshal
	data, err := json.Marshal(entry)
	if err != nil {
		return false
	}
	return len(data) <= MaxEntrySize
}

// ValidateEntries filters entries, returning only valid ones and a count of rejected.
func ValidateEntries(entries []Entry) (valid []Entry, rejected int) {
	valid = make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if ValidateEntry(entry) {
			valid = append(valid, entry)
		} else {
			rejected++
		}
	}
	return valid, rejected
}
