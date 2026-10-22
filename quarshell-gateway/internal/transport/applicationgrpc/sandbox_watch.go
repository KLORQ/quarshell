package applicationgrpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/KLORQ/quarshell-core/domain"
	gatewayv1 "github.com/KLORQ/quarshell-proto/gen/go/openshell/gateway/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const sandboxWatchBuffer = 128

// WatchSandbox streams an initial sandbox snapshot, bounded log replay and live
// status/log updates. Platform events remain unsupported.
func (s *Server) WatchSandbox(request *gatewayv1.WatchSandboxRequest, stream gatewayv1.OpenShell_WatchSandboxServer) error {
	if request == nil || strings.TrimSpace(request.GetSandbox()) == "" {
		return status.Error(codes.InvalidArgument, "sandbox is required")
	}
	if request.GetFollowEvents() || request.GetEventTail() > 0 {
		return status.Error(codes.Unimplemented, "platform event watch is not configured")
	}
	if !request.GetFollowLogs() && !request.GetFollowStatus() {
		return status.Error(codes.InvalidArgument, "follow_logs or follow_status is required")
	}
	if s.Sandboxes == nil {
		return status.Error(codes.Unavailable, "sandbox service is not configured")
	}
	if since := request.GetSinceTime(); since != nil && !since.IsValid() {
		return status.Error(codes.InvalidArgument, "since_time is invalid")
	}
	ref := domain.SandboxRef{Workspace: workspaceID(request.GetWorkspaceScope()), Sandbox: domain.SandboxID(request.GetSandbox())}
	current, err := s.Sandboxes.Get(stream.Context(), ref)
	if err != nil {
		return grpcError(err)
	}

	resumeCursor := strings.TrimSpace(request.GetResumeAfterCursor())
	resumeSequence := uint64(0)
	if resumeCursor != "" {
		if s.SandboxLogs == nil {
			return status.Error(codes.OutOfRange, "watch cursor is not available in this Gateway instance")
		}
		var sameEpoch, valid bool
		resumeSequence, sameEpoch, valid = s.parseSandboxLogCursor(request.GetSandbox(), resumeCursor)
		if !sameEpoch {
			return status.Error(codes.OutOfRange, "watch cursor belongs to a previous Gateway cursor space")
		}
		if !valid {
			return status.Error(codes.InvalidArgument, "resume_after_cursor was not issued for this sandbox and Gateway instance")
		}
	}
	var live chan *gatewayv1.SandboxStreamEvent
	var replay []*gatewayv1.SandboxStreamEvent
	if request.GetFollowLogs() {
		live = make(chan *gatewayv1.SandboxStreamEvent, sandboxWatchBuffer)
		s.logMu.Lock()
		if s.logWatchers == nil {
			s.logWatchers = make(map[string]map[chan *gatewayv1.SandboxStreamEvent]struct{})
		}
		if s.logWatchers[request.GetSandbox()] == nil {
			s.logWatchers[request.GetSandbox()] = make(map[chan *gatewayv1.SandboxStreamEvent]struct{})
		}
		s.logWatchers[request.GetSandbox()][live] = struct{}{}
		stored, readErr := s.sandboxLogSnapshotLocked(stream.Context(), request.GetSandbox())
		s.logMu.Unlock()
		if readErr != nil {
			s.removeSandboxLogWatcher(request.GetSandbox(), live)
			return status.Error(codes.Internal, "read sandbox log replay")
		}
		defer s.removeSandboxLogWatcher(request.GetSandbox(), live)
		if resumeCursor != "" {
			if len(stored) == 0 {
				return status.Error(codes.OutOfRange, "watch cursor is no longer retained")
			}
			first := cursorSequence(stored[0].GetCursor())
			last := cursorSequence(stored[len(stored)-1].GetCursor())
			if resumeSequence > last {
				return status.Error(codes.InvalidArgument, "resume_after_cursor is ahead of the latest sandbox log")
			}
			if first > 1 && resumeSequence < first-1 {
				return status.Error(codes.OutOfRange, "watch cursor has been trimmed from the sandbox log window")
			}
		} else {
			tail := int(request.GetLogTailLines())
			if tail == 0 {
				tail = 200
			}
			if tail > sandboxLogRetention {
				tail = sandboxLogRetention
			}
			if tail > 0 && len(stored) > tail {
				stored = stored[len(stored)-tail:]
			}
		}
		for _, event := range stored {
			line := event.GetLog()
			if line == nil || (resumeCursor != "" && cursorSequence(event.GetCursor()) <= resumeSequence) {
				continue
			}
			if watchLogMatches(line, request) {
				replay = append(replay, event)
			}
		}
	}

	lastSnapshot := sandboxProto(current, nil)
	if err := stream.Send(&gatewayv1.SandboxStreamEvent{Payload: &gatewayv1.SandboxStreamEvent_Sandbox{Sandbox: lastSnapshot}}); err != nil {
		return err
	}
	for _, event := range replay {
		if err := stream.Send(event); err != nil {
			return err
		}
	}
	lastLogSequence := resumeSequence
	for _, event := range replay {
		if sequence := cursorSequence(event.GetCursor()); sequence > lastLogSequence {
			lastLogSequence = sequence
		}
	}
	if request.GetStopOnTerminal() && terminalWatchPhase(phaseFromState(current.State)) {
		return nil
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event := <-live:
			if line := event.GetLog(); line != nil {
				sequence := cursorSequence(event.GetCursor())
				if sequence <= lastLogSequence || !watchLogMatches(line, request) {
					continue
				}
				lastLogSequence = sequence
			}
			if err := stream.Send(proto.Clone(event).(*gatewayv1.SandboxStreamEvent)); err != nil {
				return err
			}
		case <-ticker.C:
			if !request.GetFollowStatus() && !request.GetStopOnTerminal() {
				continue
			}
			latest, err := s.Sandboxes.Get(stream.Context(), ref)
			if err != nil {
				return grpcError(err)
			}
			phase := phaseFromState(latest.State)
			if request.GetFollowStatus() {
				snapshot := sandboxProto(latest, nil)
				if !proto.Equal(lastSnapshot, snapshot) {
					if err := stream.Send(&gatewayv1.SandboxStreamEvent{Payload: &gatewayv1.SandboxStreamEvent_Sandbox{Sandbox: snapshot}}); err != nil {
						return err
					}
					lastSnapshot = snapshot
				}
			}
			if request.GetStopOnTerminal() && terminalWatchPhase(phase) {
				return nil
			}
		}
	}
}

