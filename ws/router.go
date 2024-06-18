package ws

import (
	"strings"
)

type HandlerFunc func(a *Context)
type HandlersChain []HandlerFunc

type IRouter interface {
	Use(middleware ...HandlerFunc) IRouter
	Group(name string) IRouter
	Add(name string, fn ...HandlerFunc)
}

type Routers struct {
	manager        *ActionManager
	handlerMembers HandlersChain
	groups         []string
}

func NewRouter() Routers {
	return Routers{
		manager: InitManager(),
	}
}

func (r Routers) Add(name string, fn ...HandlerFunc) {
	if r.groups != nil {
		name = strings.Join(r.groups, ".") + "." + name
	}

	has := r.manager.Has(name)
	if has {
		panic("Duplicate route: " + name)
	}

	chains := make(HandlersChain, len(r.handlerMembers), len(r.handlerMembers)+len(fn))
	copy(chains, r.handlerMembers)

	r.manager.Add(name, append(chains, fn...))
}

func (r Routers) Use(middleware ...HandlerFunc) IRouter {
	r.handlerMembers = append(r.handlerMembers, middleware...)
	return r
}

func (r Routers) Group(name string) IRouter {
	r.groups = append(r.groups, name)
	return r
}
