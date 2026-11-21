// screenshot.go — Transport-independent screenshot use case: validate, command the extension, handle the image.
// Why: The observe, interact and analyze paths each reached the screenshot workflow through MCP
// request/response types and the whole *capture.Capture; this package owns the workflow behind a
// small interface so MCP parsing and JSON-RPC assembly stay in the adapters.
// Docs: docs/features/feature/observe/index.md

// Package screenshot is the first transport-independent application use case.
// It must not import internal/mcp, internal/capture/**, net/http or cmd/**
// (enforced by scripts/contracts/layering).
package screenshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// DefaultTimeout bounds both queueing and the wait for the extension's reply.
const DefaultTimeout = 20 * time.Second

// Deps wires the use case to its collaborators.
type Deps struct {
	Commands        queries.QueryRunner
	TrackingEnabled func() bool
	// Timeout overrides DefaultTimeout when positive.
	Timeout time.Duration
}

// Options are the caller-chosen capture settings.
type Options struct {
	Format        string
	Quality       int
	FullPage      bool
	Selector      string
	WaitForStable bool
	SaveTo        string
}

// Result is a captured screenshot. Fields is the extension's reply minus
// data_url (plus save_to / save_to_error); DataURL carries the image itself.
type Result struct {
	Fields  map[string]any
	DataURL string
}

// Path returns the persisted screenshot file: the extension's absolute path,
// or its bare filename resolved under the daemon screenshots directory.
func (r Result) Path() string {
	if p, ok := r.Fields["path"].(string); ok && p != "" {
		return p
	}
	if f, ok := r.Fields["filename"].(string); ok && f != "" {
		if dir, err := state.ScreenshotsDir(); err == nil {
			return filepath.Join(dir, f)
		}
	}
	return ""
}

// Image splits DataURL into base64 payload and MIME type; both are empty when
// the capture carried no usable image.
func (r Result) Image() (base64Data, mimeType string) {
	return util.SplitDataURL(r.DataURL)
}

// FailureKind classifies why a capture failed so adapters can map it.
type FailureKind string

const (
	KindNotTracking     FailureKind = "not_tracking"
	KindInvalidFormat   FailureKind = "invalid_format"
	KindInvalidQuality  FailureKind = "invalid_quality"
	KindQueueFull       FailureKind = "queue_full"
	KindTimeout         FailureKind = "timeout"
	KindMalformedResult FailureKind = "malformed_result"
	KindExtensionError  FailureKind = "extension_error"
)

// Failure is the only error type Capture returns.
type Failure struct {
	Kind   FailureKind
	Detail string
}

func (f *Failure) Error() string { return string(f.Kind) + ": " + f.Detail }

func fail(kind FailureKind, detail string) *Failure { return &Failure{Kind: kind, Detail: detail} }

// Service runs the screenshot workflow.
type Service struct {
	deps Deps
}

// New builds the use case.
func New(deps Deps) *Service {
	if deps.Timeout <= 0 {
		deps.Timeout = DefaultTimeout
	}
	return &Service{deps: deps}
}

// Capture validates opts, asks the extension for a screenshot and applies save_to.
func (s *Service) Capture(ctx context.Context, opts Options) (Result, error) {
	if !s.deps.TrackingEnabled() {
		return Result{}, fail(KindNotTracking, "no tab is being tracked")
	}
	if err := validate(opts); err != nil {
		return Result{}, err
	}
	queryID, err := s.deps.Commands.Enqueue(queries.PendingQuery{Type: "screenshot", Params: queryParams(opts)}, s.deps.Timeout, "")
	if err != nil {
		return Result{}, fail(KindQueueFull, err.Error())
	}
	raw, err := s.deps.Commands.WaitQueryResult(ctx, queryID, s.deps.Timeout)
	if err != nil {
		return Result{}, fail(KindTimeout, err.Error())
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Result{}, fail(KindMalformedResult, err.Error())
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if errMsg, ok := fields["error"].(string); ok {
		return Result{}, fail(KindExtensionError, errMsg)
	}
	result := Result{Fields: fields}
	if du, ok := fields["data_url"].(string); ok && du != "" {
		result.DataURL = du
		delete(fields, "data_url")
	}
	applySaveTo(result, opts.SaveTo)
	return result, nil
}

func validate(opts Options) error {
	if opts.Format != "" && opts.Format != "png" && opts.Format != "jpeg" {
		return fail(KindInvalidFormat, opts.Format)
	}
	if opts.Quality != 0 && (opts.Quality < 1 || opts.Quality > 100) {
		return fail(KindInvalidQuality, fmt.Sprintf("%d", opts.Quality))
	}
	return nil
}

func queryParams(opts Options) json.RawMessage {
	params := map[string]any{}
	if opts.Format != "" {
		params["format"] = opts.Format
	}
	if opts.Quality > 0 {
		params["quality"] = opts.Quality
	}
	if opts.FullPage {
		params["full_page"] = true
	}
	if opts.Selector != "" {
		params["selector"] = opts.Selector
	}
	if opts.WaitForStable {
		params["wait_for_stable"] = true
	}
	// A map of strings, ints and bools always marshals.
	encoded, _ := json.Marshal(params)
	return encoded
}

// applySaveTo copies the image to a user-chosen path (#386); a failure is
// reported in the result rather than failing the capture.
func applySaveTo(result Result, saveTo string) {
	if saveTo == "" || result.DataURL == "" {
		return
	}
	if err := saveToPath(saveTo, result.DataURL); err != nil {
		result.Fields["save_to_error"] = err.Error()
		return
	}
	result.Fields["save_to"] = saveTo
}

// saveToPath writes the data URL to saveTo, creating parent directories. Only
// .png, .jpg and .jpeg extensions are accepted.
func saveToPath(saveTo, dataURL string) error {
	absPath, err := filepath.Abs(saveTo)
	if err != nil {
		return fmt.Errorf("invalid save_to path: %w", err)
	}
	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return fmt.Errorf("save_to must have .png, .jpg, or .jpeg extension, got %q", ext)
	}
	b64Data, _ := util.SplitDataURL(dataURL)
	if b64Data == "" {
		return fmt.Errorf("screenshot_save: invalid data URL format. Expected 'data:image/...;base64,...'")
	}
	imageData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return fmt.Errorf("failed to decode image data: %w", err)
	}
	dir := filepath.Dir(absPath)
	// #nosec G301 -- user-chosen directory for screenshot save
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	// #nosec G306 -- user-specified path for screenshot save
	if err := os.WriteFile(absPath, imageData, 0o644); err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}
	return nil
}
