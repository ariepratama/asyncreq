package asyncreq

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"time"
)

func NewRedisPostHandler(redisClient *redis.Client, PostRequestOptions PostRequestRedisOptions, OnPostRequest OnPostRequest, OnPostRequestCompleted OnPostRequestCompleted, OnPostError OnPostError) RedisPostHandler {
	onPostRequestCallback := func(ctx context.Context, request PostRequest) PostResponse {
		return OnPostRequestCompleted(ctx, request)
	}

	return RedisPostHandler{
		RedisClient:        redisClient,
		PostRequestOptions: PostRequestOptions,
		OnPostRequest: func(ctx context.Context, request PostRequest) {
			OnPostRequest(ctx, request)
			go onPostRequestCallback(ctx, request)
		},
		OnPostRequestCompleted: OnPostRequestCompleted,
		OnPostError:            OnPostError,
	}
}

func NewRedisGetHandler(redisClient *redis.Client, OnGetError OnGetError) RedisGetHandler {
	return RedisGetHandler{
		RedisClient: redisClient,
		OnGetError:  OnGetError,
	}
}

func (r RedisPostHandler) Do(request PostRequest) PostResponse {
	return r.DoWtCtx(context.Background(), request)
}

func (r RedisPostHandler) DoWtCtx(ctx context.Context, request PostRequest) PostResponse {
	requestId, _ := uuid.NewUUID()
	key := requestId.String()
	val := &PostRequestInternal{
		RequestPayload:     request.Payload,
		ResponsePayload:    "",
		CreatedAt:          time.Now().UnixMilli(),
		IsResponseFinished: false,
	}

	valBytes, marshalErr := json.Marshal(val)

	if marshalErr != nil {
		return r.OnPostError(ctx, marshalErr)
	}

	cmd := r.RedisClient.Set(ctx, key, string(valBytes), r.PostRequestOptions.ttl)

	if cmd.Err() != nil {
		return r.OnPostError(ctx, cmd.Err())
	}

	go r.OnPostRequest(ctx, request)
	return PostResponse{
		IsError:      false,
		ErrorMessage: "",
		RequestId:    key,
	}
}

func (r RedisGetHandler) Do(requestId string) GetResponse {
	return r.DoWtCtx(context.Background(), requestId)
}

func (r RedisGetHandler) DoWtCtx(ctx context.Context, requestId string) GetResponse {
	cmd := r.RedisClient.Get(ctx, requestId)
	if cmd.Err() != nil {
		return r.OnGetError(ctx, cmd.Err())
	}

	postRequestInternalStr := cmd.Val()
	var postRequestInternal = &PostRequestInternal{}

	unmarshalErr := json.Unmarshal([]byte(postRequestInternalStr), postRequestInternal)
	if unmarshalErr != nil {
		return r.OnGetError(ctx, unmarshalErr)
	}

	if !postRequestInternal.IsResponseFinished {
		return GetResponse{
			IsRequestFinished: false,
			ResponsePayload:   "",
		}
	}

	return GetResponse{
		IsRequestFinished: true,
		ResponsePayload:   postRequestInternal.ResponsePayload,
	}
}
