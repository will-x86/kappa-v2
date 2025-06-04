package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponse(t *testing.T) {
	resp := NewResponse(http.StatusOK, map[string]string{"message": "hello"}, "req-123")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "req-123", resp.RequestID)
	assert.Equal(t, map[string]string{"Content-Type": "application/json"}, resp.Headers)
	assert.Equal(t, map[string]string{"message": "hello"}, resp.Body)
}

func TestResponse_WithHeader(t *testing.T) {
	resp := NewResponse(http.StatusOK, nil, "")
	resp = resp.WithHeader("X-Custom-Header", "value")

	assert.Equal(t, "value", resp.Headers["X-Custom-Header"])
	assert.Equal(t, "application/json", resp.Headers["Content-Type"]) // Ensure default is still there
}

func TestResponse_WithStatusCode(t *testing.T) {
	resp := NewResponse(http.StatusOK, nil, "")
	resp = resp.WithStatusCode(http.StatusAccepted)

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}
func TestCreateInvocationHandler2(t *testing.T){

	baseMockHandler := func(e Event) Response {
		// Base assertions for event fields populated by createInvocationHandler
		assert.NotEmpty(t, e.RequestID, "RequestID should be populated")
		assert.Equal(t, "POST", e.HTTPMethod, "HTTPMethod in event should be POST (from parsing logic)")

		if name, ok := e.Body["name"].(string); ok && name == "test" {
			return NewResponse(http.StatusOK, map[string]string{"reply": "hello " + name}, e.RequestID)
		}
		if _, ok := e.Body["needs_specific_reply"]; ok {
			return NewResponse(http.StatusCreated, map[string]string{"special_reply": "handled"}, e.RequestID)
		}
		return NewResponse(http.StatusBadRequest, map[string]string{"error": "bad input"}, e.RequestID)
	}

	invocationHandler := createInvocationHandler(baseMockHandler)

	tests := []struct {
		name               string
		method             string
		path               string
		body               any
		headers            map[string]string
		expectedStatusCode int
		expectedBodyPart   map[string]any // Using map[string]any for flexibility with JSON numbers
		checkResponse      func(t *testing.T, rr *httptest.ResponseRecorder, respBody Response)
	}{
		{
			name:   "Successful invocation with Kappa-Runtime-Aws-Request-Id header",
			method: http.MethodPost,
			path:   "/2015-03-31/functions/function/invocations",
			body:   Event{Body: map[string]any{"name": "test"}, HTTPMethod: "POST"}, // HTTPMethod in body is for user handler
			headers: map[string]string{
				"Kappa-Runtime-Aws-Request-Id": "kappa-aws-id",
				"Content-Type":                 "application/json",
			},
			expectedStatusCode: http.StatusOK,
			expectedBodyPart:   map[string]any{"reply": "hello test"},
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, respBody Response) {
				assert.Equal(t, "kappa-aws-id", respBody.RequestID)
			},
		},
		{
			name:   "Successful invocation with X-Request-Id header",
			method: http.MethodPost,
			path:   "/2015-03-31/functions/function/invocations",
			body:   Event{Body: map[string]any{"name": "test"}, HTTPMethod: "POST"},
			headers: map[string]string{
				"X-Request-Id": "x-req-id",
				"Content-Type": "application/json",
			},
			expectedStatusCode: http.StatusOK,
			expectedBodyPart:   map[string]any{"reply": "hello test"},
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, respBody Response) {
				assert.Equal(t, "req-x-req-id", respBody.RequestID) // "req-" prefix is added
			},
		},
		{
			name:   "Successful invocation with request ID in event body",
			method: http.MethodPost,
			path:   "/2015-03-31/functions/function/invocations",
			// RequestID in event body is prioritized if createInvocationHandler populates event.RequestID before handler call
			// Current logic: header -> event.RequestID (if empty) -> handler
			body: Event{Body: map[string]any{"name": "test"}, RequestID: "event-body-id", HTTPMethod: "POST"},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatusCode: http.StatusOK,
			expectedBodyPart:   map[string]any{"reply": "hello test"},
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, respBody Response) {
				// If no header ID, it might generate one, or use the one from event body
				// Current logic: uses event-body-id if it's passed in the event JSON.
				assert.Equal(t, "event-body-id", respBody.RequestID)
			},
		},
		{
			name:   "Handler returns different status code",
			method: http.MethodPost,
			path:   "/2015-03-31/functions/function/invocations",
			body:   Event{Body: map[string]any{"needs_specific_reply": true}, HTTPMethod: "POST"},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			expectedStatusCode: http.StatusOK, // The HTTP wrapper always returns 200 OK on successful handler execution
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, respBody Response) {
				assert.Equal(t, http.StatusCreated, respBody.StatusCode) // This is the handler's intended status
				assert.Equal(t, map[string]any{"special_reply": "handled"}, respBody.Body)
			},
		},
		{
			name:               "Method not allowed",
			method:             http.MethodGet,
			path:               "/2015-03-31/functions/function/invocations",
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:               "Bad request body (not JSON)",
			method:             http.MethodPost,
			path:               "/2015-03-31/functions/function/invocations",
			body:               "this is not json",
			headers:            map[string]string{"Content-Type": "application/json"},
			expectedStatusCode: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rr *httptest.ResponseRecorder, _ Response) {
				var errResp map[string]string
				err := json.Unmarshal(rr.Body.Bytes(), &errResp)
				require.NoError(t, err)
				assert.Equal(t, "Invalid request body", errResp["error"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody []byte
			var err error

			if strBody, ok := tt.body.(string); ok {
				reqBody = []byte(strBody)
			} else if tt.body != nil {
				reqBody, err = json.Marshal(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBuffer(reqBody))
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rr := httptest.NewRecorder()

			invocationHandler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatusCode, rr.Code)

			if rr.Code == http.StatusOK && tt.expectedBodyPart != nil { // For successful handler calls that return a body
				var respBody Response
				err := json.Unmarshal(rr.Body.Bytes(), &respBody)
				require.NoError(t, err, "Failed to unmarshal response body: %s", rr.Body.String())
				assert.Equal(t, tt.expectedBodyPart, respBody.Body)
			}

			if tt.checkResponse != nil {
				var respStruct Response // May be empty if not StatusOK from overall handler perspective
				if rr.Body.Len() > 0 && rr.Header().Get("Content-Type") == "application/json" {
					// Try to unmarshal only if it seems like a JSON response from our handler
					if err := json.Unmarshal(rr.Body.Bytes(), &respStruct); err != nil {
						if tt.expectedStatusCode == http.StatusOK { // Only require no error if we expected a full response struct
							require.NoError(t, err, "Failed to unmarshal response for checkResponse")
						}
					}
				}
				tt.checkResponse(t, rr, respStruct)
			}
		})
	}
}
func TestCreateInvocationHandler(t *testing.T) {
	mockHandler := func(e Event) Response {
		require.Equal(t, "test-id", e.RequestID)
		require.Equal(t, "POST", e.HTTPMethod, e.HTTPMethod) // This comes from the Event struct itself
		if name, ok := e.Body["name"].(string); ok && name == "test" {
			return NewResponse(http.StatusOK, map[string]string{"reply": "hello " + name}, e.RequestID)
		}
		return NewResponse(http.StatusBadRequest, map[string]string{"error": "bad input"}, e.RequestID)
	}

	invocationHandler := createInvocationHandler(mockHandler)

	t.Run("Successful invocation", func(t *testing.T) {
		eventPayload := Event{
			Body:      map[string]any{"name": "test"},
			RequestID: "test-id", // Set here for clarity, though handler logic also extracts from header
			HTTPMethod: "POST",
		}
		bodyBytes, _ := json.Marshal(eventPayload)

		req := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", bytes.NewBuffer(bodyBytes))
		req.Header.Set("Kappa-Runtime-Aws-Request-Id", "test-id")
		rr := httptest.NewRecorder()

		invocationHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		var respBody Response
		err := json.Unmarshal(rr.Body.Bytes(), &respBody)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, respBody.StatusCode)
		assert.Equal(t, "test-id", respBody.RequestID)
		expectedReply := map[string]any{"reply": "hello test"} // JSON unmarshals numbers to float64
		assert.Equal(t, expectedReply, respBody.Body)
	})

	t.Run("Method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/2015-03-31/functions/function/invocations", nil)
		rr := httptest.NewRecorder()
		invocationHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})

	t.Run("Bad request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/2015-03-31/functions/function/invocations", bytes.NewBufferString("not json"))
		rr := httptest.NewRecorder()
		invocationHandler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
		// You could also check the response body for the error message
	})
}

func TestHandleHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handleHealth(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

