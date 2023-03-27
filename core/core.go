package core

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"time"
)

type OnPostRequest struct {
	callback      ProcessReqFunCallback
	processReqFun ProcessReqFun
}

func (o OnPostRequest) DoWtCtx(ctx context.Context, request PostRequest) {
	go o.processReqFun(ctx, request, o.callback)
}

type RedisPostHandler struct {
	redisClient   *redis.Client
	onPostRequest OnPostRequest
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
	cmd := r.redisClient.Set(ctx, key, val, time.Second*30)

	if cmd.Err() != nil {
		return PostResponse{
			IsError:      true,
			ErrorMessage: cmd.Err().Error(),
			RequestId:    key,
		}
	}

	r.onPostRequest.DoWtCtx(ctx, request)

	return PostResponse{
		IsError:      false,
		ErrorMessage: "",
		RequestId:    key,
	}
}

type RedisGetHandler struct {
	redisClient *redis.Client
}

func (r RedisGetHandler) handle(requestId string) GetResponse {
	return r.handleWtCtx(context.Background(), requestId)
}

func (r RedisGetHandler) handleWtCtx(ctx context.Context, requestId string) GetResponse {
	cmd := r.redisClient.Get(ctx, requestId)
	if cmd.Err() != nil {
		//TODO implement
		panic("unhandled getting value from redis")
	}

	postRequestInternalStr := cmd.Val()
	var postRequestInternal *PostRequestInternal

	unmarshalErr := json.Unmarshal([]byte(postRequestInternalStr), postRequestInternal)
	if unmarshalErr != nil {
		//TODO implement
		panic("unhandled unmarshalling error")
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
