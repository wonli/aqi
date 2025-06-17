package ws

import (
	"encoding/json"

	"github.com/tidwall/gjson"
)

func (c *Context) Get(key string) string {
	return gjson.Get(c.Params, key).String()
}

func (c *Context) GetInt(key string) int {
	v := gjson.Get(c.Params, key).Int()
	return int(v)
}

func (c *Context) GetJson(s any) error {
	return json.Unmarshal([]byte(c.Params), s)
}

func (c *Context) GetSliceVal(key string, options ...string) string {
	find := false
	v := gjson.Get(c.Params, key).String()
	for _, val := range options {
		if v == val {
			find = true
			break
		}
	}

	if find {
		return v
	}

	return ""
}

func (c *Context) GetPagination() *Pagination {
	p := &Page{}
	page := gjson.Get(c.Params, "page").String()
	if page != "" {
		_ = json.Unmarshal([]byte(page), &p)
	}

	return InitPagination(p, 100)
}

func (c *Context) GetMaxPagination(max int) *Pagination {
	p := &Page{}
	page := gjson.Get(c.Params, "page").String()
	if page != "" {
		_ = json.Unmarshal([]byte(page), &p)
	}

	return InitPagination(p, max)
}

func (c *Context) GetMinInt(key string, min int) int {
	d := c.GetInt(key)
	if d < min {
		return min
	}

	return d
}

func (c *Context) GetRangeInt(key string, min, max int) int {
	d := c.GetInt(key)
	if d < min {
		return min
	}

	if d > max {
		return max
	}

	return d
}

func (c *Context) GetBool(key string) bool {
	return gjson.Get(c.Params, key).Bool()
}

func (c *Context) GetId(key string) uint {
	v := gjson.Get(c.Params, key).Int()
	if v > 0 {
		return uint(v)
	}

	return 0
}
