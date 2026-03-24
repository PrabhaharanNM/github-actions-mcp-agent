package agents

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	httpTimeout = 30 * time.Second
	maxRetries  = 3
)

// doRequest performs an HTTP GET with a single custom header, retrying up to
// maxRetries times with exponential backoff on transient failures.
func doRequest(ctx context.Context, url, headerName, headerValue string) ([]byte, error) {
	client := &http.Client{Timeout: httpTimeout}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request for %s: %w", url, err)
		}
		if headerName != "" {
			req.Header.Set(headerName, headerValue)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("GET %s: %w", url, err)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("reading response from %s: %w", url, readErr)
			continue
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("GET %s returned %d: %s", url, resp.StatusCode, truncate(body, 256))
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("GET %s returned %d: %s", url, resp.StatusCode, truncate(body, 256))
		}

		return body, nil
	}

	return nil, fmt.Errorf("GET %s failed after %d retries: %w", url, maxRetries, lastErr)
}

// basicAuthValue returns a base64-encoded "Basic" Authorization header value.
func basicAuthValue(username, password string) string {
	creds := username + ":" + password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

// truncate returns the first n bytes of body as a string for error messages.
func truncate(body []byte, n int) string {
	if len(body) <= n {
		return string(body)
	}
	return string(body[:n]) + "..."
}
