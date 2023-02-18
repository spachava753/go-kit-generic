package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-kit/kit/endpoint"
	httptransport "github.com/go-kit/kit/transport/http"
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

func makeUppercaseEndpoint(svc StringService) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(uppercaseRequest)
		v, err := svc.Uppercase(req.S)
		if err != nil {
			return uppercaseResponse{v, err.Error()}, nil
		}
		return uppercaseResponse{v, ""}, nil
	}
}

func makeCountEndpoint(svc StringService) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(countRequest)
		v := svc.Count(req.S)
		return countResponse{v}, nil
	}
}

func decodeUppercaseRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request uppercaseRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func decodeCountRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request countRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	return request, nil
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	return json.NewEncoder(w).Encode(response)
}

func main() {
	svc := stringService{}

	countHandler := httptransport.NewServer(
		makeCountEndpoint(svc),
		decodeCountRequest,
		encodeResponse,
	)

	createHttpHandler("/uppercase", makeUppercaseEndpoint(svc),
		decodeUppercaseRequest,
		encodeResponse)
	http.Handle("/count", countHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func createHttpHandler(path string, e endpoint.Endpoint,
	dec httptransport.DecodeRequestFunc,
	enc httptransport.EncodeResponseFunc) {
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
