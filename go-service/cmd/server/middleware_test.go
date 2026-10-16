package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireInternalSecret(t *testing.T) {
	secret := "test-secret-12345"
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	handler := withMiddleware(requireInternalSecret(secret, dummyHandler))

	tests := []struct {
		name           string
		method         string
		path           string
		headers        map[string]string
		expectedStatus int
	}{
		{
			name:           "health check without secret allowed",
			method:         "GET",
			path:           "/health",
			headers:        nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "options preflight without secret allowed",
			method:         "OPTIONS",
			path:           "/api/dm/overview",
			headers:        nil,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "api endpoint without secret returns 401",
			method:         "GET",
			path:           "/api/dm/overview",
			headers:        nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "api endpoint with invalid secret returns 401",
			method: "GET",
			path:   "/api/dm/overview",
			headers: map[string]string{
				"X-Internal-Secret": "wrong-secret",
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "api endpoint with valid X-Internal-Secret header allowed",
			method: "GET",
			path:   "/api/dm/overview",
			headers: map[string]string{
				"X-Internal-Secret": secret,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "api endpoint with valid Bearer Authorization header allowed",
			method: "GET",
			path:   "/api/dm/overview",
			headers: map[string]string{
				"Authorization": "Bearer " + secret,
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Fatalf("expected status %d, got %d. Body: %s", tc.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

