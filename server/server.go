package server

import (
	"net"
	"net/http"
)

// Server provides a web interface for interacting with the application.
type Server struct {
	stopped  chan bool
	mux      *http.ServeMux
	l        net.Listener
	username string
	password string
}

// New creates a new HTTP server.
func New(addr, username, password string) (*Server, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	var (
		mux = http.NewServeMux()
		srv = http.Server{}
		s   = &Server{
			stopped: make(chan bool),
			mux:     mux,
			l:       l,
		}
	)
	srv.Handler = s
	go func() {
		defer close(s.stopped)
		srv.Serve(l)
	}()
	return s, nil
}

// ServeHTTP ensures that HTTP basic auth credentials match the requires ones
// and then routes the request to the muxer.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(s.username) != 0 && len(s.password) != 0 {
		username, password, ok := r.BasicAuth()
		if !ok || username != s.username || password != s.password {
			w.Header().Set("WWW-Authenticate", "Basic realm=cloudanchor")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

// Close shuts down the server.
func (s *Server) Close() {
	s.l.Close()
	<-s.stopped
}
