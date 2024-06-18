package apic

import "net/url"

type RequestData struct {
	Url        string     `json:"url"`
	HttpMethod HttpMethod `json:"httpMethod,omitempty"`
	ApiId      string     `json:"apiId"`
	Path       string     `json:"path,omitempty"`
	Query      url.Values `json:"query,omitempty"`
	Form       Params     `json:"form,omitempty"`
	WWWForm    Params     `json:"WWWForm,omitempty"`
	PostBody   Params     `json:"post_body,omitempty"`
	Header     Params     `json:"header,omitempty"`
	Debug      bool       `json:"debug"`
}

func (a *RequestData) InitFromApiClient(api Api) {
	if a.Url == "" {
		a.Url = api.Url()
	}

	if a.Path == "" {
		a.Path = api.Path()
	}

	if a.HttpMethod == "" {
		a.HttpMethod = api.HttpMethod()
	}

	if a.Query == nil {
		a.Query = api.Query()
	}

	if a.PostBody == nil {
		a.PostBody = api.PostBody()
	}

	if a.Header == nil {
		a.Header = api.Headers()
	}

	if a.Form == nil {
		a.Form = api.FormData()
	}

	if a.WWWForm == nil {
		a.WWWForm = api.WWWFormData()
	}

	a.Debug = api.Debug()
}

func (a *RequestData) MarshalToString() (string, error) {
	return marshal(a)
}
