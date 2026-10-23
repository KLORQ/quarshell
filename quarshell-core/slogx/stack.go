package slogx

import (
	"log/slog"
	"runtime"
	"strings"
)

const stackSkipFrames = 4

func stackTrace(depth int) string {
	if depth <= 0 {
		depth = 32
	}
	pcs := make([]uintptr, depth)
	n := runtime.Callers(stackSkipFrames, pcs)
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for {
		frame, more := frames.Next()
		if frame.Function == "" {
			if !more {
				break
			}
			continue
		}
		if strings.HasPrefix(frame.Function, "runtime.") ||
			strings.Contains(frame.Function, "log/slog") ||
			strings.Contains(frame.Function, "slogx.") {
			if !more {
				break
			}
			continue
		}
		b.WriteString(frame.Function)
		b.WriteByte('\n')
		b.WriteString("\t")
		b.WriteString(frame.File)
		b.WriteByte(':')
		b.WriteString(itoa(frame.Line))
		b.WriteByte('\n')
		if !more {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func appendStackAttr(r *slog.Record, cfg *Config) {
	if !cfg.StackOnError {
		return
	}
	if r.Level < slog.LevelError {
		return
	}
	stack := stackTrace(cfg.StackDepth)
	if stack == "" {
		return
	}
	r.AddAttrs(slog.String("stack", stack))
}
