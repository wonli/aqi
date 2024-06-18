package ws

type ApiData struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`

	HttpStatus int
}
