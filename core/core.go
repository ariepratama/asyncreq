package core

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"time"
)

type OnPostRequest struct {
	Callback      ProcessReqFunCallback
	ProcessReqFun ProcessReqFun
}

func (o OnPostRequest) DoWtCtx(ctx context.Context, request PostRequest) {
	go o.ProcessReqFun(ctx, request, o.Callback)
}

type RedisPostHandler struct {
	RedisClient   *redis.Client
	OnPostRequest OnPostRequest
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
		return PostResponse{
			IsError:      true,
			ErrorMessage: marshalErr.Error(),
			RequestId:    key,
		}
	}

	cmd := r.RedisClient.Set(ctx, key, string(valBytes), time.Second*30)

	if cmd.Err() != nil {
		return PostResponse{
			IsError:      true,
			ErrorMessage: cmd.Err().Error(),
			RequestId:    key,
		}
	}

	r.OnPostRequest.DoWtCtx(ctx, request)

	return PostResponse{
		IsError:      false,
		ErrorMessage: "",
		RequestId:    key,
	}
}

type RedisGetHandler struct {
	RedisClient *redis.Client
}

func (r RedisGetHandler) Do(requestId string) GetResponse {
	return r.DoWtCtx(context.Background(), requestId)
}

func (r RedisGetHandler) DoWtCtx(ctx context.Context, requestId string) GetResponse {
	cmd := r.RedisClient.Get(ctx, requestId)
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
