package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type StringService interface {
	Uppercase(string) (string, error)
	Count(string) int
}

type stringService struct{}

func (stringService) Uppercase(s string) (string, error) {
	if s == "" {
		return "", ErrEmpty
	}
	return strings.ToUpper(s), nil
}

func (stringService) Count(s string) int {
	return len(s)
}

// ErrEmpty is returned when input string is empty
var ErrEmpty = errors.New("Empty string")

type uppercaseRequest struct {
	S string `json:"s"`
}

type uppercaseResponse struct {
	V   string `json:"v"`
	Err string `json:"err,omitempty"` // errors don't JSON-marshal, so we use a string
}

type countRequest struct {
	S string `json:"s"`
}

type countResponse struct {
	V int `json:"v"`
}

type Endpoint[Req any, Resp any] func(ctx context.Context, request Req) (response Resp, err error)

type DecodeRequestFunc[Req any] func(context.Context, *http.Request) (request Req, err error)

type EncodeResponseFunc[Resp any] func(context.Context, http.ResponseWriter, Resp) error

func makeUppercaseEndpoint(svc StringService) Endpoint[uppercaseRequest, uppercaseResponse] {
	return func(_ context.Context, request uppercaseRequest) (uppercaseResponse, error) {
		v, err := svc.Uppercase(request.S)
		if err != nil {
			return uppercaseResponse{v, err.Error()}, nil
		}
		return uppercaseResponse{v, ""}, nil
	}
}

func makeCountEndpoint(svc StringService) Endpoint[countRequest, countResponse] {
	return func(_ context.Context, request countRequest) (countResponse, error) {
		v := svc.Count(request.S)
		return countResponse{v}, nil
	}
}

func decodeUppercaseRequest(_ context.Context, r *http.Request) (uppercaseRequest, error) {
	var request uppercaseRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return uppercaseRequest{}, err
	}
	return request, nil
}

func decodeCountRequest(_ context.Context, r *http.Request) (countRequest, error) {
	var request countRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return countRequest{}, err
	}
	return request, nil
}

func encodeUppercaseResponse(_ context.Context, w http.ResponseWriter, response uppercaseResponse) error {
	return json.NewEncoder(w).Encode(response)
}

func encodeCountResponse(_ context.Context, w http.ResponseWriter, response countResponse) error {
	return json.NewEncoder(w).Encode(response)
}

func main() {
	svc := stringService{}

	createHttpHandler("/uppercase", makeUppercaseEndpoint(svc),
		decodeUppercaseRequest,
		encodeUppercaseResponse)
	createHttpHandler("/count", makeCountEndpoint(svc),
		decodeCountRequest,
		encodeCountResponse)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createHttpHandler[Req any, Resp any](path string, e Endpoint[Req, Resp],
	dec DecodeRequestFunc[Req],
	enc EncodeResponseFunc[Resp]) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		request, err := dec(ctx, r)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("err: %s", err)))
			return
		}

		response, err := e(ctx, request)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("err: %s", err)))
			return
		}

		if writeErr := enc(ctx, w, response); writeErr != nil {
			w.Write([]byte(fmt.Sprintf("err: %s", writeErr)))
			return
		}
	})
}
