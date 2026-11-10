package screenshot

import "context"

// ReadWithin returns at cancellation even if a read-only observation settles
// later. The worker owns its result and cannot publish authority after return.
// Do not use this boundary around a mutation that could continue after timeout.
func ReadWithin[T any](ctx context.Context, read func() T) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	done := make(chan T, 1)
	go func() { done <- read() }()
	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	case value := <-done:
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		return value, nil
	}
}
