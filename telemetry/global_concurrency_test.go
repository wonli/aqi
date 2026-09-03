package telemetry

import (
	"context"
	"sync"
	"testing"
)

type testProvider struct{}

type testSpan struct{}

func (testProvider) Start(ctx context.Context, _ string, _ Fields) (context.Context, Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	span := testSpan{}
	return ContextWithSpan(ctx, span), span
}

func (testSpan) SetFields(Fields)             {}
func (testSpan) RecordError(error)            {}
func (testSpan) SetStatus(StatusCode, string) {}
func (testSpan) End()                         {}

func TestGlobalProviderConcurrentSetGetAndStart(t *testing.T) {
	SetProvider(nil)
	defer SetProvider(nil)

	const workers = 32
	const iterations = 1000

	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(worker int) {
			defer wg.Done()
			<-start

			for j := 0; j < iterations; j++ {
				switch worker % 3 {
				case 0:
					if j%2 == 0 {
						SetProvider(testProvider{})
					} else {
						SetProvider(nil)
					}
				case 1:
					if GetProvider() == nil {
						t.Error("GetProvider returned nil")
						return
					}
				default:
					ctx, span := Start(context.Background(), "test", Fields{"worker": worker})
					if ctx == nil || span == nil {
						t.Error("Start returned nil context or span")
						return
					}
				}
			}
		}(i)
	}

	close(start)
	wg.Wait()
}