func (s *Server) sandboxLogSnapshotLocked(ctx context.Context, sandboxID string) ([]*gatewayv1.SandboxStreamEvent, error) {
	if s.SandboxLogs != nil {
		lines, err := s.SandboxLogs.List(ctx, sandboxID)
		if err != nil {
			return nil, err
		}
		out := make([]*gatewayv1.SandboxStreamEvent, 0, len(lines))
		for _, line := range lines {
			item := &gatewayv1.SandboxLogLine{SandboxId: line.SandboxID, Level: line.Level, Target: line.Target, Message: line.Message, Source: line.Source}
			if !line.EventTime.IsZero() {
				item.EventTime = timestamppb.New(line.EventTime)
			}
			out = append(out, &gatewayv1.SandboxStreamEvent{Payload: &gatewayv1.SandboxStreamEvent_Log{Log: item}, Cursor: s.sandboxLogCursor(sandboxID, line.Sequence)})
		}
		return out, nil
	}
	lines := s.sandboxLogs[sandboxID]
	out := make([]*gatewayv1.SandboxStreamEvent, 0, len(lines))
	for _, line := range lines {
		out = append(out, logStreamEvent(proto.Clone(line).(*gatewayv1.SandboxLogLine), ""))
	}
	return out, nil
}

func (s *Server) removeSandboxLogWatcher(sandboxID string, watcher chan *gatewayv1.SandboxStreamEvent) {
	s.logMu.Lock()
	delete(s.logWatchers[sandboxID], watcher)
	if len(s.logWatchers[sandboxID]) == 0 {
		delete(s.logWatchers, sandboxID)
	}
	s.logMu.Unlock()
}

