package app

import (
	"github.com/chenchaoyi/gtmux/internal/agentcaller"
	"github.com/chenchaoyi/gtmux/internal/hqpane"
)

type callerIdentity = agentcaller.Identity

func currentCaller() (callerIdentity, bool) { return agentcaller.Current() }
func currentCallerIsHQ() bool               { return agentcaller.IsHQ() }
func commandDiagnosticActor() string        { return agentcaller.DiagnosticActor() }
func callerHQPane() string                  { return hqpane.Find() }
