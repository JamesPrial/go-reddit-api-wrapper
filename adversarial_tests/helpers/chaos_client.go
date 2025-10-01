package helpers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// ChaosMode defines the type of chaos to inject
type ChaosMode int

const (
	// ChaosNone performs normal HTTP requests
	ChaosNone ChaosMode = iota

	// ChaosTimeout simulates request timeouts
	ChaosTimeout

	// ChaosConnectionReset simulates connection reset
	ChaosConnectionReset

	// ChaosPartialRead simulates partial response reads
	ChaosPartialRead

	// ChaosSlowResponse simulates very slow responses
	ChaosSlowResponse

	// ChaosRandomError simulates random errors
	ChaosRandomError

	// ChaosMalformedResponse simulates malformed HTTP responses
	ChaosMalformedResponse

	// ChaosEmptyBody simulates empty response bodies
	ChaosEmptyBody

	// ChaosOversizedBody simulates extremely large responses
	ChaosOversizedBody

	// ChaosInvalidJSON simulates invalid JSON in response
	ChaosInvalidJSON

	// ChaosDNSFailure simulates DNS resolution failures
	ChaosDNSFailure

	// ChaosIntermittent randomly applies different chaos modes
	ChaosIntermittent
)

// ChaosConfig configures the chaos client behavior
type ChaosConfig struct {
	// Mode determines which type of chaos to inject
	Mode ChaosMode

	// FailureRate determines probability of failure (0.0 to 1.0)
	// Only used for ChaosIntermittent mode
	FailureRate float64

	// Delay adds artificial delay to responses
	Delay time.Duration

	// PartialReadBytes specifies how many bytes to read before failing
	// Only used for ChaosPartialRead mode
	PartialReadBytes int

	// CustomResponse allows injecting custom HTTP responses
	CustomResponse *http.Response

	// CustomError allows injecting custom errors
	CustomError error
}

// ChaosClient wraps an http.Client and injects various failure modes
type ChaosClient struct {
	client      *http.Client
	config      *ChaosConfig
	requestNum  uint64
	rnd         *rand.Rand
}

// NewChaosClient creates a new chaos client
func NewChaosClient(config *ChaosConfig) *ChaosClient {
	if config == nil {
		config = &ChaosConfig{Mode: ChaosNone}
	}
	return &ChaosClient{
		client:     &http.Client{Timeout: 30 * time.Second},
		config:     config,
		requestNum: 0,
		rnd:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Do executes an HTTP request with chaos injection
func (c *ChaosClient) Do(req *http.Request) (*http.Response, error) {
	_ = atomic.AddUint64(&c.requestNum, 1)

	// Check if we should inject chaos
	mode := c.config.Mode
	if mode == ChaosIntermittent {
		if c.rnd.Float64() < c.config.FailureRate {
			// Select a random chaos mode
			modes := []ChaosMode{
				ChaosTimeout,
				ChaosConnectionReset,
				ChaosPartialRead,
				ChaosSlowResponse,
				ChaosRandomError,
				ChaosMalformedResponse,
				ChaosEmptyBody,
				ChaosInvalidJSON,
			}
			mode = modes[c.rnd.Intn(len(modes))]
		} else {
			mode = ChaosNone
		}
	}

	// Apply delay if configured
	if c.config.Delay > 0 {
		time.Sleep(c.config.Delay)
	}

	// Apply chaos based on mode
	switch mode {
	case ChaosNone:
		return c.client.Do(req)

	case ChaosTimeout:
		// Create a context that times out immediately
		ctx, cancel := context.WithTimeout(req.Context(), 1*time.Nanosecond)
		defer cancel()
		return c.client.Do(req.WithContext(ctx))

	case ChaosConnectionReset:
		return nil, errors.New("connection reset by peer")

	case ChaosDNSFailure:
		return nil, &DNSError{
			Err:    "no such host",
			Server: "8.8.8.8",
		}

	case ChaosRandomError:
		errorMessages := []string{
			"network unreachable",
			"connection refused",
			"broken pipe",
			"no route to host",
			"connection timed out",
			"TLS handshake timeout",
			"certificate verify failed",
		}
		return nil, errors.New(errorMessages[c.rnd.Intn(len(errorMessages))])

	case ChaosPartialRead:
		// Make the request normally
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}

		// Read only partial data
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Determine how much to read
		partialSize := c.config.PartialReadBytes
		if partialSize <= 0 || partialSize >= len(bodyBytes) {
			partialSize = len(bodyBytes) / 2
		}
		if partialSize > len(bodyBytes) {
			partialSize = len(bodyBytes)
		}

		// Create a reader that fails mid-read
		partialBody := &partialReadCloser{
			reader:     bytes.NewReader(bodyBytes[:partialSize]),
			failAfter:  partialSize,
			totalRead:  0,
		}

		resp.Body = partialBody
		return resp, nil

	case ChaosSlowResponse:
		// Add significant delay
		time.Sleep(5 * time.Second)
		return c.client.Do(req)

	case ChaosMalformedResponse:
		// Return a response with malformed structure
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          io.NopCloser(strings.NewReader("This is not valid HTTP response data\x00\x01\x02")),
			ContentLength: -1,
			Request:       req,
			Header:        make(http.Header),
		}, nil

	case ChaosEmptyBody:
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          io.NopCloser(bytes.NewReader([]byte{})),
			ContentLength: 0,
			Request:       req,
			Header:        make(http.Header),
		}, nil

	case ChaosOversizedBody:
		// Generate a 15MB response (over the 10MB limit)
		largeData := make([]byte, 15*1024*1024)
		for i := range largeData {
			largeData[i] = byte('A' + (i % 26))
		}
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          io.NopCloser(bytes.NewReader(largeData)),
			ContentLength: int64(len(largeData)),
			Request:       req,
			Header:        make(http.Header),
		}, nil

	case ChaosInvalidJSON:
		invalidJSON := `{"access_token": "valid_token", "expires_in": "not_a_number"}`
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    200,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Body:          io.NopCloser(strings.NewReader(invalidJSON)),
			ContentLength: int64(len(invalidJSON)),
			Request:       req,
			Header:        make(http.Header),
		}, nil

	default:
		if c.config.CustomResponse != nil {
			return c.config.CustomResponse, c.config.CustomError
		}
		return c.client.Do(req)
	}
}

