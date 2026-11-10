package codexbridge

import (
	"context"
	"errors"
	"github.com/gorilla/websocket"
	"testing"
)

func TestCFR007ActualAdmissionDialKeepsCancellationAndDeadlineReasons(t *testing.T) {
	for _, kind := range []string{"cancelled", "deadline", "cancel-in-dial", "unavailable", "ready"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, e := dialForAdmission(ctx, func(context.Context) (*websocket.Conn, error) {
				switch kind {
				case "cancelled":
					return nil, context.Canceled
				case "deadline":
					return nil, context.DeadlineExceeded
				case "cancel-in-dial":
					cancel()
					return nil, errors.New("closed")
				case "ready":
					return nil, nil
				}
				return nil, errors.New("offline")
			})
			want := "daemon-unavailable"
			switch kind {
			case "cancelled", "cancel-in-dial":
				want = "admission-cancelled"
			case "deadline":
				want = "admission-deadline"
			case "ready":
				if e != nil {
					t.Fatal(e)
				}
				return
			}
			if admissionReason(e) != want || admissionStatus(e) != 503 {
				t.Fatal(kind, e, admissionReason(e))
			}
		})
	}
}
