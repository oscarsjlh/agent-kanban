package domain

import "testing"

func TestWaitingRequiresReason(t *testing.T) {
	if err := CanMove(Inbox, Waiting, false, false); err == nil {
		t.Fatal("expected error")
	}
	if err := CanMove(Inbox, Waiting, true, false); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalIssuesCannotMove(t *testing.T) {
	if err := CanMove(Done, Ready, false, false); err == nil {
		t.Fatal("expected done terminal error")
	}
	if err := CanMove(Wontfix, Ready, false, false); err == nil {
		t.Fatal("expected wontfix terminal error")
	}
}

func TestClaimExclusivity(t *testing.T) {
	if err := CanClaim("agent-a"); err == nil {
		t.Fatal("expected claimed error")
	}
	if err := CanClaim(""); err != nil {
		t.Fatal(err)
	}
	if err := CanRelease("agent-a", "agent-b"); err == nil {
		t.Fatal("expected wrong claimer error")
	}
	if err := CanRelease("agent-a", "agent-a"); err != nil {
		t.Fatal(err)
	}
}
