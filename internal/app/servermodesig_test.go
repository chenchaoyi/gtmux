package app

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/servermode"
)

// The signature is what a remote surface shows of server mode. What moves on its own
// (the heartbeat, `since`) must not change it, or every slow tick would push an event;
// what the phone shows must.
func TestServerModeSigMovesOnlyWithWhatIsShown(t *testing.T) {
	on := servermode.Status{State: servermode.StateOn, SystemDisableSleep: true, OwnedByGtmux: true,
		Power: "battery", BatteryPct: 64, Guard: servermode.Guard{Installed: true, Healthy: true}}
	base := serverModeSig(on)

	same := on
	same.HeartbeatAt, same.Since = 999, 888
	if serverModeSig(same) != base {
		t.Error("the heartbeat or since changed the signature")
	}
	for name, mut := range map[string]func(*servermode.Status){
		"state":         func(s *servermode.Status) { s.State = servermode.StateLapsed },
		"unknown":       func(s *servermode.Status) { s.State = servermode.StateUnknown },
		"kernel":        func(s *servermode.Status) { s.SystemDisableSleep = false },
		"guard":         func(s *servermode.Status) { s.Guard.Healthy = false },
		"power":         func(s *servermode.Status) { s.Power = "ac" },
		"charge":        func(s *servermode.Status) { s.BatteryPct = 30 },
		"exit":          func(s *servermode.Status) { s.LastExit = &servermode.Exit{At: 5, Reason: "revoked"} },
		"another owner": func(s *servermode.Status) { s.OwnedByGtmux = false },
	} {
		st := on
		mut(&st)
		if serverModeSig(st) == base {
			t.Errorf("%s did not change the signature", name)
		}
	}

	// Off and not ours: a laptop charging is not server-mode news.
	off := servermode.Status{State: servermode.StateOff, Power: "battery", BatteryPct: 64}
	charging := off
	charging.Power, charging.BatteryPct = "ac", 65
	if serverModeSig(off) != serverModeSig(charging) {
		t.Error("power and charge changed the signature while server mode is off")
	}
}

func TestServerModeSignatureIsEmptyUntilRead(t *testing.T) {
	lastServerModeSig.Store("")
	if serverModeSignature() != "" {
		t.Error("a signature before any read")
	}
	lastServerModeSig.Store(serverModeSig(servermode.Status{State: servermode.StateOff}))
	if serverModeSignature() == "" {
		t.Error("no signature after a read")
	}
}
