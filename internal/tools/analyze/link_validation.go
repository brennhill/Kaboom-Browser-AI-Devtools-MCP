// Purpose: Validates batches of URLs concurrently via HEAD requests with SSRF-safe transport and timeout controls.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload/uploadsec"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// MaxLinkValidationURLs is the upper bound on URLs accepted per validation request.
const MaxLinkValidationURLs = 1000

// LinkValidationParams are parameters for verifying links server-side.
type LinkValidationParams struct {
	URLs       []string `json:"urls"`
	TimeoutMS  int      `json:"timeout_ms,omitempty"`
	MaxWorkers int      `json:"max_workers,omitempty"`
}

// LinkValidationResult contains server-side verification of a single link.
type LinkValidationResult struct {
	URL        string `json:"url"`
	Status     int    `json:"status"`
	Code       string `json:"code"`
	TimeMS     int    `json:"time_ms"`
	RedirectTo string `json:"redirect_to,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ClampInt clamps v to [min, max], using defaultVal if v is zero.
func ClampInt(v, defaultVal, min, max int) int {
	if v == 0 {
		v = defaultVal
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// FilterHTTPURLs returns only URLs with http:// or https:// prefix.
func FilterHTTPURLs(urls []string) []string {
	valid := make([]string, 0, len(urls))
	for _, u := range urls {
		if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
			valid = append(valid, u)
		}
	}
	return valid
}

// ClassifyHTTPStatus maps an HTTP status code to a link health category.
func ClassifyHTTPStatus(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "ok"
	case status >= 300 && status < 400:
		return "redirect"
	case status == 401 || status == 403:
		return "requires_auth"
	default:
		return "broken"
	}
}

// NewLinkValidationClient creates an HTTP client with SSRF-safe transport and a 5-redirect limit.
func NewLinkValidationClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: uploadsec.NewSSRFSafeTransport(nil),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// DoLinkRequest tries HEAD first, falling back to GET if HEAD fails or returns 405.
// The version parameter is used for the User-Agent header.
func DoLinkRequest(client *http.Client, linkURL string, version string) (*http.Response, error) {
	ua := fmt.Sprintf("Kaboom/%s (+https://kaboom.dev)", version)

	req, err := http.NewRequest("HEAD", linkURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req) // #nosec G704 -- client uses ssrfSafeTransport() to block private/internal targets

	if err != nil || (resp != nil && resp.StatusCode == http.StatusMethodNotAllowed) {
		if resp != nil {
			_ = resp.Body.Close() // lint:body-close-ok close HEAD response before GET retry
		}
		req, err = http.NewRequest("GET", linkURL, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}
		req.Header.Set("User-Agent", ua)
		resp, err = client.Do(req) // #nosec G704 -- client uses ssrfSafeTransport() to block private/internal targets
	}

	return resp, err
}

// BuildLinkResult drains the response body and builds a LinkValidationResult from a successful response.
func BuildLinkResult(resp *http.Response, url string, timeMS int) LinkValidationResult {
	defer func() { _ = resp.Body.Close() }()

	if _, drainErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20)); drainErr != nil {
		return LinkValidationResult{
			URL:    url,
			Status: resp.StatusCode,
			Code:   "broken",
			TimeMS: timeMS,
			Error:  "failed to read response body: " + drainErr.Error(),
		}
	}

	result := LinkValidationResult{
		URL:    url,
		Status: resp.StatusCode,
		Code:   ClassifyHTTPStatus(resp.StatusCode),
		TimeMS: timeMS,
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		result.RedirectTo = loc
	}
	return result
}

// ValidateSingleLinkWithClient performs HTTP verification of a single URL using a shared client.
// The version parameter is used for the User-Agent header.
func ValidateSingleLinkWithClient(client *http.Client, linkURL string, version string) LinkValidationResult {
	startTime := time.Now()

	resp, err := DoLinkRequest(client, linkURL, version)
	timeMS := int(time.Since(startTime).Milliseconds())

	if err != nil {
		return LinkValidationResult{
			URL:    linkURL,
			Status: 0,
			Code:   "broken",
			TimeMS: timeMS,
			Error:  err.Error(),
		}
	}

	return BuildLinkResult(resp, linkURL, timeMS)
}

// ValidateLinksServerSide performs HTTP HEAD/GET requests to verify link status.
// The version parameter is used for the User-Agent header.
func ValidateLinksServerSide(urls []string, timeoutMS int, maxWorkers int, version string) []LinkValidationResult {
	if len(urls) == 0 {
		return []LinkValidationResult{}
	}

	workerCount := maxWorkers
	if workerCount > len(urls) {
		workerCount = len(urls)
	}

	// Share one HTTP client across all workers (connection pooling)
	client := NewLinkValidationClient(time.Duration(timeoutMS) * time.Millisecond)

	results := make([]LinkValidationResult, len(urls))
	jobs := make(chan int)

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = ValidateSingleLinkWithClient(client, urls[idx], version)
			}
		})
	}

	for i := range urls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}
