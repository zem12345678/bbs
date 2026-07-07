package middleware

import (
	"strconv"

	"credit-service/pkg/exception"
	"google.golang.org/grpc/metadata"
)

var (
	// Namespace todo
	Namespace = "default"
)

const (
	// ResponseCodeHeader todo
	ResponseCodeHeader = "x-rpc-code"
	// ResponseReasonHeader todo
	ResponseReasonHeader = "x-rpc-reason"
	// ResponseDescHeader todo
	ResponseDescHeader = "x-rpc-desc"
	// ResponseMetaHeader todo
	ResponseMetaHeader = "x-rpc-meta"
	// ResponseDataHeader todo
	ResponseDataHeader = "x-rpc-data"
)

// NewExceptionFromTrailer todo
func NewExceptionFromTrailer(md metadata.MD, err error) *exception.ApiException {
	ctx := newGrpcCtx(md)
	code, _ := strconv.Atoi(ctx.get(ResponseCodeHeader))
	reason := ctx.get(ResponseReasonHeader)
	message := ctx.get(ResponseDescHeader)
	ctx.get(ResponseMetaHeader)
	ctx.get(ResponseDataHeader)
	if message == "" {
		message = err.Error()
	}
	return exception.NewApiException(code, reason).WithMessage(message).WithNamespace(Namespace)
}

type grpcCtx struct {
	md metadata.MD
}

func newGrpcCtx(md metadata.MD) grpcCtx {
	return grpcCtx{md: md}
}

func (c grpcCtx) get(key string) string {
	values := c.md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
