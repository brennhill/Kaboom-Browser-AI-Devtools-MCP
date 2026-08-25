// correlate.go — Correlates local browser request context with local OpenTelemetry JSON spans.
// Docs: docs/features/feature/backend-trace-correlation/index.md

package tracecorr

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"
)

const maxTraceSourceBytes = 32 << 20

type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Name         string            `json:"name"`
	Service      string            `json:"service,omitempty"`
	Category     string            `json:"category"`
	DurationMs   float64           `json:"duration_ms"`
	Attributes   map[string]string `json:"-"`
	startNano    int64
	endNano      int64
}

type RequestCorrelation struct {
	URL        string             `json:"url"`
	RequestID  string             `json:"request_id,omitempty"`
	TraceID    string             `json:"trace_id,omitempty"`
	Status     string             `json:"status"`
	Spans      []Span             `json:"spans,omitempty"`
	Breakdown  map[string]float64 `json:"breakdown,omitempty"`
	DurationMs float64            `json:"duration_ms,omitempty"`
}

type Result struct {
	Status   string               `json:"status"`
	Source   string               `json:"source,omitempty"`
	Requests []RequestCorrelation `json:"requests,omitempty"`
	Error    string               `json:"error,omitempty"`
}

type sourceDocument struct {
	Spans []sourceSpan `json:"spans"`
}

type sourceSpan struct {
	TraceID           string            `json:"trace_id"`
	SpanID            string            `json:"span_id"`
	ParentSpanID      string            `json:"parent_span_id"`
	Name              string            `json:"name"`
	StartTimeUnixNano json.RawMessage   `json:"start_time_unix_nano"`
	EndTimeUnixNano   json.RawMessage   `json:"end_time_unix_nano"`
	Attributes        map[string]string `json:"attributes"`
}

type otlpDocument struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []otlpAttribute `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []struct {
				TraceID           string          `json:"traceId"`
				SpanID            string          `json:"spanId"`
				ParentSpanID      string          `json:"parentSpanId"`
				Name              string          `json:"name"`
				StartTimeUnixNano json.RawMessage `json:"startTimeUnixNano"`
				EndTimeUnixNano   json.RawMessage `json:"endTimeUnixNano"`
				Attributes        []otlpAttribute `json:"attributes"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}

type otlpAttribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string          `json:"stringValue"`
		IntValue    json.RawMessage `json:"intValue"`
		BoolValue   *bool           `json:"boolValue"`
		DoubleValue *float64        `json:"doubleValue"`
	} `json:"value"`
}

func CorrelateFile(path string, entries []types.NetworkWaterfallEntry) Result {
	if path == "" {
		return Result{Status: "not_configured"}
	}
	data, err := readBounded(path)
	if err != nil {
		return Result{Status: "source_error", Source: path, Error: err.Error()}
	}
	spans, err := parseSpans(data)
	if err != nil {
		return Result{Status: "source_error", Source: path, Error: bounded(err.Error(), 128)}
	}
	byTrace, byRequestID := indexSpans(spans)
	return correlateEntries(path, entries, byTrace, byRequestID)
}

func indexSpans(spans []Span) (map[string][]Span, map[string]map[string]struct{}) {
	byTrace := make(map[string][]Span)
	byRequestID := make(map[string]map[string]struct{})
	for _, span := range spans {
		traceID := normalizeHex(span.TraceID)
		byTrace[traceID] = append(byTrace[traceID], span)
		indexSpanRequestIDs(byRequestID, span, traceID)
	}
	return byTrace, byRequestID
}

func indexSpanRequestIDs(byRequestID map[string]map[string]struct{}, span Span, traceID string) {
	for _, key := range []string{"http.request_id", "request.id", "x-request-id"} {
		requestID := span.Attributes[key]
		if requestID == "" {
			continue
		}
		if byRequestID[requestID] == nil {
			byRequestID[requestID] = make(map[string]struct{})
		}
		byRequestID[requestID][traceID] = struct{}{}
	}
}

