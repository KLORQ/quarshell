package supervisor

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const (
	supervisorOTLPEndpointEnv          = "QUARSHELL_OTLP_ENDPOINT"
	supervisorTraceParentEnv           = "TRACEPARENT"
	supervisorTraceStateEnv            = "TRACESTATE"
	supervisorTelemetryShutdownTimeout = 5 * time.Second
)

// configureSupervisorTelemetry creates an isolated OTLP provider. It does not
// install global propagators or resource detectors in the guest process.
func configureSupervisorTelemetry(ctx context.Context, endpoint, traceparent, tracestate string) (*sdktrace.TracerProvider, *sdkmetric.MeterProvider, trace.Tracer, metric.Int64Counter, context.Context, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, nil, nil, nil, context.Background(), nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, nil, nil, nil, context.Background(), fmt.Errorf("endpoint must be an absolute HTTP(S) URL with a host")
	}
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint))
	if err != nil {
		return nil, nil, nil, nil, context.Background(), errors.New("create OTLP trace exporter")
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpointURL(endpoint))
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, nil, nil, nil, context.Background(), errors.New("create OTLP metric exporter")
	}
	res := resource.NewSchemaless(attribute.String("service.name", "quarshell-supervisor"))
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	metrics := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)), sdkmetric.WithResource(res))
	exits, err := metrics.Meter("github.com/KLORQ/quarshell-runtime/internal/app/supervisor").Int64Counter("quarshell.supervisor.workload.exits")
	if err != nil {
		_ = provider.Shutdown(ctx)
		_ = metrics.Shutdown(ctx)
		return nil, nil, nil, nil, context.Background(), errors.New("create supervisor workload exit metric")
	}
	parent := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{
		"traceparent": traceparent,
		"tracestate":  tracestate,
	})
	return provider, metrics, provider.Tracer("github.com/KLORQ/quarshell-runtime/internal/app/supervisor"), exits, parent, nil
}
