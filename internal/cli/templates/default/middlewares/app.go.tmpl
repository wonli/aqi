package middlewares

import (
	"github.com/wonli/aqi/ws"
)

func App() ws.HandlerFunc {
	return func(a *ws.Context) {
		if a.Client.AppId == "" || a.Client.Platform == "" {
			appId := a.Client.HttpRequest.URL.Query().Get("appId")
			platform := a.Client.HttpRequest.URL.Query().Get("platform")

			//获取storeId
			if platform == "" {
				a.SendCode(10003, "不支持的平台")
				a.Abort()
				return
			}

			a.Client.AppId = appId
			a.Client.Platform = platform
		}

		// app登录
		if a.Client.AppId == "app" && a.Client.ClientId == "" {
			clientId := a.Client.HttpRequest.URL.Query().Get("clientId")
			if clientId == "" {
				a.SendCode(10013, "client不能为空")
				a.Abort()
				return
			}

			a.Client.ClientId = clientId
			err := a.Client.Hub.UserLogin(clientId, a.Client.AppId, a.Client)
			if err != nil {
				a.SendCode(10013, "登录失败")
				a.Abort()
				return
			}
		}
	}
}
