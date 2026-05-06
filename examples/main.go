package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wonli/aqi"
	"github.com/wonli/aqi/mcp"
	"github.com/wonli/aqi/middlewares"
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
	wsr := ws.NewRouter().Use(middlewares.Telemetry(), middlewares.Recovery())
	wsr.Add("hi", func(a *ws.Context) {
		a.Observe(map[string]any{
			"demo": true,
		})
		a.Send(ws.H{
			"hi": time.Now(),
		})
	})

	mcpServer := mcp.NewServer(app,
		mcp.WithName("aqi-example"),
		mcp.WithVersion("0.1.0"),
	)

	mcpServer.Tool("time.now", mcp.Tool{
		Description: "Get current server time.",
		InputSchema: mcp.EmptyObjectSchema(),
		Policy:      mcp.ToolPolicy{ReadOnly: true},
		Handler: func(ctx *mcp.Context) {
			ctx.Send(struct {
				Unix int64  `json:"unix"`
				Time string `json:"time"`
			}{
				Unix: time.Now().Unix(),
				Time: time.Now().Format(time.RFC3339),
			})
		},
	})

	engine.Any("/mcp", gin.WrapH(mcpServer.HTTPHandler()))

	app.WithHttpServer(engine)
	app.Start()
}
