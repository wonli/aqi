package ws

type Page struct {
	Current  int   `json:"current"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type Pagination struct {
	*Page `json:"page"`

	Rows   any `json:"rows,omitempty"`
	ExData H   `json:"exData,omitempty"`

	Limit  int `json:"-"` //PageSize alias
	Offset int `json:"-"` //避免每次计算offset
}

func InitPagination(p *Page, maxSize int) *Pagination {
	if p.Current <= 1 {
		p.Current = 1
	}

	if p.PageSize <= 0 {
		p.PageSize = 10
	}

	if maxSize > 0 && p.PageSize > maxSize {
		p.PageSize = maxSize
	}

	return &Pagination{
		Page:   p,
		Offset: (p.Current - 1) * p.PageSize,
		Limit:  p.PageSize,
	}
}

func (p *Pagination) AddExData(key string, val any) {
	if p.ExData == nil {
		p.ExData = H{}
	}

	p.ExData[key] = val
}
