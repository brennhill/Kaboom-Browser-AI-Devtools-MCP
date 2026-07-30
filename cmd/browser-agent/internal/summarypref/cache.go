// cache.go — Caches the session response-mode preference and applies it to tool arguments.

package summarypref

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/statediag"
)

type Loader func() ([]byte, error)

type Cache struct {
	mu     sync.RWMutex
	load   Loader
	value  bool
	loaded bool
	report statediag.Reporter
}

func New(load Loader, report statediag.Reporter) *Cache {
	return &Cache{load: load, report: report}
}

func (c *Cache) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	if c.loaded {
		value := c.value
		c.mu.RUnlock()
		return value
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return c.value
	}

	c.loaded = true
	c.value = false
	if c.load == nil {
		return false
	}
	data, err := c.load()
	if err != nil {
		if !errors.Is(err, statediag.ErrAbsent) {
			c.reportRecovery("Saved response-mode preference could not be read; full responses are active.")
		}
		return false
	}
	if len(data) == 0 {
		return false
	}
	var pref struct {
		Summary bool `json:"summary"`
	}
	if json.Unmarshal(data, &pref) != nil {
		c.reportRecovery("Saved response-mode preference was malformed; full responses are active.")
		return false
	}
	c.value = pref.Summary
	return c.value
}

func (c *Cache) reportRecovery(detail string) {
	if c.report == nil {
		return
	}
	c.report.Report(statediag.Diagnostic{
		Name:   "response_mode_state",
		Detail: detail,
		Fix:    "Save the response mode again with configure(what='store', namespace='session', key='response_mode').",
	})
}

func (c *Cache) Invalidate() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loaded = false
	c.value = false
}

func (c *Cache) Inject(args json.RawMessage) json.RawMessage {
	if c == nil {
		return args
	}
	if !c.Enabled() {
		return args
	}
	if len(args) == 0 || string(args) == "null" {
		return json.RawMessage(`{"summary":true}`)
	}

	var values map[string]json.RawMessage
	if json.Unmarshal(args, &values) != nil {
		return args
	}
	if _, exists := values["summary"]; exists {
		return args
	}
	if _, exists := values["full"]; exists {
		return args
	}
	if values == nil {
		values = make(map[string]json.RawMessage)
	}
	values["summary"] = json.RawMessage(`true`)
	result, _ := json.Marshal(values)
	return result
}
