package middleware

import (
	"bytes"
	"io"
	Log "live-chatter/pkg/logger"

	"github.com/gin-gonic/gin"
)

func RequestDumpMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		Log.Info(
			"[Request]\n"+
				"\tMethod: %s\n"+
				"\tURL: %s\n"+
				"\tHeaders: %v\n"+
				"\tBody: %s",
			c.Request.Method,
			c.Request.URL.String(),
			c.Request.Header,
			string(bodyBytes),
		)

		c.Next()
	}
}
