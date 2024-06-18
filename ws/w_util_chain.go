package ws

func New(action string) *Action {
	return &Action{
		Action: action,
	}
}

func (m *Action) WithCode(code int) *Action {
	m.Code = code
	return m
}

func (m *Action) WithData(data any) *Action {
	m.Data = data
	return m
}

func (m *Action) WithMsg(msg string) *Action {
	m.Msg = msg
	return m
}
