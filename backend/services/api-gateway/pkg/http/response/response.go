package response

import (
	"api-gateway/pkg/exception"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel/trace"
)

func Success(c *gin.Context, data any) {
	// 脱敏
	//if err := desense.MaskStruct(data); err != nil {
	//
	//}

	c.JSON(http.StatusOK, newSuccessEnvelope(c, data))
}

func SuccessWithStatus(c *gin.Context, status int, data any) {
	envelope := newSuccessEnvelope(c, data)
	envelope.HttpCode = status
	c.JSON(status, envelope)
}

// 异常情况的数据返回, 返回我们的业务Exception
func Failed(c *gin.Context, err error) {
	// 如果出现多个Handler， 需要通过手动abord
	defer c.Abort()

	var e *exception.ApiException
	if !errors.As(err, &e) {
		// 非可以预期, 没有定义业务的情况
		e = exception.NewApiException(
			http.StatusInternalServerError,
			http.StatusText(http.StatusInternalServerError),
		).WithMessage(err.Error())
		e.HttpCode = http.StatusInternalServerError

	}

	if e.Service == "" {
		e.WithNamespace(viper.GetString("app.name"))
	}
	enrichEnvelope(c, e)
	c.JSON(e.GetHttpCode(), e)
}

func newSuccessEnvelope(c *gin.Context, data any) *exception.ApiException {
	e := &exception.ApiException{
		Service:  viper.GetString("app.name"),
		HttpCode: http.StatusOK,
		Code:     0,
		Reason:   "成功",
		Message:  "success",
		Meta:     map[string]any{},
		Data:     data,
	}
	enrichEnvelope(c, e)
	return e
}

func enrichEnvelope(c *gin.Context, e *exception.ApiException) {
	if e == nil || c == nil || c.Request == nil {
		return
	}
	if e.RequestID == "" {
		e.RequestID = requestID(c)
	}
	if e.TraceID == "" {
		e.TraceID = traceID(c)
	}
}

func requestID(c *gin.Context) string {
	if value := c.GetHeader("X-Request-ID"); value != "" {
		return value
	}
	if value := c.GetHeader("X-Request-Id"); value != "" {
		return value
	}
	if value := c.GetHeader("X-Correlation-ID"); value != "" {
		return value
	}
	return ""
}

func traceID(c *gin.Context) string {
	spanContext := trace.SpanContextFromContext(c.Request.Context())
	if spanContext.HasTraceID() {
		return spanContext.TraceID().String()
	}
	return ""
}