func (s *Server) publishSandboxLogLocked(sandboxID string, line *gatewayv1.SandboxLogLine, cursor string) {
	if len(s.logWatchers[sandboxID]) == 0 {
		return
	}
	event := logStreamEvent(line, cursor)
	for watcher := range s.logWatchers[sandboxID] {
		select {
		case watcher <- event:
		default:
			select {
			case <-watcher:
			default:
			}
			warning := &gatewayv1.SandboxStreamEvent{Payload: &gatewayv1.SandboxStreamEvent_Warning{Warning: &gatewayv1.SandboxStreamWarning{Message: "sandbox log watch dropped events because its buffer was full"}}}
			select {
			case watcher <- warning:
			default:
			}
		}
	}
}

func logStreamEvent(line *gatewayv1.SandboxLogLine, cursor string) *gatewayv1.SandboxStreamEvent {
	return &gatewayv1.SandboxStreamEvent{Payload: &gatewayv1.SandboxStreamEvent_Log{Log: proto.Clone(line).(*gatewayv1.SandboxLogLine)}, Cursor: cursor}
}

func (s *Server) sandboxLogCursor(sandboxID string, sequence uint64) string {
	if s.logCursorEpoch == "" {
		s.logCursorEpoch = randomSessionID()
	}
	digest := sha256.Sum256([]byte(sandboxID))
	return fmt.Sprintf("%s-%s-%020d", s.logCursorEpoch, hex.EncodeToString(digest[:6]), sequence)
}

func (s *Server) parseSandboxLogCursor(sandboxID, cursor string) (uint64, bool, bool) {
	if s.logCursorEpoch == "" {
		s.logCursorEpoch = randomSessionID()
	}
	epochPrefix := s.logCursorEpoch + "-"
	if !strings.HasPrefix(cursor, epochPrefix) {
		return 0, false, false
	}
	digest := sha256.Sum256([]byte(sandboxID))
	prefix := epochPrefix + hex.EncodeToString(digest[:6]) + "-"
	if !strings.HasPrefix(cursor, prefix) {
		return 0, true, false
	}
	sequence, err := strconv.ParseUint(strings.TrimPrefix(cursor, prefix), 10, 64)
	return sequence, true, err == nil && sequence > 0
}

func cursorSequence(cursor string) uint64 {
	parts := strings.Split(cursor, "-")
	if len(parts) == 0 {
		return 0
	}
	sequence, _ := strconv.ParseUint(parts[len(parts)-1], 10, 64)
	return sequence
}

func watchLogMatches(line *gatewayv1.SandboxLogLine, request *gatewayv1.WatchSandboxRequest) bool {
	if sources := request.GetLogSources(); len(sources) > 0 {
		source := line.GetSource()
		if source == "" {
			source = "gateway"
		}
		matched := false
		for _, allowed := range sources {
			if allowed == source {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if since := request.GetSinceTime(); since != nil {
		if line.GetEventTime() == nil || line.GetEventTime().AsTime().Before(since.AsTime()) {
			return false
		}
	}
	minimum := logLevelRank(request.GetLogMinLevel())
	return minimum == 0 || logLevelRank(line.GetLevel()) >= minimum
}

func terminalWatchPhase(phase gatewayv1.SandboxPhase) bool {
	switch phase {
	case gatewayv1.SandboxPhase_SANDBOX_PHASE_READY,
		gatewayv1.SandboxPhase_SANDBOX_PHASE_COMPLETED,
		gatewayv1.SandboxPhase_SANDBOX_PHASE_STOPPED,
		gatewayv1.SandboxPhase_SANDBOX_PHASE_ERROR:
		return true
	default:
		return false
	}
}
