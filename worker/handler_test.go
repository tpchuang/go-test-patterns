package worker

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	expectedRetryError = "failed after 4 attempts"
)

type MockRoundTripper struct {
	Response  *http.Response
	Err       error
	CallCount int
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.CallCount++
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Response, nil
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type ErrorReader struct{}

func (e *ErrorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("read error")
}

func (e *ErrorReader) Close() error {
	return nil
}

func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

var _ = Describe("Handler", func() {
	var (
		handler       *DefaultHandler
		mockTransport *MockRoundTripper
		testURL       string
	)

	BeforeEach(func() {
		mockTransport = &MockRoundTripper{}
		handler = &DefaultHandler{
			httpClient: &http.Client{
				Transport: mockTransport,
			},
		}
		testURL = "https://example.com/api/test"
	})

	Describe("getData", func() {
		Context("when HTTP request is successful", func() {
			It("should return nil error for 200 OK response", func() {
				mockTransport.Response = createMockResponse(200, `{"status":"success"}`)

				err := handler.getData(testURL)

				Expect(err).To(BeNil())
				Expect(mockTransport.CallCount).To(Equal(1))
			})
		})

		Context("when HTTP request returns non-200 status", func() {
			It("should return error for 404 Not Found", func() {
				mockTransport.Response = createMockResponse(404, `{"error":"not found"}`)

				err := handler.getData(testURL)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected status code: 404"))
				Expect(mockTransport.CallCount).To(Equal(1))
			})

			It("should return error for 400 Bad Request", func() {
				mockTransport.Response = createMockResponse(400, `{"error":"bad request"}`)

				err := handler.getData(testURL)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected status code: 400"))
				Expect(mockTransport.CallCount).To(Equal(1))
			})
		})

		Context("when HTTP request fails with network error", func() {
			It("should retry 3 times and return error", func() {
				mockTransport.Err = fmt.Errorf("network error")

				err := handler.getData(testURL)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedRetryError))
				Expect(mockTransport.CallCount).To(Equal(4))
			})
		})

		Context("when HTTP request returns 5xx server errors", func() {
			It("should retry 3 times for 500 Internal Server Error", func() {
				mockTransport.Response = createMockResponse(500, `{"error":"server error"}`)

				err := handler.getData(testURL)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedRetryError))
				Expect(mockTransport.CallCount).To(Equal(4))
			})

			It("should retry 3 times for 503 Service Unavailable", func() {
				mockTransport.Response = createMockResponse(503, `{"error":"service unavailable"}`)

				err := handler.getData(testURL)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedRetryError))
				Expect(mockTransport.CallCount).To(Equal(4))
			})
		})

		Context("when HTTP request succeeds after retries", func() {
			It("should succeed on second attempt after initial 500 error", func() {
				callCount := 0
				mockTransport = &MockRoundTripper{}

				handler.httpClient.Transport = RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					callCount++
					if callCount == 1 {
						return createMockResponse(500, `{"error":"server error"}`), nil
					}
					return createMockResponse(200, `{"status":"success"}`), nil
				})

				err := handler.getData(testURL)

				Expect(err).To(BeNil())
				Expect(callCount).To(Equal(2))
			})
		})

		Context("when reading response body fails", func() {
			It("should return error when io.ReadAll fails", func() {
				mockTransport.Response = &http.Response{
					StatusCode: 200,
					Body:       &ErrorReader{},
					Header:     make(http.Header),
				}

				err := handler.getData(testURL)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to read response body"))
				Expect(mockTransport.CallCount).To(Equal(1))
			})
		})
	})

	Describe("Handle", func() {
		It("should call getData with the provided URL", func() {
			mockTransport.Response = createMockResponse(200, `{"status":"success"}`)

			err := handler.Handle(testURL)

			Expect(err).To(BeNil())
			Expect(mockTransport.CallCount).To(Equal(1))
		})

		It("should return error when getData fails", func() {
			mockTransport.Err = fmt.Errorf("connection refused")

			err := handler.Handle(testURL)

			Expect(err).To(HaveOccurred())
		})
	})
})
