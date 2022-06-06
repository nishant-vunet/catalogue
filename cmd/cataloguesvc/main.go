package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"golang.org/x/net/context"

	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/microservices-demo/catalogue"

	//	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/middleware"
)

const (
	ServiceName = "catalogue"
)

/*var (
	HTTPLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Time (in seconds) spent serving HTTP requests.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "status_code", "isWS"})
)*/

/*func init() {
	prometheus.MustRegister(HTTPLatency)
}*/

func main() {
	var (
		port   = flag.String("port", "80", "Port to bind HTTP listener") // TODO(pb): should be -addr, default ":80"
		images = flag.String("images", "./images/", "Image path")
		dsn    = flag.String("DSN", "catalogue_user:default_password@tcp(catalogue-db:3306)/socksdb", "Data Source Name: [username[:password]@][protocol[(address)]]/dbname")
		//dsn     = flag.String("DSN", "catalogue_user:default_password@tcp(0.0.0.0:3306)/socksdb", "Data Source Name: [username[:password]@][protocol[(address)]]/dbname")
		otelurl = flag.String("otel", os.Getenv("OTEL"), "OTLP address")
	)
	flag.Parse()

	fmt.Fprintf(os.Stderr, "images: %q\n", *images)
	abs, err := filepath.Abs(*images)
	fmt.Fprintf(os.Stderr, "Abs(images): %q (%v)\n", abs, err)
	pwd, err := os.Getwd()
	fmt.Fprintf(os.Stderr, "Getwd: %q (%v)\n", pwd, err)
	files, _ := filepath.Glob(*images + "/*")
	fmt.Fprintf(os.Stderr, "ls: %q\n", files) // contains a list of all files in the current directory

	// Log domain.
	var logger log.Logger
	ctx := context.Background()
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		//	logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		//	logger = log.NewContext(logger).With("caller", log.DefaultCaller)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller")
	}

	//var bsp sdktrace.SpanProcessor

	if *otelurl == "" {
		traceExporter, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			level.Error(logger).Log("failed to initialize stdouttrace export pipeline: %v", err)
			os.Exit(1)
		}
		bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(bsp))
		otel.SetTracerProvider(tp)
	} else {
                //logger := log.NewContext(logger).With("tracer", "Otel")
               //      otelurlValue := *otelurl
                level.Info(logger).Log("addr", otelurl)

                ctx := context.Background()
/*              traceExporter, err := otlptracegrpc.New(ctx,
                        otlptracegrpc.WithEndpoint(otelurlValue),
                        otlptracegrpc.WithInsecure(),
                        otlptracegrpc.WithDialOption(grpc.WithBlock()),
                )*/
                traceExporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			level.Error(logger).Log("Failed to create the collector exporter: %v", err)
			os.Exit(1)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			if err := traceExporter.Shutdown(ctx); err != nil {
				otel.Handle(err)
			}
		}()

		res, err := resource.New(ctx,
			resource.WithAttributes(
				// the service name used to display traces in backends
				semconv.ServiceNameKey.String("catalogue"),
			),
		)

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(
				traceExporter,
				// add following two options to ensure flush
				sdktrace.WithBatchTimeout(5*time.Second),
				sdktrace.WithMaxExportBatchSize(10),
			),
		)
		defer func() {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			if err := tp.Shutdown(ctx); err != nil {
				otel.Handle(err)
			}
		}()
		otel.SetTracerProvider(tp)
		propagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
		otel.SetTextMapPropagator(propagator)
	}

	// Mechanical stuff.
	errc := make(chan error)

	// Data domain.
	db, err := sqlx.Open("mysql", *dsn)
	if err != nil {
		logger.Log("err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Check if DB connection can be made, only for logging purposes, should not fail/exit
	err = db.Ping()
	if err != nil {
		logger.Log("Error", "Unable to connect to Database", "DSN", dsn)
	}

	// Service domain.
	var service catalogue.Service
	{
		service = catalogue.NewCatalogueService(db, logger)
		service = catalogue.LoggingMiddleware(logger)(service)
	}

	// Endpoint domain.
	endpoints := catalogue.MakeEndpoints(service)

	// HTTP router
	router := catalogue.MakeHTTPHandler(ctx, endpoints, *images, logger)
	/*httpMiddleware := []middleware.Interface{
		middleware.Instrument{
			//Duration:     HTTPLatency,
			RouteMatcher: router,
		},
	}

	// Handler
	handler := middleware.Merge(httpMiddleware...).Wrap(router)*/
	handler := middleware.Merge().Wrap(router)
	//fmt.Println(handler)
	// Create and launch the HTTP server.
	go func() {
		logger.Log("transport", "HTTP", "port", *port)
		errc <- http.ListenAndServe(":"+*port, otelhttp.NewHandler(handler, "catalogue",
			otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
		))
	}()

	// Capture interrupts.
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	logger.Log("exit", <-errc)
}
