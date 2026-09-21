package model

import "time"

// PriorityDecision models 优先级决定 as an independently versioned aggregate. The fields
// cover ownership, operational context, evidence and measured risk so later
// changes naturally span persistence, service and UI layers.
type PriorityDecision struct {
	BaseModel
	Facility    string                     `json:"facility" gorm:"size:120;index"`
	Owner       string                     `json:"owner" gorm:"size:120;index"`
	Category    string                     `json:"category" gorm:"size:80;index"`
	RiskLevel   string                     `json:"riskLevel" gorm:"size:32;index"`
	MetricValue float64                    `json:"metricValue"`
	MetricUnit  string                     `json:"metricUnit" gorm:"size:24"`
	EffectiveAt time.Time                  `json:"effectiveAt"`
	Evidence    string                     `json:"evidence" gorm:"size:2000"`
	RelatedCode string                     `json:"relatedCode" gorm:"size:64;index"`
	PreparedBy  string                     `json:"preparedBy" gorm:"size:80;index;not null"`
	// RetestDueAt opens when the decision is finalized as restrict/urgent and
	// stays recorded after the retest so the window remains auditable.
	RetestDueAt      *time.Time                 `json:"retestDueAt"`
	RetestConclusion string                     `json:"retestConclusion" gorm:"size:500"`
	RetestBy         string                     `json:"retestBy" gorm:"size:80"`
	RetestAt         *time.Time                 `json:"retestAt"`
	RetestOverdue    bool                       `json:"retestOverdue" gorm:"-"`
	Revisions        []PriorityDecisionRevision `json:"revisions" gorm:"foreignKey:PriorityDecisionID;constraint:OnDelete:CASCADE"`
}

func (item *PriorityDecision) GetBase() *BaseModel { return &item.BaseModel }

func (item PriorityDecision) TableName() string { return "priority_decisions" }

var PriorityDecisionInitialStatus = "draft"

// RetestDueDaysHighRisk and RetestDueDaysDefault bound the retest window that
// opens when a decision is finalized as restrict or urgent.
const (
	RetestDueDaysHighRisk = 3
	RetestDueDaysDefault  = 7
)

// RetestRequired reports whether a finalized priority level opens a retest window.
func RetestRequired(status string) bool { return status == "restrict" || status == "urgent" }

// RetestDueDeadline computes the retest deadline from the finalization time:
// high-risk decisions get a shorter window than the rest.
func RetestDueDeadline(riskLevel string, finalizedAt time.Time) time.Time {
	days := RetestDueDaysDefault
	if riskLevel == "high" || riskLevel == "critical" {
		days = RetestDueDaysHighRisk
	}
	return finalizedAt.AddDate(0, 0, days)
}

// RetestPending reports whether a finalized decision still waits for a retest
// conclusion to be registered.
func (item *PriorityDecision) RetestPending() bool {
	return item.RetestDueAt != nil && item.RetestAt == nil
}

// RefreshRetestOverdue derives the display-only overdue flag. The persisted
// decision keeps its original status even while the retest is overdue.
func (item *PriorityDecision) RefreshRetestOverdue(now time.Time) {
	item.RetestOverdue = item.RetestPending() && now.After(*item.RetestDueAt)
}

// PriorityDecisionRevision is append-only. It is written in the same
// transaction as the aggregate so an accepted version can always be traced
// back to its evidence, actor and request.
type PriorityDecisionRevision struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	PriorityDecisionID uint      `json:"priorityDecisionId" gorm:"uniqueIndex:idx_priority_revision,priority:1;not null"`
	Version            uint      `json:"version" gorm:"uniqueIndex:idx_priority_revision,priority:2;not null"`
	Status             string    `json:"status" gorm:"size:40;index;not null"`
	Evidence           string    `json:"evidence" gorm:"size:2000;not null"`
	Reason             string    `json:"reason" gorm:"size:500;not null"`
	Actor              string    `json:"actor" gorm:"size:80;index;not null"`
	RequestID          string    `json:"requestId" gorm:"size:64;index;not null"`
	Snapshot           string    `json:"snapshot" gorm:"type:text;not null"`
	CreatedAt          time.Time `json:"createdAt" gorm:"index"`
}
