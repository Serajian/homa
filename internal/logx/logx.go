// Package logx gives homa a single logging switch. Every package asks for a
// logger with For, and main decides at runtime where the output goes.
//
// Output is discarded until main says otherwise, because homa draws a
// terminal UI that stray log lines would ruin.
package logx

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
)

// current is the handler everything ultimately writes to. It is swapped
// atomically, so Set takes effect even for loggers handed out earlier.
var current atomic.Pointer[slog.Handler]

func init() { Silence() }

// Set sends all homa logging to h.
func Set(h slog.Handler) {
	if h == nil {
		Silence()
		return
	}
	current.Store(&h)
}

// Silence discards all homa logging. This is the startup default.
func Silence() {
	h := slog.DiscardHandler
	current.Store(&h)
}

// ToWriter is the common case: readable lines at or above level, written to w.
// Pass os.Stderr to watch homa work, or an open file to keep a trace.
func ToWriter(w io.Writer, level slog.Level) {
	Set(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}

// For returns a logger tagged with the calling package's name, so a line's
// origin is visible in the output.
func For(pkg string) *slog.Logger {
	return slog.New(&proxy{}).With("pkg", pkg)
}

// proxy forwards to whatever handler is current at the moment of each call.
//
// slog handlers are meant to be immutable: WithAttrs and WithGroup return new
// handlers with the attributes already baked in. That would freeze the
// destination too, so instead the proxy records those calls and replays them
// against the current handler on every write.
type proxy struct{ ops []op }

// op is one recorded WithAttrs or WithGroup call. Exactly one field is set.
type op struct {
	group string
	attrs []slog.Attr
}

func (p *proxy) resolve() slog.Handler {
	h := *current.Load()
	for _, o := range p.ops {
		if o.group != "" {
			h = h.WithGroup(o.group)
		} else {
			h = h.WithAttrs(o.attrs)
		}
	}
	return h
}

func (p *proxy) with(o op) *proxy {
	// Copy rather than append in place: slog may share the parent handler
	// across several loggers, and they must not scribble on each other.
	ops := make([]op, len(p.ops), len(p.ops)+1)
	copy(ops, p.ops)
	return &proxy{ops: append(ops, o)}
}

func (p *proxy) Enabled(ctx context.Context, l slog.Level) bool {
	return p.resolve().Enabled(ctx, l)
}

//nolint:gocritic // the signature is fixed by slog.Handler; Record is passed by value by design
func (p *proxy) Handle(ctx context.Context, r slog.Record) error {
	return p.resolve().Handle(ctx, r)
}

func (p *proxy) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return p
	}
	return p.with(op{attrs: attrs})
}

func (p *proxy) WithGroup(name string) slog.Handler {
	if name == "" {
		return p
	}
	return p.with(op{group: name})
}
