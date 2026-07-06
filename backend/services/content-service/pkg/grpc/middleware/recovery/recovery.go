package recovery

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RecoveryExplanation 异常消息
const RecoveryExplanation = "Something went wrong"

func StreamRecoverInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("%s. Recovering, but please report this.", RecoveryExplanation)
				log.Printf("%+v\n\n%s", r, debug.Stack())
				// 返回500报错
				err = status.Errorf(codes.Internal, "%v", msg)
				return
			}
		}()

		return handler(srv, stream)
	}
}

// UnaryRecoverInterceptor catches panics in processing unary requests and recovers.
// func UnaryRecoverInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo,
// 	handler grpc.UnaryHandler) (resp interface{}, err error) {
// 	defer handleCrash(func(r interface{}) {
// 		err = toPanicError(r)
// 	})

// 	return handler(ctx, req)
// }

func UnaryRecoverInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp interface{}, err error) {
		//defer handleCrash(func(r interface{}) {
		//	err = toPanicError(r)
		//})
		defer func() {
			if r := recover(); r != nil {
				msg := fmt.Sprintf("%s. Recovering, but please report this.", RecoveryExplanation)
				log.Printf("%+v\n\n%s", r, debug.Stack())
				// 返回500报错
				err = status.Errorf(codes.Internal, "%v", msg)
				return
			}
		}()

		return handler(ctx, req)
	}
}

func handleCrash(handler func(interface{})) {
	if r := recover(); r != nil {
		handler(r)
	}
}

func toPanicError(r interface{}) error {
	log.Printf("%+v\n\n%s", r, debug.Stack())
	return status.Errorf(codes.Internal, "panic: %v", r)
}
