package catalogue

// endpoints.go contains the endpoint definitions, including per-method request
// and response structs. Endpoints are the binding between the service and
// transport.

import (
	"fmt"
	"github.com/go-kit/kit/endpoint"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/net/context"

	"go.opentelemetry.io/contrib/instrumentation/github.com/go-kit/kit/otelkit"
)

// Endpoints collects the endpoints that comprise the Service.
type Endpoints struct {
	ListEndpoint   endpoint.Endpoint
	CountEndpoint  endpoint.Endpoint
	GetEndpoint    endpoint.Endpoint
	TagsEndpoint   endpoint.Endpoint
	HealthEndpoint endpoint.Endpoint
}

var tracer = otel.Tracer("catalogue")

// MakeEndpoints returns an Endpoints structure, where each endpoint is
// backed by the given service.
func MakeEndpoints(s Service) Endpoints {
	return Endpoints{
		ListEndpoint:   otelkit.EndpointMiddleware(otelkit.WithOperation("list catalogue"))(MakeListEndpoint(s)),
		CountEndpoint:  otelkit.EndpointMiddleware(otelkit.WithOperation("count catalogue"))(MakeCountEndpoint(s)),
		GetEndpoint:    otelkit.EndpointMiddleware(otelkit.WithOperation("get catalogue"))(MakeGetEndpoint(s)),
		TagsEndpoint:   otelkit.EndpointMiddleware(otelkit.WithOperation("tags catalogue"))(MakeTagsEndpoint(s)),
		HealthEndpoint: otelkit.EndpointMiddleware(otelkit.WithOperation("health check"))(MakeHealthEndpoint(s)),
		//		ListEndpoint:   opentracing.TraceServer(tracer, "GET /catalogue")(MakeListEndpoint(s)),
		//		CountEndpoint:  opentracing.TraceServer(tracer, "GET /catalogue/size")(MakeCountEndpoint(s)),
		//		GetEndpoint:    opentracing.TraceServer(tracer, "GET /catalogue/{id}")(MakeGetEndpoint(s)),
		//		TagsEndpoint:   opentracing.TraceServer(tracer, "GET /tags")(MakeTagsEndpoint(s)),
		//		HealthEndpoint: opentracing.TraceServer(tracer, "GET /health")(MakeHealthEndpoint(s)),
	}
}

// MakeListEndpoint returns an endpoint via the given service.
func MakeListEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		span := oteltrace.SpanFromContext(ctx)
		req := request.(listRequest)
		span.SetAttributes(attribute.String("tags", fmt.Sprintf("%v", req.Order)), attribute.String("order", fmt.Sprintf("%v", req.Order)), attribute.String("Page Number", fmt.Sprintf("%v", req.PageNum)), attribute.String("tags", fmt.Sprintf("%v", req.PageSize)), attribute.String("service", "catalogue"))
		socks, err := s.List(req.Tags, req.Order, req.PageNum, req.PageSize, span.SpanContext().TraceID().String())
		return listResponse{Socks: socks, Err: err}, err
	}
}

// MakeCountEndpoint returns an endpoint via the given service.
func MakeCountEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		span := oteltrace.SpanFromContext(ctx)
		req := request.(countRequest)
		span.SetAttributes(attribute.String("counts", fmt.Sprintf("%v", req.Tags)), attribute.String("service", "catalogue"))
		n, err := s.Count(req.Tags, span.SpanContext().TraceID().String())
		return countResponse{N: n, Err: err}, err
	}
}

// MakeGetEndpoint returns an endpoint via the given service.
func MakeGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		span := oteltrace.SpanFromContext(ctx)
		req := request.(getRequest)
		span.SetAttributes(attribute.String("ID", fmt.Sprintf("%v", req.ID)), attribute.String("service", "catalogue"))
		sock, err := s.Get(req.ID, span.SpanContext().TraceID().String())
		return getResponse{Sock: sock, Err: err}, err
	}
}

// MakeTagsEndpoint returns an endpoint via the given service.
func MakeTagsEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		span := oteltrace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("service", "catalogue"))
		tags, err := s.Tags(span.SpanContext().TraceID().String())
		return tagsResponse{Tags: tags, Err: err}, err
	}
}

// MakeHealthEndpoint returns current health of the given service.
func MakeHealthEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		span := oteltrace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("service", "catalogue"))
		health := s.Health()
		return healthResponse{Health: health}, nil
	}
}

type listRequest struct {
	Tags     []string `json:"tags"`
	Order    string   `json:"order"`
	PageNum  int      `json:"pageNum"`
	PageSize int      `json:"pageSize"`
}

type listResponse struct {
	Socks   []Sock `json:"sock"`
	Err     error  `json:"err"`
	TraceID oteltrace.TraceID
}

type countRequest struct {
	Tags []string `json:"tags"`
}

type countResponse struct {
	N       int   `json:"size"` // to match original
	Err     error `json:"err"`
	TraceID oteltrace.TraceID
}

type getRequest struct {
	ID string `json:"id"`
}

type getResponse struct {
	Sock    Sock  `json:"sock"`
	Err     error `json:"err"`
	TraceID oteltrace.TraceID
}

type tagsRequest struct {
	//
}

type tagsResponse struct {
	Tags    []string `json:"tags"`
	Err     error    `json:"err"`
	TraceID oteltrace.TraceID
}

type healthRequest struct {
	//
}

type healthResponse struct {
	Health []Health `json:"health"`
}
