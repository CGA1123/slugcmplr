package obs

import (
	"context"
	"errors"

	"github.com/twitchtv/twirp"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// TwirpOtelInterceptor returns a twirp.Interceptor which adds otel RPC
// attributes to the current HTTP span.
func TwirpOtelInterceptor() twirp.Interceptor {
	return func(n twirp.Method) twirp.Method {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			span := trace.SpanFromContext(ctx)

			pkg, _ := twirp.PackageName(ctx)
			svc, _ := twirp.ServiceName(ctx)
			mtd, _ := twirp.MethodName(ctx)
			fqn := pkg + "." + mtd + "/" + mtd

			span.SetAttributes(
				semconv.RPCSystemKey.String("twirp"),
				semconv.RPCServiceKey.String(svc),
				semconv.RPCMethodKey.String(mtd),
				attribute.String("rpc.package", pkg),
				attribute.String("rpc.fqn", fqn),
			)

			res, err := n(ctx, req)
			if err != nil {
				var terr twirp.Error
				if errors.As(err, &terr) {
					span.SetAttributes(
						attribute.String("rpc.error_code", string(terr.Code())),
						attribute.String("rpc.error_message", terr.Msg()),
					)
				} else {
					span.SetAttributes(
						attribute.String("rpc.error_code", "other"),
						attribute.String("rpc.error_message", err.Error()),
					)
				}
			}

			return res, err
		}
	}
}
