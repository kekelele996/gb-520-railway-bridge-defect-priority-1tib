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
	Revisions   []PriorityDecisionRevision `json:"revisions" gorm:"foreignKey:PriorityDecisionID;constraint:OnDelete:CASCADE"`
}

func (item *PriorityDecision) GetBase() *BaseModel { return &item.BaseModel }

func (item PriorityDecision) TableName() string { return "priority_decisions" }

var PriorityDecisionInitialStatus = "draft"

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
