// Purpose: Converts network body and waterfall entries into HAR 1.2 entry structures.
// Why: Separates entry-level conversion from HAR document assembly and file I/O.
package har

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/types"

// networkBodyToHAREntry converts a single NetworkBody to a HAR entry.
func networkBodyToHAREntry(body types.NetworkBody) harEntry {
	return harEntry{
		StartedDateTime: body.Timestamp,
		Time:            body.Duration,
		Request:         buildHARRequest(body),
		Response:        buildHARResponse(body),
		Timings: harTimings{
			Send:    -1,
			Wait:    body.Duration,
			Receive: -1,
		},
	}
}

func buildHARRequest(body types.NetworkBody) harRequest {
	req := harRequest{
		Method:      body.Method,
		URL:         body.URL,
		HTTPVersion: "HTTP/1.1",
		Headers:     make([]harNameValue, 0),
		QueryString: parseQueryString(body.URL),
		HeadersSize: -1,
		BodySize:    0,
	}

	if body.RequestBody != "" {
		req.PostData = &harPostData{
			MimeType: body.ContentType,
			Text:     body.RequestBody,
		}
		req.BodySize = len(body.RequestBody)
	}

	if body.RequestTruncated {
		req.Comment = "Body truncated at 8KB by Kaboom Agentic Browser"
	}

	return req
}

func buildHARResponse(body types.NetworkBody) harResponse {
	headers := make([]harNameValue, 0, len(body.ResponseHeaders))
	for name, value := range body.ResponseHeaders {
		headers = append(headers, harNameValue{Name: name, Value: value})
	}

	resp := harResponse{
		Status:      body.Status,
		StatusText:  httpStatusText(body.Status),
		HTTPVersion: "HTTP/1.1",
		Headers:     headers,
		Content: harContent{
			Size:     len(body.ResponseBody),
			MimeType: body.ContentType,
			Text:     body.ResponseBody,
		},
		HeadersSize: -1,
		BodySize:    len(body.ResponseBody),
	}

	if body.ResponseTruncated {
		resp.Comment = "Body truncated at 16KB by Kaboom Agentic Browser"
	}

	return resp
}

// waterfallToHAREntry converts a waterfall entry to a lightweight HAR entry.
func waterfallToHAREntry(wf types.NetworkWaterfallEntry) harEntry {
	durationMs := int(wf.Duration)
	sendMs, waitMs, receiveMs := computeWaterfallTimings(wf)

	return harEntry{
		StartedDateTime: wf.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		Time:            durationMs,
		Request: harRequest{
			Method:      "GET",
			URL:         wf.URL,
			HTTPVersion: "HTTP/1.1",
			Headers:     make([]harNameValue, 0),
			QueryString: parseQueryString(wf.URL),
			HeadersSize: -1,
			BodySize:    0,
		},
		Response: harResponse{
			Status:      0,
			StatusText:  "",
			HTTPVersion: "HTTP/1.1",
			Headers:     make([]harNameValue, 0),
			Content: harContent{
				Size:     wf.DecodedBodySize,
				MimeType: "",
			},
			HeadersSize: -1,
			BodySize:    wf.EncodedBodySize,
		},
		Timings: harTimings{
			Send:    sendMs,
			Wait:    waitMs,
			Receive: receiveMs,
		},
		Comment: "From resource timing (no body captured)",
	}
}

// enrichTimingsFromWaterfall replaces -1 timing values with computed values from waterfall data.
func enrichTimingsFromWaterfall(entry *harEntry, wf types.NetworkWaterfallEntry) {
	sendMs, waitMs, receiveMs := computeWaterfallTimings(wf)
	if entry.Timings.Send == -1 {
		entry.Timings.Send = sendMs
	}
	if entry.Timings.Receive == -1 {
		entry.Timings.Receive = receiveMs
	}
	// Update wait only if it was the default (entire duration).
	if entry.Timings.Wait == entry.Time && waitMs >= 0 {
		entry.Timings.Wait = waitMs
	}
}

// computeWaterfallTimings derives send/wait/receive from PerformanceResourceTiming fields.
// All values are in milliseconds. Returns -1 for phases that can't be computed.
func computeWaterfallTimings(wf types.NetworkWaterfallEntry) (send, wait, receive int) {
	if wf.FetchStart <= 0 || wf.ResponseEnd <= 0 {
		return -1, int(wf.Duration), -1
	}

	// send = fetchStart - startTime (DNS + connection setup)
	sendF := wf.FetchStart - wf.StartTime
	if sendF < 0 {
		sendF = 0
	}

	// receive = responseEnd - fetchStart - (total - duration would be wait)
	// Simplified: total = send + wait + receive, so wait = duration - send - receive
	// We estimate receive as a fraction, but more accurately:
	// send phase = fetchStart - startTime
	// the rest (fetchStart to responseEnd) = wait + receive
	// Without responseStart, we can't split wait/receive precisely.
	// Use: wait = most of the remaining, receive = 0
	remainF := wf.ResponseEnd - wf.FetchStart
	if remainF < 0 {
		remainF = 0
	}

	send = int(sendF)
	wait = int(remainF)
	receive = 0
	return send, wait, receive
}