// RoundTrip implements http.RoundTripper interface
func (c *ChaosClient) RoundTrip(req *http.Request) (*http.Response, error) {
	return c.Do(req)
}

// partialReadCloser is an io.ReadCloser that fails after reading a certain amount
type partialReadCloser struct {
	reader    io.Reader
	failAfter int
	totalRead int
}

func (p *partialReadCloser) Read(buf []byte) (int, error) {
	if p.totalRead >= p.failAfter {
		return 0, errors.New("connection reset during read")
	}

	n, err := p.reader.Read(buf)
	p.totalRead += n

	if p.totalRead >= p.failAfter {
		return n, errors.New("connection reset during read")
	}

	return n, err
}

func (p *partialReadCloser) Close() error {
	return nil
}

// DNSError simulates DNS lookup failures
type DNSError struct {
	Err    string
	Server string
}

func (e *DNSError) Error() string {
	return fmt.Sprintf("lookup failed: %s (server: %s)", e.Err, e.Server)
}

func (e *DNSError) Temporary() bool {
	return true
}

func (e *DNSError) Timeout() bool {
	return false
}

// MockResponseBuilder helps build custom mock responses
type MockResponseBuilder struct {
	status     int
	body       string
	headers    map[string]string
	delay      time.Duration
}

// NewMockResponseBuilder creates a new mock response builder
func NewMockResponseBuilder() *MockResponseBuilder {
	return &MockResponseBuilder{
		status:  200,
		headers: make(map[string]string),
	}
}

// WithStatus sets the HTTP status code
func (b *MockResponseBuilder) WithStatus(code int) *MockResponseBuilder {
	b.status = code
	return b
}

// WithBody sets the response body
func (b *MockResponseBuilder) WithBody(body string) *MockResponseBuilder {
	b.body = body
	return b
}

// WithHeader adds a header to the response
func (b *MockResponseBuilder) WithHeader(key, value string) *MockResponseBuilder {
	b.headers[key] = value
	return b
}

// WithDelay adds a delay before returning the response
func (b *MockResponseBuilder) WithDelay(delay time.Duration) *MockResponseBuilder {
	b.delay = delay
	return b
}

// Build creates the HTTP response
func (b *MockResponseBuilder) Build(req *http.Request) *http.Response {
	if b.delay > 0 {
		time.Sleep(b.delay)
	}

	header := make(http.Header)
	for k, v := range b.headers {
		header.Set(k, v)
	}

	return &http.Response{
		Status:        fmt.Sprintf("%d %s", b.status, http.StatusText(b.status)),
		StatusCode:    b.status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          io.NopCloser(strings.NewReader(b.body)),
		ContentLength: int64(len(b.body)),
		Request:       req,
		Header:        header,
	}
}
