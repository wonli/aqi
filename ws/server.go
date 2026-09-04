package ws

import (
	"net/http"
	"sync"

	"github.com/wonli/aqi/i18n"
)

type Server struct {
	engine http.Handler

	fn http.HandlerFunc

	port            string
	isDev           bool
	dataPath        string
	defaultLanguage string
	i18n             *i18n.Manager
}

var (
	wss  *Server
	once sync.Once
)

func NewServer(engine http.Handler) *Server {
	once.Do(func() {
		InitManager()
		wss = &Server{
			engine:          engine,
			fn:              HttpHandler,
			defaultLanguage: "zh",
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

func (s *Server) SetLanguage(language string) {
	if language != "" {
		s.defaultLanguage = language
	}
}

func (s *Server) DefaultLanguage() string {
	if s == nil || s.defaultLanguage == "" {
		return "zh"
	}
	return s.defaultLanguage
}

func (s *Server) Init() {
	s.i18n = i18n.New(s.dataPath, s.DefaultLanguage())
}

func (s *Server) translate(language, action string, code int, msg string) string {
	if s == nil || s.i18n == nil {
		return msg
	}
	return s.i18n.Translate(language, action, code, msg)
}

func (s *Server) Run() {
	err := http.ListenAndServe(s.port, s.engine)
	if err != nil {
		panic(err)
	}
}
