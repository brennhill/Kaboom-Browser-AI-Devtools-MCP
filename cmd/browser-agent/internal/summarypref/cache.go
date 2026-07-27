// cache.go — Caches the session response-mode preference and applies it to tool arguments.

package summarypref

import (
	"encoding/json"
	"sync"
)

type Loader func() ([]byte, error)

type Cache struct {
	mu     sync.RWMutex
	load   Loader
	value  bool
	loaded bool
}

func New(load Loader) *Cache {
	return &Cache{load: load}
}

func (c *Cache) Enabled() bool {
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
	if err != nil || len(data) == 0 {
		return false
	}
	var pref struct {
		Summary bool `json:"summary"`
	}
	if json.Unmarshal(data, &pref) != nil {
		return false
	}
	c.value = pref.Summary
	return c.value
}

func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loaded = false
	c.value = false
}

func (c *Cache) Inject(args json.RawMessage) json.RawMessage {
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
