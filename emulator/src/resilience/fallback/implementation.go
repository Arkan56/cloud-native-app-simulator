package fallback

import (
	"application-emulator/src/generated/client"
	model "application-model"
	"application-model/generated"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	TypeStatic  = "static"
	TypeService = "service"
)

type FallbackImpl struct {
	Config model.FallbackConfig
}

func NewFallback(cfg model.FallbackConfig) *FallbackImpl {
	return &FallbackImpl{
		Config: cfg,
	}
}

// ExecuteHTTP executes the configured HTTP fallback strategy.
func (f *FallbackImpl) ExecuteHTTP(
	payload []byte,
	headers http.Header,
	endpoint string) (*http.Response, error) {

	log.Printf(
		"[FALLBACK HTTP] executing type=%s endpoint=%s",
		f.Config.Type,
		endpoint,
	)

	switch f.Config.Type {

	case TypeStatic:

		log.Printf(
			"[FALLBACK HTTP] using static response code=%d message=%s",
			f.Config.ResponseCode,
			f.Config.ResponseMessage,
		)

		fallbackResponse := &generated.Response{
			Endpoint: endpoint,
			Message:  f.Config.ResponseMessage,
			Tasks:    nil,
		}

		data, err := protojson.Marshal(fallbackResponse)
		if err != nil {
			return nil, err
		}

		response := &http.Response{
			StatusCode: f.Config.ResponseCode,
			Body:       io.NopCloser(bytes.NewBuffer(data)),
			Header:     make(http.Header),
		}

		response.Header.Set(
			"Content-Type",
			"application/json",
		)

		return response, nil

	case TypeService:

		var url string

		log.Printf(
			"[FALLBACK HTTP] forwarding request to fallback service=%s",
			url,
		)

		if f.Config.FallbackPort == 0 {
			url = fmt.Sprintf(
				"http://%s/%s",
				f.Config.FallbackService,
				f.Config.FallbackEndpoint,
			)
		} else {
			url = fmt.Sprintf(
				"http://%s:%d/%s",
				f.Config.FallbackService,
				f.Config.FallbackPort,
				f.Config.FallbackEndpoint,
			)
		}

		request, err := http.NewRequest(
			http.MethodPost,
			url,
			bytes.NewBuffer(payload),
		)

		if err != nil {
			return nil, err
		}

		// Forward original headers
		for key, values := range headers {
			for _, value := range values {
				request.Header.Add(key, value)
			}
		}

		response, err := http.DefaultClient.Do(request)

		if err != nil {
			log.Printf(
				"[FALLBACK HTTP] fallback request failed err=%v",
				err,
			)
			return nil, err
		}

		log.Printf(
			"[FALLBACK HTTP] fallback request succeeded status=%d",
			response.StatusCode,
		)

		return response, nil

	default:

		return nil, fmt.Errorf(
			"invalid fallback type '%s'",
			f.Config.Type,
		)
	}
}

// ExecuteGRPC executes the configured gRPC fallback strategy.
func (f *FallbackImpl) ExecuteGRPC(
	payload string,
	endpoint string,
	callOptions ...grpc.CallOption,
) (*generated.Response, error) {

	log.Printf(
		"[FALLBACK GRPC] executing type=%s endpoint=%s",
		f.Config.Type,
		endpoint,
	)

	switch f.Config.Type {

	case TypeStatic:

		log.Printf(
			"[FALLBACK GRPC] using static response message=%s",
			f.Config.ResponseMessage,
		)

		return &generated.Response{
			Endpoint: endpoint,
			Message:  f.Config.ResponseMessage,
			Tasks:    nil,
		}, nil

	case TypeService:

		var url string

		log.Printf(
			"[FALLBACK GRPC] connecting to fallback service=%s",
			url,
		)

		if f.Config.FallbackPort == 0 {
			url = f.Config.FallbackService
		} else {
			url = fmt.Sprintf(
				"%s:%d",
				f.Config.FallbackService,
				f.Config.FallbackPort,
			)
		}

		conn, err := grpc.Dial(
			url,
			grpc.WithTransportCredentials(
				insecure.NewCredentials(),
			),
		)

		if err != nil {
			log.Printf(
				"[FALLBACK GRPC] failed to connect err=%v",
				err,
			)
			return nil, err
		}

		log.Printf(
			"[FALLBACK GRPC] grpc connection established",
		)

		defer conn.Close()

		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)

		defer cancel()

		request := &generated.Request{
			Payload: payload,
		}

		response, err := client.CallGeneratedEndpoint(
			ctx,
			conn,
			f.Config.FallbackService,
			f.Config.FallbackEndpoint,
			request,
			callOptions...,
		)

		if err != nil {
			log.Printf(
				"[FALLBACK GRPC] fallback request failed err=%v",
				err,
			)
			return nil, err
		}

		log.Printf(
			"[FALLBACK GRPC] fallback request succeeded",
		)

		return response, nil

	default:

		return nil, fmt.Errorf(
			"invalid fallback type '%s'",
			f.Config.Type,
		)
	}
}
