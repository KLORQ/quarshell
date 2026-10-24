package supervisor

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type supervisorTraceCollector struct {
	mu      sync.Mutex
	request *collectorpb.ExportTraceServiceRequest
	metrics *metricspb.ExportMetricsServiceRequest
}

func TestSupervisorTelemetryExportsSpanWithW3CParent(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	collector := &supervisorTraceCollector{}
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "opentelemetry.proto.collector.trace.v1.TraceService",
		HandlerType: (*interface{})(nil),
		Methods: []grpc.MethodDesc{{MethodName: "Export", Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			request := new(collectorpb.ExportTraceServiceRequest)
			if err := decode(request); err != nil {
				return nil, err
			}
			collector.mu.Lock()
			collector.request = request
			collector.mu.Unlock()
			return &collectorpb.ExportTraceServiceResponse{}, nil
		}}},
	}, collector)
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "opentelemetry.proto.collector.metrics.v1.MetricsService",
		HandlerType: (*interface{})(nil),
		Methods: []grpc.MethodDesc{{MethodName: "Export", Handler: func(_ any, ctx context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
			request := new(metricspb.ExportMetricsServiceRequest)
			if err := decode(request); err != nil {
				return nil, err
			}
			collector.mu.Lock()
			collector.metrics = request
			collector.mu.Unlock()
			return &metricspb.ExportMetricsServiceResponse{}, nil
		}}},
	}, collector)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()

	provider, metrics, tracer, exits, parent, err := configureSupervisorTelemetry(t.Context(), "http://"+listener.Addr().String(), "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "")
	if err != nil {
		t.Fatal(err)
	}
	_, span := tracer.Start(parent, "supervisor.run")
	span.End()
	exits.Add(t.Context(), 1, metric.WithAttributes(attribute.Int("process.exit.code", 0)))
	shutdownCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := provider.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	if err := metrics.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}

	collector.mu.Lock()
	request := collector.request
	metricRequest := collector.metrics
	collector.mu.Unlock()
	if request == nil || len(request.ResourceSpans) != 1 {
		t.Fatalf("collector received no supervisor spans: %#v", request)
	}
	resourceSpans := request.ResourceSpans[0]
	serviceFound := false
	for _, attr := range resourceSpans.Resource.Attributes {
		serviceFound = serviceFound || attr.Key == "service.name" && attr.Value.GetStringValue() == "quarshell-supervisor"
	}
	wantParent, _ := hex.DecodeString("00f067aa0ba902b7")
	parentFound := false
	for _, scope := range resourceSpans.ScopeSpans {
		for _, exported := range scope.Spans {
			parentFound = parentFound || bytes.Equal(exported.ParentSpanId, wantParent)
		}
	}
	if !serviceFound || !parentFound {
		t.Fatalf("service name=%v parent span=%v", serviceFound, parentFound)
	}
	if metricRequest == nil || len(metricRequest.ResourceMetrics) != 1 {
		t.Fatalf("collector received no supervisor metrics: %#v", metricRequest)
	}
	metricFound := false
	for _, scope := range metricRequest.ResourceMetrics[0].ScopeMetrics {
		for _, exported := range scope.Metrics {
			if exported.Name == "quarshell.supervisor.workload.exits" && len(exported.GetSum().DataPoints) == 1 {
				point := exported.GetSum().DataPoints[0]
				metricFound = point.GetAsInt() == 1 && len(point.Attributes) == 1 && point.Attributes[0].Key == "process.exit.code" && point.Attributes[0].Value.GetIntValue() == 0
			}
		}
	}
	if !metricFound {
		t.Fatalf("workload exit counter was not exported: %#v", metricRequest.ResourceMetrics[0].ScopeMetrics)
	}
}

func TestSupervisorTelemetryRejectsMalformedEndpoint(t *testing.T) {
	if _, _, _, _, _, err := configureSupervisorTelemetry(t.Context(), "://invalid", "", ""); err == nil {
		t.Fatal("malformed OTLP endpoint was accepted")
	}
}
