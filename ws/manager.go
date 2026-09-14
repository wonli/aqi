package ws

import "sync"

type route struct {
	handlers HandlersChain
	coder    Coder
}

type ActionManager struct {
	routeMap map[string]*route
	coder    Coder
}

var msy sync.Once
var manager *ActionManager

func InitManager() *ActionManager {
	msy.Do(func() {
		manager = &ActionManager{
			routeMap: map[string]*route{},
		}

		//处理websocket
		go NewHubc().Run()
	})

	return manager
}

func (m *ActionManager) Add(name string, handlers HandlersChain) {
	m.add(name, handlers, nil)
}

func (m *ActionManager) add(name string, handlers HandlersChain, coder Coder) {
	m.routeMap[name] = &route{
		handlers: handlers,
		coder:    coder,
	}
}

func (m *ActionManager) Has(name string) bool {
	_, ok := m.routeMap[name]
	return ok
}

func (m *ActionManager) Handlers(name string) HandlersChain {
	r := m.route(name)
	if r == nil {
		return nil
	}
	return r.handlers
}

func (m *ActionManager) route(name string) *route {
	return m.routeMap[name]
}

func (m *ActionManager) routeCoder(name string) Coder {
	r := m.route(name)
	if r == nil {
		return nil
	}
	return r.coder
}

func (m *ActionManager) registerCoder(coder Coder) {
	if coder == nil {
		panic("websocket coder cannot be nil")
	}
	if m.coder != nil {
		panic("websocket coder already registered")
	}

	m.coder = coder
}

func (m *ActionManager) Coder() Coder {
	return m.coder
}
