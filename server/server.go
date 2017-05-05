package server

import (
	"net"
	"net/http"

	"github.com/gorilla/mux"
)

// Server provides a web interface for interacting with the application.
type Server struct {
	stopped  chan bool
	router   *mux.Router
	listener net.Listener
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
		srv    = http.Server{}
		router = mux.NewRouter()
		s      = &Server{
			stopped:  make(chan bool),
			router:   router,
			listener: l,
		}
	)
	srv.Handler = s
	router.PathPrefix("/static").Handler(http.FileServer(HTTP))
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
	s.router.ServeHTTP(w, r)
}

// Close shuts down the server.
func (s *Server) Close() {
	s.listener.Close()
	<-s.stopped
}
