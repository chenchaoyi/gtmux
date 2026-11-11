package codexbridge

import (
	"context"
	"errors"
	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/gorilla/websocket"
)

func dialForAdmission(ctx context.Context, dial func(context.Context) (*websocket.Conn, error)) (*websocket.Conn, error) {
	c, err := dial(ctx)
	if ctx.Err() != nil {
		if c != nil {
			_ = c.Close()
		}
		return nil, ctx.Err()
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, &resume.FrontendAdmissionError{Code: "daemon-unavailable", Retryable: true}
	}
	return c, nil
}