func correlateEntries(path string, entries []types.NetworkWaterfallEntry, byTrace map[string][]Span, byRequestID map[string]map[string]struct{}) Result {
	result := Result{Status: "no_matches", Source: path}
	for _, entry := range entries {
		request, aggregate := correlateRequest(entry, byTrace, byRequestID)
		if aggregate == "ambiguous" {
			result.Status = "ambiguous"
		} else if aggregate == "correlated" && result.Status != "ambiguous" {
			result.Status = "correlated"
		}
		result.Requests = append(result.Requests, request)
	}
	if len(entries) == 0 {
		result.Status = "no_trace_context"
	}
	return result
}

func correlateRequest(entry types.NetworkWaterfallEntry, byTrace map[string][]Span, byRequestID map[string]map[string]struct{}) (RequestCorrelation, string) {
	traceID := traceIDFromTraceparent(entry.Traceparent)
	request := RequestCorrelation{URL: entry.URL, RequestID: entry.RequestID, TraceID: traceID, Status: "unmatched"}
	if traceID == "" && entry.RequestID != "" {
		matches := byRequestID[entry.RequestID]
		if len(matches) > 1 {
			request.Status = "ambiguous"
			return request, "ambiguous"
		}
		for matchedTraceID := range matches {
			traceID = matchedTraceID
			request.TraceID = matchedTraceID
		}
	}
	if traceID == "" {
		if entry.RequestID == "" {
			request.Status = "trace_context_unavailable"
		}
		return request, ""
	}
	if matched := byTrace[traceID]; len(matched) > 0 {
		request.Status = "correlated"
		request.Spans = append([]Span(nil), matched...)
		request.Breakdown, request.DurationMs = summarize(matched)
		return request, "correlated"
	}
	return request, ""
}

func readBounded(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > maxTraceSourceBytes {
		return nil, &sourceError{message: "trace_source_too_large"}
	}
	return io.ReadAll(io.LimitReader(file, maxTraceSourceBytes+1))
}

type sourceError struct{ message string }

func (e *sourceError) Error() string { return e.message }

func parseSpans(data []byte) ([]Span, error) {
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(data, &shape); err != nil {
		return nil, err
	}
	_, hasCompact := shape["spans"]
	_, hasOTLP := shape["resourceSpans"]
	if !hasCompact && !hasOTLP {
		return nil, &sourceError{message: "unsupported_trace_schema"}
	}
	spans, err := parseCompactSpans(data)
	if err != nil {
		return nil, err
	}
	otlpSpans, err := parseOTLPSpans(data)
	if err != nil {
		return nil, err
	}
	return append(spans, otlpSpans...), nil
}

func parseCompactSpans(data []byte) ([]Span, error) {
	var document sourceDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	spans := make([]Span, 0, len(document.Spans))
	for _, raw := range document.Spans {
		start, end, ok := spanWindow(raw.StartTimeUnixNano, raw.EndTimeUnixNano, raw.TraceID, raw.SpanID, raw.ParentSpanID)
		if !ok {
			return nil, &sourceError{message: "invalid_trace_span"}
		}
		spans = append(spans, buildSpan(spanIdentity{traceID: raw.TraceID, spanID: raw.SpanID, parentSpanID: raw.ParentSpanID, name: raw.Name}, raw.Attributes, start, end))
	}
	return spans, nil
}

func parseOTLPSpans(data []byte) ([]Span, error) {
	var otlp otlpDocument
	if err := json.Unmarshal(data, &otlp); err != nil {
		return nil, err
	}
	var spans []Span
	for _, resourceSpans := range otlp.ResourceSpans {
		resourceAttributes := otlpAttributes(resourceSpans.Resource.Attributes)
		for _, scope := range resourceSpans.ScopeSpans {
			for _, raw := range scope.Spans {
				attributes := mergedAttributes(resourceAttributes, raw.Attributes)
				start, end, ok := spanWindow(raw.StartTimeUnixNano, raw.EndTimeUnixNano, raw.TraceID, raw.SpanID, raw.ParentSpanID)
				if !ok {
					return nil, &sourceError{message: "invalid_trace_span"}
				}
				spans = append(spans, buildSpan(spanIdentity{traceID: raw.TraceID, spanID: raw.SpanID, parentSpanID: raw.ParentSpanID, name: raw.Name}, attributes, start, end))
			}
		}
	}
	return spans, nil
}

