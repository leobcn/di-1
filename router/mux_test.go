package router_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kkrs/di/router"
)

func mustReqResponse(method, path string, body io.ReadCloser) (*http.Request, *httptest.ResponseRecorder) {
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		panic(err)
	}
	return req, httptest.NewRecorder()
}

func TestMux(t *testing.T) {
	router := router.New()
	get, ok := "GET", "/ok"
	router.HandleFunc(get, ok, func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	req, rw := mustReqResponse(get, ok, nil)
	router.ServeHTTP(rw, req)
	t.Logf("Scenario: Router routes request %s %s to handler successfully", get, ok)
	if rw.Code != http.StatusOK {
		t.Fatalf("Expected status %s, but got %s", http.StatusText(http.StatusOK), http.StatusText(rw.Code))
	}

	t.Log()
	t.Logf("Scenario: Router returns %s if request method is different from the one handler was registered with",
		http.StatusText(http.StatusMethodNotAllowed))

	notAllowed := "/methodnotallowed"
	router.HandleFunc(get, notAllowed, func(http.ResponseWriter, *http.Request) {
		// should not get here
	})

	req, rw = mustReqResponse("POST", notAllowed, nil)
	router.ServeHTTP(rw, req)
	if rw.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Expected status %s, but got %s", http.StatusText(http.StatusMethodNotAllowed), http.StatusText(rw.Code))
	}
}
