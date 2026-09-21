package constants

import (
	"testing"
	"time"
)

func TestBridgeAssetTransitionGraph(t *testing.T) {
	if !CanTransition(BridgeAssetTransitions, "active", "restricted") {
		t.Fatalf("expected active -> restricted transition to be allowed")
	}
	if CanTransition(BridgeAssetTransitions, "active", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}

func TestPriorityRequiresRetest(t *testing.T) {
	for _, status := range []string{"restrict", "urgent"} {
		if !PriorityRequiresRetest(status) {
			t.Fatalf("%s finalization must carry a retest obligation", status)
		}
	}
	for _, status := range []string{"draft", "observe"} {
		if PriorityRequiresRetest(status) {
			t.Fatalf("%s must not carry a retest obligation", status)
		}
	}
}

func TestRetestWindow(t *testing.T) {
	for _, risk := range []string{"high", "critical"} {
		if got := RetestWindow(risk); got != 3*24*time.Hour {
			t.Fatalf("risk %s must retest within 3 days, got %v", risk, got)
		}
	}
	for _, risk := range []string{"low", "medium"} {
		if got := RetestWindow(risk); got != 7*24*time.Hour {
			t.Fatalf("risk %s must retest within 7 days, got %v", risk, got)
		}
	}
}