func spanWindow(startRaw, endRaw json.RawMessage, traceID, spanID, parentSpanID string) (int64, int64, bool) {
	start, startOK := numericJSON(startRaw)
	end, endOK := numericJSON(endRaw)
	if !startOK || !endOK || !validHexID(traceID, 32) || !validHexID(spanID, 16) || (parentSpanID != "" && !validHexID(parentSpanID, 16)) || end < start {
		return 0, 0, false
	}
	return start, end, true
}

func mergedAttributes(resource map[string]string, spanAttributes []otlpAttribute) map[string]string {
	attributes := cloneAttributes(resource)
	for key, value := range otlpAttributes(spanAttributes) {
		attributes[key] = value
	}
	return attributes
}

// spanIdentity carries the identity and name fields shared by the compact and
// OTLP span sources.
type spanIdentity struct {
	traceID, spanID, parentSpanID, name string
}

func buildSpan(id spanIdentity, attributes map[string]string, start, end int64) Span {
	return Span{
		TraceID: normalizeHex(id.traceID), SpanID: normalizeHex(id.spanID), ParentSpanID: normalizeHex(id.parentSpanID),
		Name: bounded(id.name, 256), Service: bounded(attributes["service.name"], 128),
		Category: classifySpan(id.name, attributes), DurationMs: float64(end-start) / 1_000_000, Attributes: attributes,
		startNano: start, endNano: end,
	}
}

func otlpAttributes(values []otlpAttribute) map[string]string {
	attributes := make(map[string]string, len(values))
	for _, attribute := range values {
		value := attribute.Value.StringValue
		if value == "" && len(attribute.Value.IntValue) > 0 {
			value = strings.Trim(string(attribute.Value.IntValue), `"`)
		}
		if value == "" && attribute.Value.BoolValue != nil {
			value = strconv.FormatBool(*attribute.Value.BoolValue)
		}
		if value == "" && attribute.Value.DoubleValue != nil {
			value = strconv.FormatFloat(*attribute.Value.DoubleValue, 'g', -1, 64)
		}
		if attribute.Key != "" && value != "" {
			attributes[bounded(attribute.Key, 128)] = bounded(value, 256)
		}
	}
	return attributes
}

func cloneAttributes(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func summarize(spans []Span) (map[string]float64, float64) {
	breakdown := make(map[string]float64)
	var total float64
	for _, span := range spans {
		exclusive := span.DurationMs - childIntervalDuration(span, spans)
		if exclusive < 0 {
			exclusive = 0
		}
		breakdown[span.Category] += exclusive
		if span.ParentSpanID == "" && span.DurationMs > total {
			total = span.DurationMs
		}
	}
	return breakdown, total
}

func childIntervalDuration(parent Span, spans []Span) float64 {
	type interval struct{ start, end int64 }
	intervals := make([]interval, 0)
	for _, span := range spans {
		if span.ParentSpanID != parent.SpanID {
			continue
		}
		start, end := span.startNano, span.endNano
		if start < parent.startNano {
			start = parent.startNano
		}
		if end > parent.endNano {
			end = parent.endNano
		}
		if end > start {
			intervals = append(intervals, interval{start, end})
		}
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].start < intervals[j].start })
	var covered int64
	for index := 0; index < len(intervals); {
		start, end := intervals[index].start, intervals[index].end
		index++
		for index < len(intervals) && intervals[index].start <= end {
			if intervals[index].end > end {
				end = intervals[index].end
			}
			index++
		}
		covered += end - start
	}
	return float64(covered) / 1_000_000
}

func classifySpan(name string, attributes map[string]string) string {
	combined := strings.ToLower(name + " " + attributes["service.name"] + " " + attributes["db.system"] + " " + attributes["rpc.system"])
	for _, category := range []string{"redis", "sql", "auth", "edge", "external"} {
		if strings.Contains(combined, category) || (category == "sql" && attributes["db.system"] != "" && attributes["db.system"] != "redis") {
			return category
		}
	}
	return "application"
}

func traceIDFromTraceparent(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 || len(parts[1]) != 32 {
		return ""
	}
	return normalizeHex(parts[1])
}

func numericJSON(value json.RawMessage) (int64, bool) {
	raw := strings.Trim(string(value), `"`)
	number, err := strconv.ParseInt(raw, 10, 64)
	return number, err == nil
}

func validHexID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func normalizeHex(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func bounded(value string, max int) string {
	if len(value) > max {
		return value[:max]
	}
	return value
}
