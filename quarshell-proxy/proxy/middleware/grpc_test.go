package middleware

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	middlewarev1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/middleware/v1"
	"google.golang.org/grpc"
)

type grpcMiddlewareFixture struct {
	middlewarev1.UnimplementedSupervisorMiddlewareServer
}

type grpcResponseFixture struct {
	middlewarev1.UnimplementedHttpResponsePreReturnServer
}

type blockingMiddlewareFixture struct {
	middlewarev1.UnimplementedSupervisorMiddlewareServer
	started chan struct{}
}

func (f blockingMiddlewareFixture) EvaluateHttpRequest(ctx context.Context, _ *middlewarev1.HttpRequestEvaluation) (*middlewarev1.HttpRequestResult, error) {
	close(f.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (grpcResponseFixture) Evaluate(stream middlewarev1.HttpResponsePreReturn_EvaluateServer) error {
	event, err := stream.Recv()
	if err != nil || event.GetPreflight() == nil {
		return err
	}
	preflight := &middlewarev1.HttpResponseEventResult{
		Result: &middlewarev1.HttpResponseEventResult_PreflightResult{
			PreflightResult: &middlewarev1.HttpResponsePreflightResult{
				Action: &middlewarev1.HttpResponsePreflightResult_Inspect{
					Inspect: &middlewarev1.HttpResponsePreflightInspect{
						BodyMode: middlewarev1.HttpResponseBodyMode_HTTP_RESPONSE_BODY_MODE_WHOLE_BODY_BYTES,
						HeaderMutations: []*middlewarev1.HeaderMutation{{
							Operation: &middlewarev1.HeaderMutation_Write{Write: &middlewarev1.WriteHeader{Name: "x-response-middleware", Value: "ok"}},
						}},
					},
				},
			},
		},
	}
	if err := stream.Send(preflight); err != nil {
		return err
	}
	event, err = stream.Recv()
	if err != nil || event.GetBody() == nil {
		return err
	}
	bodyResult := &middlewarev1.HttpResponseEventResult{
		Result: &middlewarev1.HttpResponseEventResult_BodyResult{
			BodyResult: &middlewarev1.HttpResponseBodyResult{
				Sequence: event.GetBody().GetSequence(),
				Action: &middlewarev1.HttpResponseBodyResult_Transform{
					Transform: &middlewarev1.HttpResponseBodyTransform{Replacement: &middlewarev1.HttpResponseBodyTransform_Data{Data: []byte("changed")}},
				},
			},
		},
	}
	if err := stream.Send(bodyResult); err != nil {
		return err
	}
	event, err = stream.Recv()
	if err != nil || event.GetTrailers() == nil {
		return err
	}
	if err := stream.Send(&middlewarev1.HttpResponseEventResult{Result: &middlewarev1.HttpResponseEventResult_TrailersResult{TrailersResult: &middlewarev1.HttpResponseTrailersResult{}}}); err != nil {
		return err
	}
	for {
		if _, err := stream.Recv(); err != nil {
			return nil
		}
	}
}

func (grpcMiddlewareFixture) EvaluateHttpRequest(_ context.Context, req *middlewarev1.HttpRequestEvaluation) (*middlewarev1.HttpRequestResult, error) {
	if string(req.GetBody()) != "payload" {
		return &middlewarev1.HttpRequestResult{Decision: middlewarev1.Decision_DECISION_DENY, Reason: "body missing"}, nil
	}
	result := &middlewarev1.HttpRequestResult{
		Decision:        middlewarev1.Decision_DECISION_ALLOW,
		HeaderMutations: []*middlewarev1.HeaderMutation{{Operation: &middlewarev1.HeaderMutation_Write{Write: &middlewarev1.WriteHeader{Name: "x-middleware", Value: "ok"}}}},
		Findings:        []*middlewarev1.Finding{{Type: "regex.openai", Count: 2, Confidence: "medium", Severity: "medium"}},
		Metadata:        map[string]string{"regex_matches_replaced": "2", "unsafe": "secret"},
	}
	if req.GetTarget().GetPath() == "/mutate" {
		result.HasBody = true
		result.Body = []byte("rewritten")
		result.HeaderMutations = append(result.HeaderMutations, &middlewarev1.HeaderMutation{Operation: &middlewarev1.HeaderMutation_Remove{Remove: &middlewarev1.RemoveHeader{Name: "x-old"}}})
	}
	return result, nil
}

func (grpcMiddlewareFixture) EvaluateWebSocketSession(stream middlewarev1.SupervisorMiddleware_EvaluateWebSocketSessionServer) error {
	event, err := stream.Recv()
	if err != nil || event.GetPreflight() == nil {
		return err
	}
	if err := stream.Send(&middlewarev1.WebSocketSessionEventResult{Result: &middlewarev1.WebSocketSessionEventResult_PreflightDecision{PreflightDecision: &middlewarev1.WebSocketPreflightDecision{Action: middlewarev1.WebSocketPreflightAction_WEB_SOCKET_PREFLIGHT_ACTION_INSPECT}}}); err != nil {
		return err
	}
	event, err = stream.Recv()
	if err != nil || event.GetSessionStart() == nil {
		return err
	}
	event, err = stream.Recv()
	if err != nil || event.GetMessage() == nil {
		return err
	}
	message := event.GetMessage()
	result := &middlewarev1.WebSocketSessionEventResult{Result: &middlewarev1.WebSocketSessionEventResult_MessageResult{MessageResult: &middlewarev1.WebSocketMessageResult{Sequence: message.GetSequence(), Decision: middlewarev1.Decision_DECISION_ALLOW, Findings: []*middlewarev1.Finding{{Type: "regex.openai", Count: 1}}, Metadata: map[string]string{"regex_matches_replaced": "1"}}}}
	if text := message.GetText(); text != "" {
		result.GetMessageResult().Replacement = &middlewarev1.WebSocketMessageResult_Text{Text: text + "!"}
	}
	if err := stream.Send(result); err != nil {
		return err
	}
	for {
		if _, err := stream.Recv(); err != nil {
			return nil
		}
	}
}

func TestGRPCStageEvaluatesTypedSupervisorMiddleware(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	middlewarev1.RegisterSupervisorMiddlewareServer(server, grpcMiddlewareFixture{})
	middlewarev1.RegisterHttpResponsePreReturnServer(server, grpcResponseFixture{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })

	stage, err := NewGRPCStage(GRPCStageConfig{Name: "guard", Endpoint: "http://" + listener.Addr().String(), Timeout: time.Second, FailClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := stage.Evaluate(context.Background(), Request{Host: "api.example", Port: 443, Method: "GET", Path: "/v1", Headers: map[string]string{"Accept": "application/json"}, Body: []byte("payload")})
	if err != nil || !decision.Allow || decision.MutateHeaders["x-middleware"] != "ok" || len(decision.Findings) != 1 || decision.Findings[0].Type != "regex.openai" || decision.Metadata["regex_matches_replaced"] != "2" {
		t.Fatalf("typed middleware decision=%+v err=%v", decision, err)
	}
}

func TestGRPCStageEvaluatesTypedResponseMiddleware(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	middlewarev1.RegisterSupervisorMiddlewareServer(server, grpcMiddlewareFixture{})
	middlewarev1.RegisterHttpResponsePreReturnServer(server, grpcResponseFixture{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	stage, err := NewGRPCStage(GRPCStageConfig{Name: "guard", Endpoint: "http://" + listener.Addr().String(), Timeout: time.Second, FailClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	resp := &Response{StatusCode: http.StatusOK, Headers: make(http.Header), Trailers: make(http.Header), Body: []byte("original")}
	if decision, err := stage.EvaluateResponse(context.Background(), Request{Host: "api.example", Port: 443, Method: "GET", Path: "/v1"}, resp); err != nil || !decision.Allow {
		t.Fatalf("response decision=%+v err=%v", decision, err)
	}
	if string(resp.Body) != "changed" || resp.Headers.Get("X-Response-Middleware") != "ok" {
		t.Fatalf("response=%q headers=%v", resp.Body, resp.Headers)
	}
}

func TestResponseBodylessOnlySkipsSemanticallyBodylessResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		status int
		want   bool
	}{
		{name: "head", method: http.MethodHead, status: http.StatusOK, want: true},
		{name: "no content", method: http.MethodGet, status: http.StatusNoContent, want: true},
		{name: "empty ok body remains inspectable", method: http.MethodGet, status: http.StatusOK, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := responseBodyless(Request{Method: tc.method}, &Response{StatusCode: tc.status}); got != tc.want {
				t.Fatalf("responseBodyless()=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyTrailerMutationsRejectsCreationAndProtectedNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  *middlewarev1.HeaderMutation
	}{
		{name: "creation", mut: &middlewarev1.HeaderMutation{Operation: &middlewarev1.HeaderMutation_Write{Write: &middlewarev1.WriteHeader{Name: "x-new", Value: "value"}}}},
		{name: "protected", mut: &middlewarev1.HeaderMutation{Operation: &middlewarev1.HeaderMutation_Remove{Remove: &middlewarev1.RemoveHeader{Name: "Content-Length"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			trailers := http.Header{"X-Existing": []string{"old"}}
			if err := applyTrailerMutations(trailers, []*middlewarev1.HeaderMutation{tc.mut}); err == nil {
				t.Fatal("expected invalid trailer mutation to fail")
			}
			if got := trailers.Get("X-Existing"); got != "old" {
				t.Fatalf("trailers changed after rejected mutation: %q", got)
			}
		})
	}
}

func TestGRPCStageFailsClosedWhenUnavailable(t *testing.T) {
	stage, err := NewGRPCStage(GRPCStageConfig{Name: "guard", Endpoint: "http://127.0.0.1:1", Timeout: 10 * time.Millisecond, FailClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := stage.Evaluate(context.Background(), Request{Host: "api.example", Port: 443, Method: "GET", Path: "/"})
	if err != nil || decision.Allow {
		t.Fatalf("unavailable middleware decision=%+v err=%v", decision, err)
	}
}

func TestGRPCStageCancellationFailsClosedPromptly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	fixture := blockingMiddlewareFixture{started: make(chan struct{})}
	middlewarev1.RegisterSupervisorMiddlewareServer(server, fixture)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	stage, err := NewGRPCStage(GRPCStageConfig{Name: "guard", Endpoint: "http://" + listener.Addr().String(), Timeout: time.Second, FailClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan Decision, 1)
	go func() {
		decision, _ := stage.Evaluate(ctx, Request{Host: "api.example", Method: "GET", Path: "/"})
		result <- decision
	}()
	select {
	case <-fixture.started:
	case <-time.After(time.Second):
		t.Fatal("middleware request did not start")
	}
	cancel()
	select {
	case decision := <-result:
		if decision.Allow || decision.Reason != "supervisor middleware unavailable" {
			t.Fatalf("cancelled middleware decision=%+v; expected fail-closed denial", decision)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled middleware request did not return promptly")
	}
}

func TestPipelineCarriesRequestBodyAndOrderedHeaderMutations(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	middlewarev1.RegisterSupervisorMiddlewareServer(server, grpcMiddlewareFixture{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	stage, err := NewGRPCStage(GRPCStageConfig{Name: "guard", Endpoint: "http://" + listener.Addr().String(), Timeout: time.Second, FailClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := (&Pipeline{Stages: []Stage{stage}}).Run(context.Background(), Request{Host: "api.example", Port: 443, Method: "POST", Path: "/mutate", Headers: map[string]string{"x-old": "secret"}, Body: []byte("payload")})
	if err != nil || !decision.Allow || !decision.HasBody || string(decision.Body) != "rewritten" {
		t.Fatalf("decision=%+v err=%v", decision, err)
	}
	removed := false
	for _, mutation := range decision.HeaderMutations {
		if mutation.Remove && mutation.Name == "x-old" {
			removed = true
		}
	}
	if !removed {
		t.Fatalf("missing remove mutation: %+v", decision.HeaderMutations)
	}
}

func TestGRPCStageEvaluatesWebSocketSession(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	middlewarev1.RegisterSupervisorMiddlewareServer(server, grpcMiddlewareFixture{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	stage, err := NewGRPCStage(GRPCStageConfig{Name: "guard", Endpoint: "http://" + listener.Addr().String(), Timeout: time.Second, FailClosed: true, Include: []string{"*.example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	pipeline := &Pipeline{Stages: []Stage{stage}}
	sessions, err := pipeline.OpenWebSocketSessions(context.Background(), WebSocketRequest{SessionID: "s1", Host: "api.example.com", Port: 443, Path: "/ws"})
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%d err=%v", len(sessions), err)
	}
	if err := sessions[0].Start(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	allow, err := sessions[0].Message(context.Background(), 1, WebSocketClientToUpstream, []byte("hello"), true)
	if err != nil || !allow.Allow || string(allow.Replacement) != "hello!" || len(allow.Findings) != 1 || allow.Findings[0].Type != "regex.openai" || allow.Metadata["regex_matches_replaced"] != "1" {
		t.Fatalf("decision=%+v err=%v", allow, err)
	}
	allow, err = sessions[0].Message(context.Background(), 2, WebSocketUpstreamToClient, []byte("response"), true)
	if err != nil || !allow.Allow || string(allow.Replacement) != "response" {
		t.Fatalf("upstream response changed: decision=%+v err=%v", allow, err)
	}
	if err := sessions[0].Close("done"); err != nil {
		t.Fatal(err)
	}
}
