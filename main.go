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

func (u uppercaseRequest) String() string {
	return fmt.Sprintf("S: %s", u.S)
}

type uppercaseResponse struct {
	V   string `json:"v"`
	Err string `json:"err,omitempty"` // errors don't JSON-marshal, so we use a string
}

func (u uppercaseResponse) String() string {
	return fmt.Sprintf("V: %s", u.V)
}

type countRequest struct {
	S string `json:"s"`
}

type countResponse struct {
	V int `json:"v"`
}

type Endpoint[Req any, Resp any] func(ctx context.Context, request Req) (response Resp, err error)

type Middleware[Req any, Resp any] func(Endpoint[Req, Resp]) Endpoint[Req, Resp]

func Chain[Req any, Resp any](outer Middleware[Req, Resp], others ...Middleware[Req, Resp]) Middleware[Req, Resp] {
	return func(next Endpoint[Req, Resp]) Endpoint[Req, Resp] {
		for i := len(others) - 1; i >= 0; i-- { // reverse
			next = others[i](next)
		}
		return outer(next)
	}
}

func annotate[Req any, Resp any](s string) Middleware[Req, Resp] {
	return func(next Endpoint[Req, Resp]) Endpoint[Req, Resp] {
		return func(ctx context.Context, request Req) (Resp, error) {
			fmt.Println(s, "pre")
			defer fmt.Println(s, "post")
			return next(ctx, request)
		}
	}
}

func logIt[Req fmt.Stringer, Resp fmt.Stringer]() Middleware[Req, Resp] {
	return func(next Endpoint[Req, Resp]) Endpoint[Req, Resp] {
		return func(ctx context.Context, request Req) (Resp, error) {
			fmt.Printf("endpoint middleware req: %s\n", request.String())
			return next(ctx, request)
		}
	}
}

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

	uppercaseEndpoint := Chain[uppercaseRequest, uppercaseResponse](
		annotate[uppercaseRequest, uppercaseResponse]("first"),
		annotate[uppercaseRequest, uppercaseResponse]("second"),
		annotate[uppercaseRequest, uppercaseResponse]("third"),
		logIt[uppercaseRequest, uppercaseResponse](),
	)(makeUppercaseEndpoint(svc))
	createHttpHandler("/uppercase", uppercaseEndpoint,
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
