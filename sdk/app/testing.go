package app

import (
	"encoding/json"
	"fmt"

	"github.com/wazeos/wazeos/sdk/driver"
)

// MockIOClient provides a mock implementation of IOClient for testing.
type MockIOClient struct {
	ctx *Context

	// Mock storage
	files       map[string][]byte             // In-memory file storage
	appResults  map[string]*Response          // Canned app call responses
	httpResults map[string]*HTTPResponse      // Canned HTTP responses
	messages    map[string][]*Message         // Message queue storage
	callLog     []MockCall                    // Log of all calls made
}

// MockCall represents a logged call for test assertions.
type MockCall struct {
	Type   string      // "ReadFile", "WriteFile", "CallApp", etc.
	Params interface{} // Parameters of the call
}

// NewMockIO creates a new mock I/O client for testing.
func NewMockIO() *MockIOClient {
	return &MockIOClient{
		files:       make(map[string][]byte),
		appResults:  make(map[string]*Response),
		httpResults: make(map[string]*HTTPResponse),
		messages:    make(map[string][]*Message),
		callLog:     []MockCall{},
	}
}

// Mock configuration methods

// MockFile sets the content for a file path.
func (m *MockIOClient) MockFile(path string, content []byte) {
	m.files[path] = content
}

// MockApp sets the response for an app call.
func (m *MockIOClient) MockApp(appName string, response *Response) {
	m.appResults[appName] = response
}

// MockHTTP sets the response for an HTTP URL.
func (m *MockIOClient) MockHTTP(url string, response *HTTPResponse) {
	m.httpResults[url] = response
}

// MockMessages sets the messages for a topic.
func (m *MockIOClient) MockMessages(topic string, messages []*Message) {
	m.messages[topic] = messages
}

// GetCallLog returns the log of all calls made.
func (m *MockIOClient) GetCallLog() []MockCall {
	return m.callLog
}

// MockIOClient implements IOClient interface

// ReadFile reads from mock file storage.
func (m *MockIOClient) ReadFile(path string) ([]byte, error) {
	m.callLog = append(m.callLog, MockCall{Type: "ReadFile", Params: path})

	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, NewError("NOT_FOUND", fmt.Sprintf("file not found: %s", path), 404)
}

// WriteFile writes to mock file storage.
func (m *MockIOClient) WriteFile(path string, data []byte) error {
	m.callLog = append(m.callLog, MockCall{Type: "WriteFile", Params: map[string]interface{}{
		"path": path,
		"size": len(data),
	}})

	m.files[path] = data
	return nil
}

// DeleteFile deletes from mock file storage.
func (m *MockIOClient) DeleteFile(path string) error {
	m.callLog = append(m.callLog, MockCall{Type: "DeleteFile", Params: path})

	if _, ok := m.files[path]; ok {
		delete(m.files, path)
		return nil
	}
	return NewError("NOT_FOUND", fmt.Sprintf("file not found: %s", path), 404)
}

// ListFiles lists files from mock storage.
func (m *MockIOClient) ListFiles(dir string) ([]string, error) {
	m.callLog = append(m.callLog, MockCall{Type: "ListFiles", Params: dir})

	var files []string
	for path := range m.files {
		files = append(files, path)
	}
	return files, nil
}

// Get makes a mock HTTP GET request.
func (m *MockIOClient) Get(url string) (*HTTPResponse, error) {
	return m.Request("GET", url, nil, nil)
}

// Post makes a mock HTTP POST request.
func (m *MockIOClient) Post(url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	return m.Request("POST", url, body, headers)
}

// Request makes a mock HTTP request.
func (m *MockIOClient) Request(method, url string, body []byte, headers map[string]string) (*HTTPResponse, error) {
	m.callLog = append(m.callLog, MockCall{Type: "HTTPRequest", Params: map[string]interface{}{
		"method": method,
		"url":    url,
	}})

	if resp, ok := m.httpResults[url]; ok {
		return resp, nil
	}

	// Default 404 response
	return &HTTPResponse{
		StatusCode: 404,
		Headers:    make(map[string]string),
		Body:       []byte("Not Found"),
	}, nil
}

// CallApp makes a mock app call.
func (m *MockIOClient) CallApp(appName string, args ...string) (*Response, error) {
	return m.CallAppWithInput(appName, nil, args...)
}

