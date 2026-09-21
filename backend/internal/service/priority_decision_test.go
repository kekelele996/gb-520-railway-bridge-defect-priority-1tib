package service

import (
	"context"
	"errors"
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

func TestPriorityDecisionVersionedIndependentReview(t *testing.T) {
	service, _ := newPriorityTestService(t)
	ctx := context.Background()
	created, err := service.Create(ctx, priorityCreateInput("PD-TEST", "evidence-v1"), "operator", "req-create")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	if created.PreparedBy != "operator" || created.Version != 1 || len(created.Revisions) != 1 {
		t.Fatalf("unexpected created decision: %+v", created)
	}

	updated, err := service.Update(ctx, created.ID, priorityUpdateInput(created.Version, "evidence-v2"), "operator", model.RoleOperator, "req-update")
	if err != nil {
		t.Fatalf("update decision: %v", err)
	}
	if updated.Version != 2 || len(updated.Revisions) != 2 {
		t.Fatalf("expected two immutable revisions, got version=%d revisions=%d", updated.Version, len(updated.Revisions))
	}

	transition := dto.TransitionRequest{Status: "urgent", ExpectedVersion: updated.Version, Reason: "independent safety review"}
	if _, err := service.Transition(ctx, updated.ID, transition, "operator", model.RoleOperator, "req-operator-final"); !errors.Is(err, ErrReviewRole) {
		t.Fatalf("operator finalization should fail with review role error, got %v", err)
	}
	if _, err := service.Transition(ctx, updated.ID, transition, "operator", model.RoleReviewer, "req-self-final"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("preparer self-approval should fail, got %v", err)
	}

	finalized, err := service.Transition(ctx, updated.ID, transition, "reviewer", model.RoleReviewer, "req-final")
	if err != nil {
		t.Fatalf("independent review: %v", err)
	}
	if finalized.Status != "urgent" || finalized.Version != 3 || len(finalized.Revisions) != 3 {
		t.Fatalf("unexpected finalized decision: %+v", finalized)
	}
	wantEvidence := []string{"evidence-v1", "evidence-v2", "evidence-v2"}
	wantActors := []string{"operator", "operator", "reviewer"}
	wantRequests := []string{"req-create", "req-update", "req-final"}
	for index, revision := range finalized.Revisions {
		if revision.Version != uint(index+1) || revision.Evidence != wantEvidence[index] || revision.Actor != wantActors[index] || revision.RequestID != wantRequests[index] || revision.Snapshot == "" {
			t.Fatalf("revision %d lost audit evidence: %+v", index+1, revision)
		}
	}
	if _, err := service.Update(ctx, finalized.ID, priorityUpdateInput(finalized.Version, "late overwrite"), "operator", model.RoleOperator, "req-late"); !errors.Is(err, ErrDecisionLocked) {
		t.Fatalf("final decision must be immutable, got %v", err)
	}
}

func TestPriorityDecisionRetestWindowAndRegistration(t *testing.T) {
	service, _ := newPriorityTestService(t)
	ctx := context.Background()

	highRisk, err := service.Create(ctx, priorityCreateInput("PD-HIGH", "high risk evidence"), "operator", "req-high")
	if err != nil {
		t.Fatalf("create high risk decision: %v", err)
	}
	beforeFinal := time.Now().UTC()
	finalized, err := service.Transition(ctx, highRisk.ID, dto.TransitionRequest{Status: "restrict", ExpectedVersion: 1, Reason: "限速复核"}, "reviewer", model.RoleReviewer, "req-high-final")
	if err != nil {
		t.Fatalf("finalize high risk decision: %v", err)
	}
	if finalized.RetestDueAt == nil || finalized.RetestAt != nil || finalized.RetestOverdue {
		t.Fatalf("finalized restrict decision must open a pending retest window: %+v", finalized)
	}
	if delta := finalized.RetestDueAt.Sub(beforeFinal); delta < 72*time.Hour || delta > 73*time.Hour {
		t.Fatalf("critical risk retest window should be 3 days, got %v", delta)
	}

	lowRiskInput := priorityCreateInput("PD-LOW", "low risk evidence")
	lowRiskInput.RiskLevel = "low"
	lowRisk, err := service.Create(ctx, lowRiskInput, "operator", "req-low")
	if err != nil {
		t.Fatalf("create low risk decision: %v", err)
	}
	lowFinal, err := service.Transition(ctx, lowRisk.ID, dto.TransitionRequest{Status: "urgent", ExpectedVersion: 1, Reason: "紧急复核"}, "reviewer", model.RoleReviewer, "req-low-final")
	if err != nil {
		t.Fatalf("finalize low risk decision: %v", err)
	}
	if delta := lowFinal.RetestDueAt.Sub(beforeFinal); delta < 7*24*time.Hour || delta > 7*24*time.Hour+time.Hour {
		t.Fatalf("low risk retest window should be 7 days, got %v", delta)
	}

	observeInput := priorityCreateInput("PD-OBSERVE", "observe evidence")
	observeDraft, err := service.Create(ctx, observeInput, "operator", "req-observe")
	if err != nil {
		t.Fatalf("create observe decision: %v", err)
	}
	observed, err := service.Transition(ctx, observeDraft.ID, dto.TransitionRequest{Status: "observe", ExpectedVersion: 1, Reason: "观察复核"}, "reviewer", model.RoleReviewer, "req-observe-final")
	if err != nil {
		t.Fatalf("finalize observe decision: %v", err)
	}
	if observed.RetestDueAt != nil {
		t.Fatalf("observe decision must not open a retest window: %+v", observed)
	}
	if _, err := service.RegisterRetest(ctx, observed.ID, dto.RegisterRetestConclusion{ExpectedVersion: observed.Version, Conclusion: "无需复测"}, "reviewer", model.RoleReviewer, "req-observe-retest"); !errors.Is(err, ErrRetestNotPending) {
		t.Fatalf("observe decision has no retest window, got %v", err)
	}

	retestInput := dto.RegisterRetestConclusion{ExpectedVersion: finalized.Version, Conclusion: "复测合格，限速维持"}
	if _, err := service.RegisterRetest(ctx, finalized.ID, retestInput, "operator", model.RoleOperator, "req-retest-role"); !errors.Is(err, ErrRetestRole) {
		t.Fatalf("operator must not register retest conclusion, got %v", err)
	}
	if _, err := service.RegisterRetest(ctx, finalized.ID, retestInput, "operator", model.RoleReviewer, "req-retest-self"); !errors.Is(err, ErrRetestSeparation) {
		t.Fatalf("preparer must not register retest conclusion, got %v", err)
	}
	registered, err := service.RegisterRetest(ctx, finalized.ID, retestInput, "reviewer", model.RoleReviewer, "req-retest")
	if err != nil {
		t.Fatalf("register retest conclusion: %v", err)
	}
	if registered.RetestAt == nil || registered.RetestBy != "reviewer" || registered.RetestConclusion != "复测合格，限速维持" || registered.RetestOverdue {
		t.Fatalf("retest registration not persisted: %+v", registered)
	}
	if registered.Version != finalized.Version+1 || len(registered.Revisions) != len(finalized.Revisions)+1 {
		t.Fatalf("retest registration must append an immutable revision, got version=%d revisions=%d", registered.Version, len(registered.Revisions))
	}
	lastRevision := registered.Revisions[len(registered.Revisions)-1]
	if lastRevision.Actor != "reviewer" || lastRevision.RequestID != "req-retest" || lastRevision.Status != "restrict" {
		t.Fatalf("retest revision lost audit evidence: %+v", lastRevision)
	}
	if _, err := service.RegisterRetest(ctx, finalized.ID, dto.RegisterRetestConclusion{ExpectedVersion: registered.Version, Conclusion: "重复登记"}, "reviewer", model.RoleReviewer, "req-retest-again"); !errors.Is(err, ErrRetestNotPending) {
		t.Fatalf("retest conclusion must be registered only once, got %v", err)
	}
}

func TestPriorityDecisionOverdueRetestBlocksNewFinalization(t *testing.T) {
	service, db := newPriorityTestService(t)
	ctx := context.Background()

	overdueDraft, err := service.Create(ctx, priorityCreateInput("PD-OVERDUE", "overdue evidence"), "operator", "req-overdue")
	if err != nil {
		t.Fatalf("create overdue decision: %v", err)
	}
	finalized, err := service.Transition(ctx, overdueDraft.ID, dto.TransitionRequest{Status: "urgent", ExpectedVersion: 1, Reason: "紧急复核"}, "reviewer", model.RoleReviewer, "req-overdue-final")
	if err != nil {
		t.Fatalf("finalize overdue decision: %v", err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	if err := db.Model(&model.PriorityDecision{}).Where("id = ?", finalized.ID).UpdateColumn("retest_due_at", past).Error; err != nil {
		t.Fatalf("force overdue retest window: %v", err)
	}

	stale, err := service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("get overdue decision: %v", err)
	}
	if !stale.RetestOverdue || stale.Status != "urgent" {
		t.Fatalf("overdue decision must keep its status and show 逾期待复测: %+v", stale)
	}
	page, err := service.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	found := false
	for _, item := range page.Items {
		if item.ID == finalized.ID {
			found = item.RetestOverdue
		}
	}
	if !found {
		t.Fatalf("decision list must surface the overdue retest flag")
	}

	blocker, err := service.Create(ctx, priorityCreateInput("PD-BLOCKED", "blocked evidence"), "operator", "req-blocked")
	if err != nil {
		t.Fatalf("create blocked decision: %v", err)
	}
	if _, err := service.Transition(ctx, blocker.ID, dto.TransitionRequest{Status: "restrict", ExpectedVersion: 1, Reason: "尝试定稿"}, "reviewer", model.RoleReviewer, "req-blocked-final"); !errors.Is(err, ErrRetestOverdue) {
		t.Fatalf("overdue retest must block new finalization for the same defect, got %v", err)
	}

	if _, err := service.RegisterRetest(ctx, finalized.ID, dto.RegisterRetestConclusion{ExpectedVersion: finalized.Version, Conclusion: "复测完成，解除逾期"}, "reviewer", model.RoleReviewer, "req-overdue-retest"); err != nil {
		t.Fatalf("register overdue retest conclusion: %v", err)
	}
	cleared, err := service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("get cleared decision: %v", err)
	}
	if cleared.RetestOverdue || cleared.RetestAt == nil {
		t.Fatalf("registered retest must clear the overdue flag: %+v", cleared)
	}
	unblocked, err := service.Transition(ctx, blocker.ID, dto.TransitionRequest{Status: "restrict", ExpectedVersion: 1, Reason: "逾期清除后定稿"}, "reviewer", model.RoleReviewer, "req-unblocked-final")
	if err != nil {
		t.Fatalf("finalization must succeed after retest registration: %v", err)
	}
	if unblocked.Status != "restrict" || unblocked.RetestDueAt == nil {
		t.Fatalf("unexpected unblocked decision: %+v", unblocked)
	}
}

func newPriorityTestService(t *testing.T) (PriorityDecisionService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:priority-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PriorityDecision{}, &model.PriorityDecisionRevision{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	security := NewSecurityService(repository.NewSecurityRepository(db), config.Config{})
	return NewPriorityDecisionService(repository.NewPriorityDecisionRepository(db), security), db
}

func priorityCreateInput(code, evidence string) dto.CreatePriorityDecision {
	return dto.CreatePriorityDecision{
		Code: code, Name: "桥梁缺陷处置决定", Description: "versioning test", Facility: "K42 bridge",
		Owner: "infrastructure team", Category: "structural", RiskLevel: "critical", MetricValue: 87,
		MetricUnit: "score", EffectiveAt: time.Now().UTC(), Evidence: evidence, RelatedCode: "DF-TEST",
	}
}

func priorityUpdateInput(version uint, evidence string) dto.UpdatePriorityDecision {
	return dto.UpdatePriorityDecision{
		ExpectedVersion: version, Name: "桥梁缺陷处置决定", Description: "updated version", Facility: "K42 bridge",
		Owner: "infrastructure team", Category: "structural", RiskLevel: "critical", MetricValue: 92,
		MetricUnit: "score", EffectiveAt: time.Now().UTC(), Evidence: evidence, RelatedCode: "DF-TEST",
	}
}
