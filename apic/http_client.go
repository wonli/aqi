package apic

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/guonaihong/gout"
	"github.com/guonaihong/gout/dataflow"
	"github.com/tidwall/gjson"
)

var Apis *ApiClients
var once sync.Once

// ApiClients api clients list
type ApiClients struct {
	ctx   context.Context
	proxy string
	named map[string]*ApiId
}

func Init() *ApiClients {
	once.Do(func() {
		Apis = &ApiClients{
			ctx:   context.Background(),
			named: map[string]*ApiId{},
		}
	})

	return Apis
}

func (a *ApiClients) Named(api *ApiId) {
	_, ok := a.named[api.Name]
	if ok {
		panic("ApiId registered multiple times")
	}

	a.named[api.Name] = api
}

func (a *ApiClients) WithContext(ctx context.Context) *ApiClients {
	a.ctx = ctx
	return a
}

func (a *ApiClients) WithProxy(proxy string) *ApiClients {
	a.proxy = proxy
	return a
}

func (a *ApiClients) Call(id *ApiId, op *Options) error {
	_, err := a.getApiData(id, op)
	if err != nil {
		return err
	}

	return nil
}

func (a *ApiClients) CallApi(id *ApiId, op *Options) (*ResponseData, error) {
	return a.getApiData(id, op)
}

func (a *ApiClients) CallNamed(name string, op *Options) (*ResponseData, error) {
	id, ok := a.named[name]
	if !ok {
		return nil, fmt.Errorf("named api not registered")
	}

	return a.getApiData(id, op)
}

func (a *ApiClients) CallGJson(id *ApiId, op *Options) (*gjson.Result, error) {
	apiData, err := a.getApiData(id, op)
	if err != nil {
		return nil, err
	}

	g := gjson.ParseBytes(apiData.Data)
	return &g, nil
}

func (a *ApiClients) CallBindJson(id *ApiId, resp any, op *Options) error {
	_, err := a.getApiData(id, op)
	if err != nil {
		return err
	}

	return id.Response.BindJson(resp)
}

func (a *ApiClients) CallFunc(id *ApiId, op *Options, callback func(a *Api, data []byte) error) error {
	apiData, err := a.getApiData(id, op)
	if err != nil {
		return err
	}

	return callback(&id.Client, apiData.Data)
}

func (a *ApiClients) getApiData(id *ApiId, op *Options) (*ResponseData, error) {
	api := id.Client
	if id.Request == nil {
		id.Request = &RequestData{}
	}

	if op == nil {
		op = &Options{}
	}

	id.Request.ApiId = id.Name
	id.Request.InitFromApiClient(id.Client)

	//setup
	api, err := api.Setup(id.Client, op)
	if err != nil {
		return nil, err
	}

	err = api.UseContext(a.ctx)
	if err != nil {
		return nil, err
	}

	// Merge query parameters
	if op.Query != nil {
		if id.Request.Query == nil {
			id.Request.Query = op.Query
		} else {
			for key, valData := range op.Query {
				if len(valData) == 1 {
					id.Request.Query.Add(key, valData[0])
				} else {
					for _, val := range valData {
						id.Request.Query.Add(key, val)
					}
				}
			}
		}
	}

	//set postBody
	if op.PostBody != nil {
		if id.Request.PostBody == nil {
			id.Request.PostBody = op.PostBody
		} else {
			for key, val := range op.PostBody {
				id.Request.PostBody[key] = val
			}
		}
	}

	//set header
	if op.Headers != nil {
		if id.Request.Header == nil {
			id.Request.Header = op.Headers
		} else {
			for key, val := range op.Headers {
				id.Request.Header[key] = val
			}
		}
	}

	err = api.OnRequest()
	if err != nil {
		return nil, err
	}

	var apiAddress = id.Request.Url + id.Request.Path
	var client *dataflow.DataFlow
	switch id.Request.HttpMethod {
	case POST:
		client = gout.POST(apiAddress)
	case DELETE:
		client = gout.DELETE(apiAddress)
	case HEAD:
		client = gout.HEAD(apiAddress)
	case OPTIONS:
		client = gout.OPTIONS(apiAddress)
	case PATCH:
		client = gout.OPTIONS(apiAddress)
	default:
		client = gout.GET(apiAddress)
	}

	if a.proxy != "" {
		client.SetProxy(a.proxy)
	}

	client.Debug(id.Request.Debug)
	if id.Request.Form != nil {
		client.SetForm(id.Request.Form)
	} else if id.Request.WWWForm != nil {
		client.SetWWWForm(id.Request.WWWForm)
	} else if id.Request.PostBody != nil {
		client.SetJSON(id.Request.PostBody)
	}

	if id.Request.Query != nil {
		client.SetQuery(id.Request.Query)
	}

	if id.Request.Header != nil {
		client.SetHeader(id.Request.Header)
	}

	id.Response = &ResponseData{}
	err = client.Code(&id.Response.HttpStatus).
		BindHeader(&id.Response.Header).BindBody(&id.Response.Data).Do()
	if err != nil {
		return nil, err
	}

	if id.Response.HttpStatus != http.StatusOK {
		err = api.OnHttpStatusError(id.Response.HttpStatus, id.Response.Data)
		if err != nil {
			return id.Response, err
		}
	}

	responseData, err := api.OnResponse(id.Response.Data)
	if err != nil {
		return nil, err
	}

	return responseData, nil
}
