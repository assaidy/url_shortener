package handlers_test

import (
	"context"
	"log/slog"
	"sync"
)

type loggedEntry struct {
	Level   slog.Level
	Message string
	Attrs   map[string]slog.Value
}

type logStore struct {
	mu      sync.Mutex
	entries []loggedEntry
}

// logRecorder is a slog.Handler that retains logs so tests can assert on them.
type logRecorder struct {
	store *logStore
	attrs []slog.Attr
}

func newLogRecorder() *logRecorder {
	return &logRecorder{store: &logStore{}}
}

func (me *logRecorder) Logger() *slog.Logger {
	return slog.New(me)
}

func (me *logRecorder) Enabled(context.Context, slog.Level) bool {
	return true
}

func (me *logRecorder) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]slog.Value, len(me.attrs)+record.NumAttrs())
	for _, attr := range me.attrs {
		if attr.Key != "" {
			attrs[attr.Key] = attr.Value
		}
	}
	record.Attrs(func(attr slog.Attr) bool {
		if attr.Key != "" {
			attrs[attr.Key] = attr.Value
		}
		return true
	})

	me.store.mu.Lock()
	me.store.entries = append(me.store.entries, loggedEntry{
		Level:   record.Level,
		Message: record.Message,
		Attrs:   attrs,
	})
	me.store.mu.Unlock()

	return nil
}

func (me *logRecorder) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &logRecorder{
		store: me.store,
		attrs: append(append([]slog.Attr(nil), me.attrs...), attrs...),
	}
}

func (me *logRecorder) WithGroup(string) slog.Handler {
	return me
}

// Find returns the first recorded entry with the given level and message.
func (me *logRecorder) Find(level slog.Level, message string) (loggedEntry, bool) {
	me.store.mu.Lock()
	defer me.store.mu.Unlock()

	for _, entry := range me.store.entries {
		if entry.Level == level && entry.Message == message {
			return entry, true
		}
	}
	return loggedEntry{}, false
}
