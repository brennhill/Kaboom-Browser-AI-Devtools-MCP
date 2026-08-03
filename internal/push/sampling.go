// sampling.go — Builds MCP sampling/createMessage requests from push events.
package push

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"
)

// BuildSamplingRequest converts a PushEvent into an MCP sampling/createMessage request.
func BuildSamplingRequest(ev PushEvent) SamplingRequest {
	var messages []SamplingMessage

	// Build content based on event type
	switch ev.Type {
	case "screenshot":
		if ev.ScreenshotB64 != "" {
			messages = append(messages, SamplingMessage{
				Role: "user",
				Content: SamplingContent{
					Type:     "image",
					Data:     ev.ScreenshotB64,
					MimeType: "image/png",
				},
			})
		}
		text := "Screenshot pushed from browser"
		if ev.Note != "" {
			text = ev.Note
		}
		if ev.PageURL != "" {
			text += fmt.Sprintf("\nPage: %s", ev.PageURL)
		}
		messages = append(messages, SamplingMessage{
			Role:    "user",
			Content: SamplingContent{Type: "text", Text: text},
		})

	case "annotations":
		text := "Draw mode annotations from browser"
		if ev.AnnotSession != "" {
			text += fmt.Sprintf(" (session: %s)", ev.AnnotSession)
		}
		if ev.PageURL != "" {
			text += fmt.Sprintf("\nPage: %s", ev.PageURL)
		}
		if len(ev.Annotations) > 0 {
			text += fmt.Sprintf("\nAnnotations: %s", string(ev.Annotations))
		}
		messages = append(messages, SamplingMessage{
			Role:    "user",
			Content: SamplingContent{Type: "text", Text: text},
		})

	case "chat":
		text := ev.Message
		if ev.PageURL != "" {
			text += fmt.Sprintf("\n\n[Sent from: %s]", ev.PageURL)
		}
		messages = append(messages, SamplingMessage{
			Role:    "user",
			Content: SamplingContent{Type: "text", Text: text},
		})

	default:
		text := fmt.Sprintf("Push event (%s) from browser", ev.Type)
		if ev.PageURL != "" {
			text += fmt.Sprintf("\nPage: %s", ev.PageURL)
		}
		messages = append(messages, SamplingMessage{
			Role:    "user",
			Content: SamplingContent{Type: "text", Text: text},
		})
	}

	return SamplingRequest{
		JSONRPC: "2.0",
		ID:      randomID(),
		Method:  "sampling/createMessage",
		Params: SamplingParams{
			Messages:       messages,
			MaxTokens:      1024,
			SystemPrompt:   "The user is pushing content from their browser to your conversation. Acknowledge and respond helpfully.",
			IncludeContext: "thisServer",
		},
	}
}

func randomID() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UnixNano()
	}
	// #nosec G115 -- the sign bit mask proves the result is within math.MaxInt64.
	return int64(binary.BigEndian.Uint64(b[:]) & 0x7FFFFFFFFFFFFFFF)
}
