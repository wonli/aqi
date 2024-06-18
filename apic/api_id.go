package apic

type ApiId struct {
	Name     string
	Client   Api
	Request  *RequestData
	Response *ResponseData
}

// Named registers as a named interface.
func (a *ApiId) Named() *ApiId {
	Apis.Named(a)
	return a
}
