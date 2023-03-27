package core

import "context"

type (
	PostRequest struct {
		Payload string `json:"payload"`
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

	PostRequestInternal struct {
		RequestPayload     string `json:"request_payload"`
		ResponsePayload    string `json:"response_payload"`
		CreatedAt          int64  `json:"created_at"`
		IsResponseFinished bool   `json:"is_response_finished"`
	}

	ProcessReqFunCallback func(request PostRequest)
	ProcessReqFun         func(ctx context.Context, request PostRequest, callback ProcessReqFunCallback)

	PostHandler interface {
		Do(request PostRequest) PostResponse
		DoWtCtx(ctx context.Context, request PostRequest) PostResponse
	}

	GetHandler interface {
		Do(requestId string) GetResponse
		DoWtCtx(requestId string) GetResponse
	}
)