// CallAppWithInput makes a mock app call with input.
func (m *MockIOClient) CallAppWithInput(appName string, input []byte, args ...string) (*Response, error) {
	m.callLog = append(m.callLog, MockCall{Type: "CallApp", Params: map[string]interface{}{
		"appName": appName,
		"args":    args,
	}})

	if resp, ok := m.appResults[appName]; ok {
		return resp, nil
	}

	return nil, NewError("NOT_FOUND", fmt.Sprintf("app not found: %s", appName), 404)
}

// Publish publishes a mock message.
func (m *MockIOClient) Publish(topic string, message []byte) error {
	return m.PublishWithKey(topic, "", message)
}

// PublishWithKey publishes a mock message with a key.
func (m *MockIOClient) PublishWithKey(topic, key string, message []byte) error {
	m.callLog = append(m.callLog, MockCall{Type: "Publish", Params: map[string]interface{}{
		"topic": topic,
		"key":   key,
	}})

	msg := &Message{
		ID:      fmt.Sprintf("msg-%d", len(m.messages[topic])),
		Topic:   topic,
		Key:     key,
		Headers: make(map[string]string),
		Body:    message,
	}

	m.messages[topic] = append(m.messages[topic], msg)
	return nil
}

// Consume consumes mock messages.
func (m *MockIOClient) Consume(topic string, opts *ConsumeOptions) ([]*Message, error) {
	m.callLog = append(m.callLog, MockCall{Type: "Consume", Params: map[string]interface{}{
		"topic": topic,
		"opts":  opts,
	}})

	if messages, ok := m.messages[topic]; ok {
		maxCount := len(messages)
		if opts != nil && opts.MaxCount > 0 && opts.MaxCount < maxCount {
			maxCount = opts.MaxCount
		}
		return messages[:maxCount], nil
	}

	return []*Message{}, nil
}

// Call makes a generic mock resource call.
func (m *MockIOClient) Call(uri, method string, body []byte, headers map[string]string) (*driver.ResourceResult, error) {
	m.callLog = append(m.callLog, MockCall{Type: "Call", Params: map[string]interface{}{
		"uri":    uri,
		"method": method,
	}})

	return &driver.ResourceResult{
		StatusCode: 200,
		Headers:    make(map[string]string),
		Body:       []byte("{}"),
	}, nil
}

// TestContext creates a context with mock I/O for testing.
func TestContext() *Context {
	mockIO := NewMockIO()

	ctx := &Context{
		RequestID:   "test-request-1",
		TraceID:     "test-trace-1",
		Principal:   "user:test",
		Permissions: driver.NewPermissionContext(nil),
		Metadata:    make(map[string]string),
	}

	ctx.io = mockIO
	ctx.Log = &Logger{ctx: ctx}

	// Store reference for mock access
	mockIO.ctx = ctx

	return ctx
}

// TestContextWithPermissions creates a test context with specific permissions.
func TestContextWithPermissions(entries []driver.PermissionEntry) *Context {
	ctx := TestContext()
	ctx.Permissions = driver.NewPermissionContext(entries)
	return ctx
}

// Example helper for creating permission entries in tests

// AllowFile creates a permission entry for file access.
func AllowFile(pattern string, mode string) driver.PermissionEntry {
	var access driver.AccessBits
	for _, char := range mode {
		switch char {
		case 'r':
			access |= driver.AccessRead
		case 'w':
			access |= driver.AccessWrite
		case 'x':
			access |= driver.AccessExecute
		}
	}

	return driver.PermissionEntry{
		URIPattern: fmt.Sprintf("file://%s", pattern),
		Access:     access,
	}
}

// AllowHTTP creates a permission entry for HTTP access.
func AllowHTTP(pattern string) driver.PermissionEntry {
	return driver.PermissionEntry{
		URIPattern: pattern,
		Access:     driver.AccessRead,
	}
}

// AllowApp creates a permission entry for app calls.
func AllowApp(pattern string) driver.PermissionEntry {
	return driver.PermissionEntry{
		URIPattern: fmt.Sprintf("fn://%s", pattern),
		Access:     driver.AccessExecute,
	}
}

// GetMockIO returns the mock I/O client from a test context.
func GetMockIO(ctx *Context) *MockIOClient {
	mock, _ := ctx.io.(*MockIOClient)
	return mock
}

// AssertCallCount checks that a specific call type was made N times.
func AssertCallCount(mock *MockIOClient, callType string, expected int) error {
	count := 0
	for _, call := range mock.callLog {
		if call.Type == callType {
			count++
		}
	}

	if count != expected {
		return fmt.Errorf("expected %d calls of type %s, got %d", expected, callType, count)
	}
	return nil
}

// MarshalJSON helper for test data.
func MustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal JSON: %v", err))
	}
	return data
}
