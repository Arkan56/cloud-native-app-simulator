package timeout

import (
	"application-emulator/src/generated/client"
	model "application-model"
	"application-model/generated"
	"context"
	"errors"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

type RequestCallback func(ctx context.Context) (any, error)

type TimeoutImpl struct {
	Duration time.Duration
}

func NewTimeout(cfg model.TimeoutConfig) *TimeoutImpl {
	return &TimeoutImpl{
		Duration: time.Duration(cfg.Duration * float64(time.Second)),
	}
}

func (t *TimeoutImpl) Execute(cb RequestCallback) (any, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		t.Duration,
	)

	defer cancel()

	done := make(chan struct{})

	var (
		response any
		err      error
	)

	go func() {
		response, err = cb(ctx)
		close(done)
	}()

	select {
	case <-done:
		return response, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *TimeoutImpl) ProxyHTTP(request *http.Request) (*http.Response, error) {

	response, err := t.Execute(func(ctx context.Context) (any, error) {
		req := request.WithContext(ctx)
		return http.DefaultClient.Do(req)
	})

	if err != nil {
		return nil, err
	}

	httpResponse, ok := response.(*http.Response)
	if !ok {
		return nil, errors.New("HTTP response from Timeout broken")
	}

	return httpResponse, nil
}

func (t *TimeoutImpl) ProxyGRPC(conn *grpc.ClientConn, service, endpoint string, request *generated.Request, options ...grpc.CallOption) (*generated.Response, error) {
	response, err := t.Execute(func(ctx context.Context) (any, error) {
		return client.CallGeneratedEndpoint(ctx, conn, service, endpoint, request, options...)
	})

	if err != nil {
		return nil, err
	}
	grpcResponse, ok := response.(*generated.Response)
	if !ok {
		return nil, errors.New("GRPC response from Timeout broken")
	}

	return grpcResponse, err
}
