package constants

import "time"

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type DefectState string

const (
	DefectStateNew        DefectState = "new"
	DefectStateVerified   DefectState = "verified"
	DefectStateMonitoring DefectState = "monitoring"
	DefectStateMitigated  DefectState = "mitigated"
	DefectStateClosed     DefectState = "closed"
)

var AllDefectState = []string{"new", "verified", "monitoring", "mitigated", "closed"}

type PriorityLevel string

const (
	PriorityLevelObserve  PriorityLevel = "observe"
	PriorityLevelRestrict PriorityLevel = "restrict"
	PriorityLevelUrgent   PriorityLevel = "urgent"
)

var AllPriorityLevel = []string{"observe", "restrict", "urgent"}

var BridgeAssetTransitions = map[string]map[string]bool{
	"active":     {"restricted": true, "closed": true},
	"restricted": {"closed": true, "retired": true, "active": true},
	"closed":     {"retired": true, "restricted": true},
	"retired":    {"closed": true},
}

var InspectionRoundTransitions = map[string]map[string]bool{
	"planned":   {"running": true, "review": true},
	"running":   {"review": true, "completed": true, "planned": true},
	"review":    {"completed": true, "running": true},
	"completed": {"review": true},
}

var DefectFindingTransitions = map[string]map[string]bool{
	"new":        {"verified": true, "monitoring": true},
	"verified":   {"monitoring": true, "mitigated": true, "new": true},
	"monitoring": {"mitigated": true, "closed": true, "verified": true},
	"mitigated":  {"closed": true, "monitoring": true},
	"closed":     {"mitigated": true},
}

var PriorityDecisionTransitions = map[string]map[string]bool{
	"draft":    {"observe": true, "restrict": true, "urgent": true},
	"observe":  {},
	"restrict": {},
	"urgent":   {},
}

// Retest deadlines: 限速/紧急 finalizations require a retest conclusion before
// the deadline. High-risk defects get a shorter window than the rest.
const (
	RetestWindowHighRisk = 3 * 24 * time.Hour
	RetestWindowDefault  = 7 * 24 * time.Hour
)

// PriorityRequiresRetest reports whether a finalized status carries a retest
// obligation. observe decisions are monitored without a retest deadline.
func PriorityRequiresRetest(status string) bool {
	return status == string(PriorityLevelRestrict) || status == string(PriorityLevelUrgent)
}

// RetestWindow returns the deadline window for a finalized decision based on
// the recorded risk level: high/critical defects must be retested sooner.
func RetestWindow(riskLevel string) time.Duration {
	if riskLevel == "high" || riskLevel == "critical" {
		return RetestWindowHighRisk
	}
	return RetestWindowDefault
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}
