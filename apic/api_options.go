package apic

import "net/url"

// Options request options
type Options struct {
	Query    url.Values
	PostBody Params
	Headers  Params
	Setup    Params
}
