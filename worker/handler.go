package worker

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultMaxRetries    = 3
	defaultRetryDelay    = 100 * time.Millisecond
	serverErrorThreshold = 500
)

// Handler defines the interface for handling URL requests
type Handler interface {
	Handle(url string) error
}

// DefaultHandler implements Handler with retry logic for HTTP requests
type DefaultHandler struct {
	httpClient *http.Client
}

// NewDefaultHandler creates a new DefaultHandler
func NewDefaultHandler() *DefaultHandler {
	return &DefaultHandler{
		httpClient: &http.Client{},
	}
}

// Handle fetches data from the given URL
func (h *DefaultHandler) Handle(url string) error {
	return h.getData(url)
}

func (h *DefaultHandler) getData(url string) error {
	resp, err := retryableAPICall(func() (*http.Response, error) {
		return h.httpClient.Get(url)
	}, defaultMaxRetries, h.isRetryable)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Received %d bytes of data\n", len(body))

	return nil
}

func (h *DefaultHandler) isRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp != nil && resp.StatusCode >= serverErrorThreshold {
		return true
	}
	return false
}

// retryableAPICall retries fn up to maxRetries times when isRetryable returns true
func retryableAPICall(fn func() (*http.Response, error), maxRetries int, isRetryable func(*http.Response, error) bool) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * defaultRetryDelay)
		}

		resp, err = fn()

		if !isRetryable(resp, err) {
			return resp, err
		}

		if resp != nil {
			resp.Body.Close()
		}

		if attempt == maxRetries {
			break
		}
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, err)
}
