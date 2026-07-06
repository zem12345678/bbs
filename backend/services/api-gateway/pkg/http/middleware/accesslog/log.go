package accesslog

import (
	"fmt"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func GinZap(logger *zap.Logger, timeFormat string, utc bool) gin.HandlerFunc {
	return GinZapWithConfig(logger, &ginzap.Config{TimeFormat: timeFormat, UTC: utc})
}

func GinZapWithConfig(logger *zap.Logger, conf *ginzap.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		path := c.Request.URL.Path
		fields := []zapcore.Field{
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("hostname", c.Request.Host),
			zap.String("startTime", start.Format("2006-01-02 15:04:05")),
			zap.String("Duration", time.Since(start).String()),
		}
		if len(c.Errors) > 0 {
			for _, e := range c.Errors.Errors() {
				logger.Error(e, fields...)
			}
		} else {
			logger.Info(fmt.Sprintf(`%s %d`, path, status), fields...)
		}
	}
}
