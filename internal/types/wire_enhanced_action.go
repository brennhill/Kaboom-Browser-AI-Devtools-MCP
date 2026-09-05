// Purpose: Defines canonical wire schema for enhanced user-action payload transport.
// Why: Keeps extension-to-daemon action serialization stable and versionable.
// Docs: docs/features/feature/session-to-test/index.md

package types

// WireAXLocator re-finds a target by accessibility semantics when its selector stops matching.
type WireAXLocator struct {
	Ref  string `json:"ref,omitempty"`
	Role string `json:"role,omitempty"`
	Name string `json:"name,omitempty"`
}

// WireViewportLocator re-finds a target by the point it occupied, in the frame that point belongs to.
type WireViewportLocator struct {
	X                int     `json:"x"`
	Y                int     `json:"y"`
	Width            int     `json:"width,omitempty"`
	Height           int     `json:"height,omitempty"`
	FrameURL         string  `json:"frame_url,omitempty"`
	ViewportWidth    int     `json:"viewport_width,omitempty"`
	ViewportHeight   int     `json:"viewport_height,omitempty"`
	DevicePixelRatio float64 `json:"device_pixel_ratio,omitempty"`
}

// WireClockPin records the clock and timezone a session held still.
type WireClockPin struct {
	EpochMs           int64  `json:"epoch_ms,omitempty"`
	TimezoneID        string `json:"timezone_id,omitempty"`
	VirtualTimePolicy string `json:"virtual_time_policy,omitempty"`
}

// WireGeoPin records the geolocation a session held still.
type WireGeoPin struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	AccuracyM float64 `json:"accuracy_m,omitempty"`
}

// WireViewportPin records the device metrics a session held still.
type WireViewportPin struct {
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"device_scale_factor,omitempty"`
	Mobile            bool    `json:"mobile,omitempty"`
}

// WireEnvironmentPin reports what a session pinned, so the emitted test states its dependencies.
type WireEnvironmentPin struct {
	Clock       *WireClockPin    `json:"clock,omitempty"`
	Geolocation *WireGeoPin      `json:"geolocation,omitempty"`
	Viewport    *WireViewportPin `json:"viewport,omitempty"`
	RandomSeed  string           `json:"random_seed,omitempty"`
	Unpinned    []string         `json:"unpinned,omitempty"`
}

// WireEnhancedAction is the canonical wire format for enhanced actions.
// Extension sends these fields; the Go daemon may add server-only enrichment.
type WireEnhancedAction struct {
	Type           string         `json:"type"`
	Timestamp      int64          `json:"timestamp"`
	URL            string         `json:"url,omitempty"`
	Selectors      map[string]any `json:"selectors,omitempty"` // any: multiple selector strategies with varying value types
	Value          string         `json:"value,omitempty"`
	InputType      string         `json:"input_type,omitempty"`
	Key            string         `json:"key,omitempty"`
	FromURL        string         `json:"from_url,omitempty"`
	ToURL          string         `json:"to_url,omitempty"`
	SelectedValue  string         `json:"selected_value,omitempty"`
	SelectedText   string         `json:"selected_text,omitempty"`
	ScrollY        int            `json:"scroll_y,omitempty"`
	TabID          int            `json:"tab_id,omitempty"`
	Classification string         `json:"classification,omitempty"`
	DurationMs     int            `json:"duration_ms,omitempty"`
	Role           string         `json:"role,omitempty"`
	// Second and third locators. A generated step carries all three so a re-render that
	// breaks the selector still leaves two independent ways to reach the same target.
	AX          *WireAXLocator       `json:"ax,omitempty"`
	Viewport    *WireViewportLocator `json:"viewport,omitempty"`
	Environment *WireEnvironmentPin  `json:"environment,omitempty"`
}
