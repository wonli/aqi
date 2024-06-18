package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wonli/aqi"
	"github.com/wonli/aqi/ws"
)

func main() {
	app := aqi.Init(
		aqi.ConfigFile("config.yaml"),
		aqi.HttpServer("Aqi", "port"),
	)

	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hi aqi!")
	})

	// Websocket
	engine.GET("/ws", func(c *gin.Context) {
		ws.HttpHandler(c.Writer, c.Request)
	})

	// Router
	wsr := ws.NewRouter()
	wsr.Add("hi", func(a *ws.Context) {
		a.Send(ws.H{
			"hi": time.Now(),
		})
	})

	app.WithHttpServer(engine)
	app.Start()
}
