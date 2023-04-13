package asyncreq

import (
	"context"
	"github.com/go-redis/redis/v8"
	"time"
)

type (
	PostRequestRedisOptions struct {
		Ttl              time.Duration
		RedisChannelName string
	}

	PostRequest struct {
		RequestPayload string `json:"request_payload"`
	}

	PostResponse struct {
		IsError      bool   `json:"is_error"`
		ErrorMessage string `json:"error_message"`
		RequestId    string `json:"request_id"`
	}

	GetResponse struct {
		IsRequestFinished bool   `json:"is_request_finished"`
		ResponsePayload   string `json:"response_payload"`
	}

	AsyncRequestData struct {
		RequestId          string `json:"request_id"`
		RequestPayload     string `json:"request_payload"`
		ResponsePayload    string `json:"response_payload"`
		CreatedAt          int64  `json:"created_at"`
		IsResponseFinished bool   `json:"is_response_finished"`
		IsResponseError    bool   `json:"is_response_error"`
	}

	RedisPostHandler struct {
		RedisClient            *redis.Client
		PostRequestOptions     PostRequestRedisOptions
		OnPostRequest          OnPostRequest
		OnPostRequestCompleted OnPostRequestCompleted
		OnPostError            OnPostError
	}

	RedisGetHandler struct {
		RedisClient *redis.Client
		OnGetError  OnGetError
	}

	// OnPostRequest is a function that will be executed after async request successfully queued
	OnPostRequest func(ctx context.Context, request *PostRequest) AsyncRequestData

	// OnPostRequestCompleted is a function that will be executed after async request successfully executed
	OnPostRequestCompleted func(ctx context.Context, request *PostRequest, data AsyncRequestData) PostResponse

	// OnPostError is a function that will be executed after async request failed to be executed
	OnPostError func(ctx context.Context, err error) PostResponse

	// OnGetError is a function that will be executed if get / describe request status failed
	OnGetError func(ctx context.Context, err error) GetResponse

	PostHandler interface {
		Do(request PostRequest) PostResponse
		DoWtCtx(ctx context.Context, request PostRequest) PostResponse
	}

	GetHandler interface {
		Do(requestId string) GetResponse
		DoWtCtx(ctx context.Context, requestId string) GetResponse
	}
)
