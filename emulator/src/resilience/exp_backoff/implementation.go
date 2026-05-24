package exp_backoff

import (
	"application-emulator/src/generated/client"
	model "application-model"
	"application-model/generated"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ExponentialBackoffImpl struct {
	Initial      time.Duration
	Max          time.Duration
	Multiplier   float64
	MaxAttempts  int
	JitterFactor float64
}

type RequestCallback func(ctx context.Context) (any, error)

func NewExpBackoff(cfg model.ExponentialBackoffConfig) *ExponentialBackoffImpl {
	return &ExponentialBackoffImpl{
		Initial:      time.Duration(cfg.Initial * float64(time.Second)),
		Max:          time.Duration(cfg.Max * float64(time.Second)),
		Multiplier:   cfg.Multiplier,
		MaxAttempts:  cfg.MaxAttempts,
		JitterFactor: 0.2,
	}
}

func (b *ExponentialBackoffImpl) GetDelay(attempt int) time.Duration {
	delay := float64(b.Initial) * math.Pow(b.Multiplier, float64(attempt))

	if d := time.Duration(delay); d > b.Max {
		delay = float64(b.Max)
	}

	if b.JitterFactor > 0 {
		jitter := delay * b.JitterFactor
		delay += (rand.Float64()*2 - 1) * jitter // rango [-jitter, +jitter]
	}

	return time.Duration(math.Max(delay, float64(b.Initial)))
}

// isRetryable decide si el error justifica un reintento.
func isRetryableGRPC(err error) bool {
	if err == nil {
		return false
	}
	code := status.Code(err)
	switch code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
		return true
	}
	return false
}

func isRetryableHTTP(statusCode int) bool {
	// Solo 5xx son retriables; 4xx son errores del cliente
	return statusCode >= 500
}

func (b *ExponentialBackoffImpl) Execute(cb RequestCallback) (any, error) {
	ctx := context.Background()
	var lastErr error

	for attempt := 0; attempt < b.MaxAttempts; attempt++ {

		response, err := cb(ctx)

		if err == nil {
			log.Printf("[RETRY] success attempt=%d/%d", attempt+1, b.MaxAttempts)
			return response, nil
		}

		lastErr = err
		delay := b.GetDelay(attempt)

		log.Printf(
			"[RETRY] failed attempt=%d/%d err=%v next_delay=%v",
			attempt+1, b.MaxAttempts, err, delay,
		)

		if attempt < b.MaxAttempts-1 {
			time.Sleep(delay)
		}
	}

	log.Printf("[RETRY] exhausted attempts=%d last_error=%v", b.MaxAttempts, lastErr)
	return nil, lastErr
}

func (b *ExponentialBackoffImpl) ProxyHTTP(request *http.Request) (*http.Response, error) {
	response, err := b.Execute(func(ctx context.Context) (any, error) {
		req := request.Clone(ctx)

		if request.Body != nil && request.GetBody != nil {
			body, err := request.GetBody()
			if err != nil {
				return nil, err
			}
			req.Body = body
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		if !isRetryableHTTP(resp.StatusCode) && resp.StatusCode >= 400 {
			return resp, nil
		}

		if isRetryableHTTP(resp.StatusCode) {
			resp.Body.Close()
			return nil, fmt.Errorf("retryable status: %d", resp.StatusCode)
		}

		return resp, nil
	})

	if err != nil {
		return nil, err
	}

	httpResponse, ok := response.(*http.Response)
	if !ok {
		return nil, errors.New("unexpected type for HTTP response")
	}

	return httpResponse, nil
}

func (b *ExponentialBackoffImpl) ProxyGRPC(
	conn *grpc.ClientConn,
	service, endpoint string,
	request *generated.Request,
	options ...grpc.CallOption,
) (*generated.Response, error) {
	response, err := b.Execute(func(ctx context.Context) (any, error) {
		resp, err := client.CallGeneratedEndpoint(ctx, conn, service, endpoint, request, options...)
		if err != nil && !isRetryableGRPC(err) {
			return nil, fmt.Errorf("non-retryable: %w", err)
		}
		return resp, err
	})

	if err != nil {
		return nil, err
	}

	grpcResponse, ok := response.(*generated.Response)
	if !ok {
		return nil, errors.New("unexpected type for gRPC response")
	}

	return grpcResponse, nil
}
