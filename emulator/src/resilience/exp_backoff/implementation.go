package exp_backoff

import (
	"application-emulator/src/generated/client"
	model "application-model"
	"application-model/generated"
	"context"
	"errors"
	"math"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExponentialBackoffImpl struct {
	Initial     time.Duration
	Max         time.Duration
	Multiplier  float64
	MaxAttempts int
}

var GRPC_ERROR = status.Error(codes.Unavailable, "Service unavailable")
var HTTP_ERROR = errors.New("Service unavailable")

type RequestCallback func(ctx context.Context) (any, error)

func NewExpBackoff(cfg model.ExponentialBackoffConfig) *ExponentialBackoffImpl {
	return &ExponentialBackoffImpl{
		Initial:     time.Duration(cfg.Initial * float64(time.Second)),
		Max:         time.Duration(cfg.Max * float64(time.Second)),
		Multiplier:  cfg.Multiplier,
		MaxAttempts: cfg.MaxAttempts,
	}
}

func (b *ExponentialBackoffImpl) GetDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return b.Initial
	}

	delay := float64(b.Initial) * math.Pow(b.Multiplier, float64(attempt))
	d := time.Duration(delay)

	if d > b.Max {
		return b.Max
	}

	return d
}

func (b *ExponentialBackoffImpl) Execute(cb RequestCallback) (any, error) {
	var lastErr error

	for attemp := 0; attemp < b.MaxAttempts; attemp++ {
		response, err := cb(context.Background())

		if err == nil {
			return response, nil
		}

		lastErr = err

		delay := b.GetDelay(attemp)

		time.Sleep(delay)
	}

	return nil, lastErr
}

func (b *ExponentialBackoffImpl) ProxyHTTP(request *http.Request) (*http.Response, error) {

	response, err := b.Execute(func(ctx context.Context) (any, error) {
		req := request.WithContext(ctx)
		return http.DefaultClient.Do(req)
	})

	if err != nil {
		return nil, err
	}

	httpResponse, ok := response.(*http.Response)
	if !ok {
		return nil, errors.New("HTTP response from Retry broken")
	}

	return httpResponse, nil
}

func (b *ExponentialBackoffImpl) ProxyGRPC(conn *grpc.ClientConn, service, endpoint string, request *generated.Request, options ...grpc.CallOption) (*generated.Response, error) {
	response, err := b.Execute(func(ctx context.Context) (any, error) {
		return client.CallGeneratedEndpoint(ctx, conn, service, endpoint, request, options...)
	})

	if err != nil {
		return nil, err
	}
	grpcResponse, ok := response.(*generated.Response)
	if !ok {
		return nil, errors.New("GRPC response from Retry broken")
	}

	return grpcResponse, err
}
