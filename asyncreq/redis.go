package asyncreq

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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
	val := &AsyncRequestData{
		RequestId:          key,
		RequestPayload:     request.RequestPayload,
		ResponsePayload:    "",
		CreatedAt:          time.Now().UnixMilli(),
		IsResponseFinished: false,
		IsResponseError:    false,
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
	log.Debug().
		Msgf(
			"Publishing request to redis channel=%v data=%v",
			r.PostRequestOptions.RedisChannelName,
			string(valBytes))
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

			asyncRequestDataResult := r.OnPostRequest(ctx, postRequest)
			postResponse := r.OnPostRequestCompleted(ctx, postRequest, asyncRequestDataResult)

			// then replace result in the cache
			cmd := r.RedisClient.Get(ctx, postResponse.RequestId)
			if cmd.Err() != nil {
				log.Warn().
					Msgf("Error getting redis key=%v err=%v", postResponse.RequestId, cmd.Err())
				r.OnPostError(ctx, cmd.Err())
				continue
			}

			asyncRequestDataFromCacheStr := cmd.Val()
			var asyncRequestDataFromCache = &AsyncRequestData{}

			getRequestUnmarshallErr := json.Unmarshal([]byte(asyncRequestDataFromCacheStr), asyncRequestDataFromCache)
			if getRequestUnmarshallErr != nil {
				log.Warn().
					Msgf(
						"Error unmarshalling from redis key=%v err=%v",
						postResponse.RequestId,
						getRequestUnmarshallErr)
				r.OnPostError(ctx, unmarshalErr)
				continue
			}

			newAsyncRequestData := &AsyncRequestData{
				RequestId:          asyncRequestDataFromCache.RequestId,
				RequestPayload:     asyncRequestDataFromCache.RequestPayload,
				ResponsePayload:    asyncRequestDataResult.ResponsePayload,
				CreatedAt:          asyncRequestDataFromCache.CreatedAt,
				IsResponseFinished: true,
				IsResponseError:    asyncRequestDataResult.IsResponseError,
			}

			newAsyncRequestDataBytes, marshalErr := json.Marshal(newAsyncRequestData)

			if marshalErr != nil {
				log.Warn().
					Msgf(
						"Error marshaling asyncRequestData key=%v err=%v",
						newAsyncRequestData,
						marshalErr)
				r.OnPostError(ctx, marshalErr)
				continue
			}

			setCmd := r.RedisClient.Set(ctx, newAsyncRequestData.RequestId, string(newAsyncRequestDataBytes), r.PostRequestOptions.Ttl)

			if setCmd.Err() != nil {
				log.Warn().
					Msgf(
						"Error setting redis value key=%v val=%v err=%v",
						newAsyncRequestData.RequestId,
						string(newAsyncRequestDataBytes),
						setCmd.Err())
				r.OnPostError(ctx, cmd.Err())
				continue
			}
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
	var asyncRequestData = &AsyncRequestData{}

	unmarshalErr := json.Unmarshal([]byte(postRequestInternalStr), asyncRequestData)
	if unmarshalErr != nil {
		return r.OnGetError(ctx, unmarshalErr)
	}

	if !asyncRequestData.IsResponseFinished {
		return GetResponse{
			IsRequestFinished: false,
			ResponsePayload:   "",
		}
	}

	return GetResponse{
		IsRequestFinished: true,
		ResponsePayload:   asyncRequestData.ResponsePayload,
	}
}
