package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/config"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestDefectFindingReflectsOverdueRetest(t *testing.T) {
	dsn := fmt.Sprintf("file:defect-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DefectFinding{}, &model.PriorityDecision{}, &model.PriorityDecisionRevision{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	priorityRepo := repository.NewPriorityDecisionRepository(db)
	defects := NewDefectFindingService(repository.NewDefectFindingRepository(db), priorityRepo, security)
	priorities := NewPriorityDecisionService(priorityRepo, security)
	ctx := context.Background()

	defect, err := defects.Create(ctx, dto.CreateDefectFinding{
		Code: "DF-RETEST", Name: "复测联动缺陷", Facility: "K42 bridge", Owner: "infrastructure team",
		Category: "structural", RiskLevel: "high", EffectiveAt: time.Now().UTC(),
		Evidence: "裂缝照片", RelatedCode: "BA-001",
	}, "operator", "req-defect")
	if err != nil {
		t.Fatalf("create defect: %v", err)
	}
	if defect.RetestOverdue {
		t.Fatalf("new defect must not be overdue: %+v", defect)
	}

	decisionInput := priorityCreateInput("PD-RETEST", "decision evidence")
	decisionInput.RelatedCode = "DF-RETEST"
	decision, err := priorities.Create(ctx, decisionInput, "operator", "req-decision")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	finalized, err := priorities.Transition(ctx, decision.ID, dto.TransitionRequest{Status: "urgent", ExpectedVersion: 1, Reason: "紧急复核"}, "reviewer", model.RoleReviewer, "req-decision-final")
	if err != nil {
		t.Fatalf("finalize decision: %v", err)
	}

	fresh, err := defects.Get(ctx, defect.ID)
	if err != nil {
		t.Fatalf("get defect before overdue: %v", err)
	}
	if fresh.RetestOverdue {
		t.Fatalf("defect must not be overdue while the retest window is open: %+v", fresh)
	}

	past := time.Now().UTC().Add(-time.Hour)
	if err := db.Model(&model.PriorityDecision{}).Where("id = ?", finalized.ID).UpdateColumn("retest_due_at", past).Error; err != nil {
		t.Fatalf("force overdue retest window: %v", err)
	}
	stale, err := defects.Get(ctx, defect.ID)
	if err != nil {
		t.Fatalf("get defect after overdue: %v", err)
	}
	if !stale.RetestOverdue || stale.Status != "new" {
		t.Fatalf("defect must show 逾期待复测 and keep its own status: %+v", stale)
	}
	page, err := defects.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list defects: %v", err)
	}
	if len(page.Items) != 1 || !page.Items[0].RetestOverdue {
		t.Fatalf("defect list must surface the overdue retest flag: %+v", page.Items)
	}

	if _, err := priorities.RegisterRetest(ctx, finalized.ID, dto.RegisterRetestConclusion{ExpectedVersion: finalized.Version, Conclusion: "复测合格"}, "reviewer", model.RoleReviewer, "req-retest"); err != nil {
		t.Fatalf("register retest conclusion: %v", err)
	}
	cleared, err := defects.Get(ctx, defect.ID)
	if err != nil {
		t.Fatalf("get defect after retest: %v", err)
	}
	if cleared.RetestOverdue {
		t.Fatalf("registered retest must clear the defect overdue flag: %+v", cleared)
	}
}
