// Package router provides an implementation of di.Router.
package router

import (
	"net/http"
	"sync"
)

type verbMux map[string]http.Handler

func (m verbMux) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h := m[req.Method]
	if h == nil {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.ServeHTTP(rw, req)
}

// Mux implements di.Router on top of http.ServeMux.
type Mux struct {
	mu sync.RWMutex
	// the request chain is Mux -> http.ServeMux -> verbMux
	// patternMux handles pattern multiplexing and verbMux verbs
	patternMux *http.ServeMux
	byPattern  map[string]verbMux // keeps track of verbMux by pattern for registration
}

// New allocates and returns a new Mux.
func New() *Mux {
	return &Mux{patternMux: http.NewServeMux(), byPattern: make(map[string]verbMux)}
}

// Handle registers handler for request matching <verb, pattern>. Any existing
// handler for those arguments will get overwritten.
func (m *Mux) Handle(verb, pattern string, handler http.Handler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h := m.byPattern[pattern]
	if h == nil { // pattern not seen before
		h = make(map[string]http.Handler)
		m.patternMux.Handle(pattern, h) // register verbMux
	}
	h[verb] = handler
}

// HandleFunc registers handler for request matching <verb, pattern>.
func (m *Mux) HandleFunc(verb, pattern string, handler func(http.ResponseWriter, *http.Request)) {
	m.Handle(verb, pattern, http.HandlerFunc(handler))
}

// ServeHTTP dispatches the request to the handler whose verb equals the request
// Method and whose pattern most closely matches the request URL.
func (m *Mux) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.patternMux.ServeHTTP(rw, req)
}
