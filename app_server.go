package aqi

import (
	"net/http"
	"os"

	"github.com/fatih/color"

	"github.com/wonli/aqi/ws"
)

func (a *AppConfig) WithHttpServer(svr http.Handler) {
	a.HttpServer = svr
}

func (a *AppConfig) Start() {
	if a.HttpServer == nil {
		color.Red("HttpServer not config")
		os.Exit(0)
	}

	if a.HttpServer != nil {
		server := ws.NewServer(a.HttpServer)
		server.SetDataPath(a.DataPath)
		server.SetIsDev(a.devMode)
		server.Init()
	}

	err := http.ListenAndServe(":"+a.ServerPort, a.HttpServer)
	if err != nil {
		color.Red("Listener error: %s", err.Error())
		os.Exit(0)
	}
}
