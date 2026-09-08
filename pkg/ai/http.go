package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const httpTimeout = 15 * time.Minute

// doJSONPost marshals reqBody to JSON, POSTs it to the given URL with the
// supplied headers, reads the response, checks the status code, and unmarshals
// the response body into respPtr.
//
// On non-200 status codes it returns an error containing the status code and
// the raw response body. Provider-specific error fields inside the JSON body
// are NOT checked here — each engine handles those after doJSONPost returns.
func doJSONPost(ctx context.Context, url string, headers map[string]string, reqBody any, respPtr any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if err := json.Unmarshal(respBody, respPtr); err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return nil
}
