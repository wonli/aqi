package ws

import "github.com/wonli/aqi/utils/ip"

func (c *Client) GetIP() string {
	return ip.GetIPAddress(c.HttpRequest)
}
