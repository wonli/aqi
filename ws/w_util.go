package ws

func Msg(action, msg string) []byte {
	res := &Action{
		Action: action,
		Msg:    msg,
	}

	return res.json()
}

func Code(action string, code int, msg string) []byte {
	res := &Action{
		Action: action,
		Code:   code,
		Msg:    msg,
	}

	return res.json()
}

func Data(action string, data any) []byte {
	res := &Action{
		Action: action,
		Data:   data,
	}

	return res.json()
}
