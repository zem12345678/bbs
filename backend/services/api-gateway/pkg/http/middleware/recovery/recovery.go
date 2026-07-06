package recovery

import (
	"api-gateway/pkg/exception"
	"api-gateway/pkg/http/response"
	"fmt"
	"net/http/httputil"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const recoveryExplanation = "Something went wrong"

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var fields []zap.Field
				fields = append(fields, zap.Any("panic", r), zap.ByteString("stack_trace", debug.Stack()),
					zap.StackSkip("stack", 3))
				if gin.IsDebugging() {
					httpRequest, _ := httputil.DumpRequest(ctx.Request, false)
					headers := strings.Split(string(httpRequest), "\r\n")
					for idx, header := range headers {
						current := strings.Split(header, ":")
						if current[0] == "Authorization" {
							headers[idx] = current[0] + ": *"
						}
					}
					fields = append(fields, zap.Strings("headers", headers))
				}
				msg := fmt.Sprintf("%s. Recovering, but please report this at %s.", recoveryExplanation,
					time.Now().Format("2006/01/02 - 15:04:05"))

				// 记录Panic日志
				logger.Panic("msg", fields...)
				// 返回500报错
				response.Failed(ctx, exception.NewInternalServerError("%s", msg))
				return
			}
		}()
		ctx.Next()
	}
}
