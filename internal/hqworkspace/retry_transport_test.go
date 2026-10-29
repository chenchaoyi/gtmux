package hqworkspace

import "testing"

func TestAcknowledgedRetryGetsNewTransportIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(t.TempDir())
	if _, err := s.Submit("message-a", "", "synthetic instruction", 100); err != nil {
		t.Fatal(err)
	}
	var ids []string
	io := DeliveryIO{Resolve: func() (Conversation, string) { return Conversation{Agent: "codex", SessionID: "hq-root"}, "" }, SendWithID: func(_ Conversation, _ string, id string) Delivery {
		ids = append(ids, id)
		return Delivery{State: "failed"}
	}}
	if _, err := s.Pump(io, 101); err != nil {
		t.Fatal(err)
	}
	if err := s.Retry("message-a", false, 102); err == nil {
		t.Fatal("uncertain retry did not need acknowledgement")
	}
	if err := s.Retry("message-a", true, 103); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pump(io, 104); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] == ids[1] {
		t.Fatal("explicit retry reused uncertain native request", ids)
	}
}
