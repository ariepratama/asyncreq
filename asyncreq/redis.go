package asyncreq

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"time"
)

func NewRedisPostHandler(redisClient *redis.Client, PostRequestOptions PostRequestRedisOptions, OnPostRequest OnPostRequest, OnPostRequestCompleted OnPostRequestCompleted, OnPostError OnPostError) RedisPostHandler {
	redisPostHandler := RedisPostHandler{
		RedisClient:            redisClient,
		PostRequestOptions:     PostRequestOptions,
		OnPostRequest:          OnPostRequest,
		OnPostRequestCompleted: OnPostRequestCompleted,
		OnPostError:            OnPostError,
	}
	redisPostHandler.initSubscriber()
	return redisPostHandler
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

	// set the value in redis so it can be polled using get handler
	cmd := r.RedisClient.Set(ctx, key, string(valBytes), r.PostRequestOptions.Ttl)

	if cmd.Err() != nil {
		return r.OnPostError(ctx, cmd.Err())
	}

	// publish using redis pubsub, such that the handler could consume and process it
	r.RedisClient.Publish(ctx, r.PostRequestOptions.RedisChannelName, string(valBytes))

	return PostResponse{
		IsError:      false,
		ErrorMessage: "",
		RequestId:    key,
	}
}

func (r RedisPostHandler) initSubscriber() {
	ctx := context.Background()
	pubSub := r.RedisClient.Subscribe(ctx, r.PostRequestOptions.RedisChannelName)
	channel := pubSub.Channel()
	go func() {
		for msg := range channel {
			// message payload should be in a form of PostRequest
			redisMessagePayload := msg.Payload
			var postRequest = &PostRequest{}
			unmarshalErr := json.Unmarshal([]byte(redisMessagePayload), postRequest)

			if unmarshalErr != nil {
				r.OnPostError(ctx, unmarshalErr)
			}

			postResponse := r.OnPostRequest(ctx, postRequest)
			postResponse = r.OnPostRequestCompleted(ctx, postRequest, postResponse)
		}
	}()

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
