package redirection

import (
	"context"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"
)

// newBufferingService builds the minimum Service needed to exercise the
// buffering job without starting the postgres- and redis-backed workers.
func newBufferingService() *Service {
	return &Service{
		redirectionRecords: make(chan redirectionRecord, redirectionIngressSize),
		redirectionBatches: make(chan *[]redirectionRecord, redirectionBatchQueueSize*4),
		redirectionPool: sync.Pool{New: func() any {
			return new(make([]redirectionRecord, 0, redirectionBatchSize))
		}},
		currentRedirectionBatch: new(make([]redirectionRecord, 0, redirectionBatchSize)),
	}
}

// startBufferJob runs the buffer job until the test ends. Closing the record
// channel lets the job drain and stop without leaking its goroutine.
func startBufferJob(t *testing.T, service *Service) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = service.redirectionRecordsBufferJob(ctx)
	}()

	t.Cleanup(func() {
		cancel()
		close(service.redirectionRecords)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("redirection buffer job did not stop")
		}
	})
}

func recordRedirection(t *testing.T, service *Service) {
	t.Helper()

	require.NoError(t, service.Record(
		context.Background(),
		uuid.NewV7(),
		uuid.NewV7(),
		"192.0.2.1",
		"Chrome",
		"Windows 10",
	))
}

// receiveRedirectionBatch waits for the buffer job to hand a batch to the
// flusher and reports when the batch was received.
func receiveRedirectionBatch(t *testing.T, service *Service, timeout time.Duration) (*[]redirectionRecord, time.Time) {
	t.Helper()

	select {
	case batch := <-service.redirectionBatches:
		return batch, time.Now()
	case <-time.After(timeout):
		require.FailNow(t, "timed out waiting for a redirection batch")
		return nil, time.Time{}
	}
}

// TestRedirectionBufferSpacesFlushes verifies that a flush which just happened
// is followed by a full flush interval, even when it landed shortly before a
// tick. Without that spacing a size-triggered flush can be followed by a tiny
// tick-triggered flush almost immediately.
func TestRedirectionBufferSpacesFlushes(t *testing.T) {
	service := newBufferingService()
	startBufferJob(t, service)

	// The first record is flushed by a ticker tick. Receiving it tells us when
	// the next tick will happen.
	recordRedirection(t, service)
	_, _ = receiveRedirectionBatch(t, service, 3*time.Second)

	// Fill a whole batch halfway to the next tick, then leave one record
	// buffered. The tick must not flush that leftover record right away.
	time.Sleep(redirectionFlushInterval / 2)

	for range redirectionBatchSize {
		recordRedirection(t, service)
	}
	fullBatch, fullBatchAt := receiveRedirectionBatch(t, service, 2*time.Second)
	require.Len(t, *fullBatch, redirectionBatchSize)

	recordRedirection(t, service)
	nextBatch, nextBatchAt := receiveRedirectionBatch(t, service, 3*time.Second)
	require.Len(t, *nextBatch, 1)

	gap := nextBatchAt.Sub(fullBatchAt)
	minGap := redirectionFlushInterval * 7 / 10
	require.GreaterOrEqual(t, gap, minGap,
		"flushed the buffered batch %s after the previous flush; want at least %s", gap, minGap)
}
