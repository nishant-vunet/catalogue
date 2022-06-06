package catalogue

// transport.go contains the binding from endpoints to a concrete transport.
// In our case we just use a REST-y HTTP transport.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/log"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/streadway/handy/breaker"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/net/context"
)

// MakeHTTPHandler mounts the endpoints into a REST-y HTTP handler.
func MakeHTTPHandler(ctx context.Context, e Endpoints, imagePath string, logger log.Logger) *mux.Router {
	r := mux.NewRouter().StrictSlash(false)
	options := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
		httptransport.ServerErrorEncoder(encodeError),
	}

	// GET /catalogue       List
	// GET /catalogue/size  Count
	// GET /catalogue/{id}  Get
	// GET /tags            Tags
	// GET /health		Health Check

	r.Methods("GET").Path("/catalogue").Handler(otelhttp.WithRouteTag("/catalogue", httptransport.NewServer(
		circuitbreaker.HandyBreaker(breaker.NewBreaker(0.2))(e.ListEndpoint),
		decodeListRequest,
		encodeListResponse,
		options...)))

	r.Methods("GET").Path("/catalogue/size").Handler(otelhttp.WithRouteTag("/catalogue/size", httptransport.NewServer(
		circuitbreaker.HandyBreaker(breaker.NewBreaker(0.2))(e.CountEndpoint),
		decodeCountRequest,
		encodeResponse,
		options...)))

	r.Methods("GET").Path("/catalogue/{id}").Handler(otelhttp.WithRouteTag("/catalogue/{id}", httptransport.NewServer(
		circuitbreaker.HandyBreaker(breaker.NewBreaker(0.2))(e.GetEndpoint),
		decodeGetRequest,
		encodeGetResponse, // special case, this one can have an error
		options...)))

	r.Methods("GET").Path("/tags").Handler(otelhttp.WithRouteTag("/tags", httptransport.NewServer(
		circuitbreaker.HandyBreaker(breaker.NewBreaker(0.2))(e.TagsEndpoint),
		decodeTagsRequest,
		encodeResponse,
		options...)))

	r.Methods("GET").PathPrefix("/catalogue/images/").Handler(http.StripPrefix(
		"/catalogue/images/",
		http.FileServer(http.Dir(imagePath)),
	))
	r.Methods("GET").Path("/health").Handler(otelhttp.WithRouteTag("/health", httptransport.NewServer(
		circuitbreaker.HandyBreaker(breaker.NewBreaker(0.2))(e.HealthEndpoint),
		decodeHealthRequest,
		encodeHealthResponse,
		options...)))

	r.Handle("/metrics", promhttp.Handler())
	return r
}

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	code := http.StatusInternalServerError
	switch err {
	case ErrNotFound:
		code = http.StatusNotFound
	}
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":       err.Error(),
		"status_code": code,
		"status_text": http.StatusText(code),
	})
}

func decodeListRequest(_ context.Context, r *http.Request) (interface{}, error) {
	pageNum := 1
	if page := r.FormValue("page"); page != "" {
		pageNum, _ = strconv.Atoi(page)
	}
	pageSize := 10
	if size := r.FormValue("size"); size != "" {
		pageSize, _ = strconv.Atoi(size)
	}
	order := "id"
	if sort := r.FormValue("sort"); sort != "" {
		order = strings.ToLower(sort)
	}
	tags := []string{}
	if tagsval := r.FormValue("tags"); tagsval != "" {
		tags = strings.Split(tagsval, ",")
	}
	return listRequest{
		Tags:     tags,
		Order:    order,
		PageNum:  pageNum,
		PageSize: pageSize,
	}, nil
}

// encodeListResponse is distinct from the generic encodeResponse because our
// clients expect that we will encode the slice (array) of socks directly,
// without the wrapping response object.
func encodeListResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(listResponse)
	return encodeResponse(ctx, w, resp.Socks)
}

func decodeCountRequest(_ context.Context, r *http.Request) (interface{}, error) {
	tags := []string{}
	if tagsval := r.FormValue("tags"); tagsval != "" {
		tags = strings.Split(tagsval, ",")
	}
	return countRequest{
		Tags: tags,
	}, nil
}

func decodeGetRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return getRequest{
		ID: mux.Vars(r)["id"],
	}, nil
}

// encodeGetResponse is distinct from the generic encodeResponse because we need
// to special-case when the getResponse object contains a non-nil error.
func encodeGetResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(getResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	return encodeResponse(ctx, w, resp.Sock)
}

func decodeTagsRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return struct{}{}, nil
}

func decodeHealthRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return struct{}{}, nil
}

func encodeHealthResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return encodeResponse(ctx, w, response.(healthResponse))
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	// All of our response objects are JSON serializable, so we just do that.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}
