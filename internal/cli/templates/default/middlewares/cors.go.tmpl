package middlewares

import "github.com/gin-gonic/gin"

func GinCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置响应头中的Access-Control-Allow-Origin为"*"，允许所有域名的跨域请求。
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "false")

		// 如果请求方法是OPTIONS，直接返回200，不继续后续的处理逻辑。
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
		} else {
			c.Next()
		}
	}
}
