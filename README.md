# Asyncreq
a library to create a asynchronous endpoint with redis backend.

### How to use
```go

handler := &PostHandler{}
// this will put the request on to the persistence layer  
requestId, putErr := handler.putRequest(jsonPayloadStr)

if putErr != nil {
	return notOk
}

return ok

```

