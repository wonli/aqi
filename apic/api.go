package apic

import (
	"context"
	"net/url"
)

// Api api client interface.
type Api interface {
	Url() string                                   // Request API full URL.
	Path() string                                  // Request path.
	Query() url.Values                             // URL query parameters.
	Headers() Params                               // Headers required for the request.
	PostBody() Params                              // Request parameters.
	FormData() Params                              // Form data as map[string]string.
	WWWFormData() Params                           // Form data as map[string]string.
	Setup(api Api, op *Options) (Api, error)       // Setup for the API.
	HttpMethod() HttpMethod                        // HTTP method of the request.
	Debug() bool                                   // Whether to run in debug mode.
	UseContext(ctx context.Context) error          // Use context.
	OnRequest() error                              // Handle request data.
	OnHttpStatusError(code int, resp []byte) error // Handle HTTP status errors.
	OnResponse(resp []byte) (*ResponseData, error) // Process response data.
}
