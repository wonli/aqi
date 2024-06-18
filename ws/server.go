package ws

import (
	"net/http"
	"sync"
)

type Server struct {
	engine http.Handler

	fn http.HandlerFunc

	port     string
	isDev    bool
	dataPath string
}

var (
	wss  *Server
	once sync.Once
)

func NewServer(engine http.Handler) *Server {
	once.Do(func() {
		InitManager()
		wss = &Server{
			engine: engine,
			port:   ":3322",
			fn:     HttpHandler,
		}
	})

	return wss
}

func (s *Server) Handler(fn http.HandlerFunc) {
	s.fn = fn
}

func (s *Server) SetPort(p string) {
	s.port = p
}

func (s *Server) SetDataPath(p string) {
	s.dataPath = p
}

func (s *Server) SetIsDev(dev bool) {
	s.isDev = dev
}

func (s *Server) Init() {

}

func (s *Server) Run() {
	err := http.ListenAndServe(s.port, s.engine)
	if err != nil {
		panic(err)
	}
}
