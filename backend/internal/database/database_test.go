package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBackfillPriorityRetestDue(t *testing.T) {
	dsn := fmt.Sprintf("file:backfill-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := migrate(db); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	finalizedAt := time.Now().UTC().Add(-10 * 24 * time.Hour)
	legacy := []model.PriorityDecision{
		{BaseModel: model.BaseModel{Code: "PD-LEGACY-HIGH", Name: "历史高风险决定", Status: "urgent", Version: 2},
			RiskLevel: "critical", RelatedCode: "DF-LEGACY-1", PreparedBy: "operator"},
		{BaseModel: model.BaseModel{Code: "PD-LEGACY-LOW", Name: "历史低风险决定", Status: "restrict", Version: 2},
			RiskLevel: "low", RelatedCode: "DF-LEGACY-2", PreparedBy: "operator"},
		{BaseModel: model.BaseModel{Code: "PD-LEGACY-OBSERVE", Name: "历史观察决定", Status: "observe", Version: 2},
			RiskLevel: "high", RelatedCode: "DF-LEGACY-3", PreparedBy: "operator"},
	}
	for index := range legacy {
		item := &legacy[index]
		if err := db.Create(item).Error; err != nil {
			t.Fatalf("create legacy decision: %v", err)
		}
		revisions := []model.PriorityDecisionRevision{
			{PriorityDecisionID: item.ID, Version: 1, Status: "draft", Evidence: "v1", Reason: "draft", Actor: "operator", RequestID: "req-draft", Snapshot: "{}", CreatedAt: finalizedAt.Add(-time.Hour)},
			{PriorityDecisionID: item.ID, Version: 2, Status: item.Status, Evidence: "v2", Reason: "final", Actor: "reviewer", RequestID: "req-final", Snapshot: "{}", CreatedAt: finalizedAt},
		}
		if err := db.Create(&revisions).Error; err != nil {
			t.Fatalf("create legacy revisions: %v", err)
		}
	}

	if err := backfillPriorityRetestDue(db); err != nil {
		t.Fatalf("backfill retest due: %v", err)
	}

	var high model.PriorityDecision
	if err := db.First(&high, "code = ?", "PD-LEGACY-HIGH").Error; err != nil {
		t.Fatalf("load high risk decision: %v", err)
	}
	if high.RetestDueAt == nil || !high.RetestDueAt.Equal(finalizedAt.AddDate(0, 0, model.RetestDueDaysHighRisk)) {
		t.Fatalf("high risk backfill must be finalization + 3 days, got %v", high.RetestDueAt)
	}
	var low model.PriorityDecision
	if err := db.First(&low, "code = ?", "PD-LEGACY-LOW").Error; err != nil {
		t.Fatalf("load low risk decision: %v", err)
	}
	if low.RetestDueAt == nil || !low.RetestDueAt.Equal(finalizedAt.AddDate(0, 0, model.RetestDueDaysDefault)) {
		t.Fatalf("low risk backfill must be finalization + 7 days, got %v", low.RetestDueAt)
	}
	var observe model.PriorityDecision
	if err := db.First(&observe, "code = ?", "PD-LEGACY-OBSERVE").Error; err != nil {
		t.Fatalf("load observe decision: %v", err)
	}
	if observe.RetestDueAt != nil {
		t.Fatalf("observe decision must not get a retest deadline, got %v", observe.RetestDueAt)
	}

	var revisionCount int64
	if err := db.Model(&model.PriorityDecisionRevision{}).Count(&revisionCount).Error; err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revisionCount != 6 {
		t.Fatalf("backfill must not touch the existing revision chain, got %d revisions", revisionCount)
	}

	// A second run must be a no-op for already backfilled decisions.
	if err := backfillPriorityRetestDue(db); err != nil {
		t.Fatalf("rerun backfill: %v", err)
	}
	var again model.PriorityDecision
	if err := db.First(&again, "code = ?", "PD-LEGACY-HIGH").Error; err != nil {
		t.Fatalf("reload high risk decision: %v", err)
	}
	if again.RetestDueAt == nil || !again.RetestDueAt.Equal(*high.RetestDueAt) {
		t.Fatalf("backfill must be idempotent, got %v want %v", again.RetestDueAt, high.RetestDueAt)
	}
}
