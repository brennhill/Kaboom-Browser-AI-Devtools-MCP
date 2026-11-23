// Purpose: Reads MCP messages from stdin supporting both line-delimited JSON and Content-Length framing.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// StdioFraming indicates how an MCP message was framed on stdin.
type StdioFraming int

const (
	// StdioFramingLine is a raw JSON line (legacy framing).
	StdioFramingLine StdioFraming = iota
	// StdioFramingContentLength is MCP Content-Length framing.
	StdioFramingContentLength
)

// ReadStdioMessageWithMode reads one MCP message and returns the detected framing mode.
// Supports both line-delimited JSON and Content-Length framed messages.
func ReadStdioMessageWithMode(reader *bufio.Reader, maxBodySize int) ([]byte, StdioFraming, error) {
	for {
		firstLineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				trimmed := strings.TrimSpace(string(firstLineBytes))
				if trimmed == "" {
					return nil, StdioFramingLine, io.EOF
				}
				return []byte(trimmed), StdioFramingLine, nil
			}
			return nil, StdioFramingLine, err
		}

		firstLine := strings.TrimSpace(string(firstLineBytes))
		if firstLine == "" {
			continue
		}

		if !isHeaderLine(firstLine) {
			return []byte(firstLine), StdioFramingLine, nil
		}

		headers := []string{firstLine}
		// Consume remaining headers until blank line.
		for {
			headerLine, headerErr := reader.ReadBytes('\n')
			if headerErr != nil {
				if errors.Is(headerErr, io.EOF) {
					return nil, StdioFramingContentLength, io.EOF
				}
				return nil, StdioFramingContentLength, headerErr
			}
			trimmedHeader := strings.TrimSpace(string(headerLine))
			if trimmedHeader == "" {
				break
			}
			headers = append(headers, trimmedHeader)
		}

		contentLength, found := parseContentLength(headers, maxBodySize)
		if !found {
			return []byte(firstLine), StdioFramingLine, nil
		}

		payload := make([]byte, contentLength)
		if _, readErr := io.ReadFull(reader, payload); readErr != nil {
			return nil, StdioFramingContentLength, readErr
		}
		return bytes.TrimSpace(payload), StdioFramingContentLength, nil
	}
}

func isHeaderLine(line string) bool {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return false
	}
	key := strings.TrimSpace(line[:idx])
	if key == "" {
		return false
	}
	for _, r := range key {
		if r == '-' {
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func parseContentLength(headers []string, maxBodySize int) (int, bool) {
	for _, header := range headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "content-length") {
			continue
		}
		contentLength, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || contentLength < 0 || contentLength > maxBodySize {
			return 0, false
		}
		return contentLength, true
	}
	return 0, false
}
