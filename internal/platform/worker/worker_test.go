package worker

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRun_FiresImmediatelyOnStart(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go Run(ctx, discardLogger(), Job{
		Name:     "test",
		Interval: time.Hour,
		Fn: func(context.Context) (int64, error) {
			if calls.Add(1) == 1 {
				close(done)
			}
			return 0, nil
		},
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not fire immediately on Run start")
	}
	cancel()
}

func TestRun_ContextCancellationStopsAllJobs(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())

	jobA := make(chan struct{}, 1)
	jobB := make(chan struct{}, 1)

	finished := make(chan struct{})
	go func() {
		Run(ctx, discardLogger(),
			Job{
				Name:     "a",
				Interval: time.Hour,
				Fn: func(context.Context) (int64, error) {
					select {
					case jobA <- struct{}{}:
					default:
					}
					return 0, nil
				},
			},
			Job{
				Name:     "b",
				Interval: time.Hour,
				Fn: func(context.Context) (int64, error) {
					select {
					case jobB <- struct{}{}:
					default:
					}
					return 0, nil
				},
			},
		)
		close(finished)
	}()

	<-jobA
	<-jobB
	cancel()

	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestRun_ErrorDoesNotStopFutureTicks(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	thirdCall := make(chan struct{})

	go Run(t.Context(), discardLogger(), Job{
		Name:     "test",
		Interval: 10 * time.Millisecond,
		Fn: func(context.Context) (int64, error) {
			n := calls.Add(1)
			if n == 3 {
				close(thirdCall)
			}
			return 0, errors.New("simulated failure")
		},
	})

	select {
	case <-thirdCall:
	case <-time.After(2 * time.Second):
		t.Fatalf("job did not retry after error, calls = %d", calls.Load())
	}
}

func TestRun_NoJobsReturnsImmediately(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	go func() {
		Run(t.Context(), discardLogger())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run with no jobs should return immediately")
	}
}
