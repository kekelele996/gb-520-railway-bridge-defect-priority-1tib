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
	service := newPriorityTestService(t)
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

func newPriorityTestService(t *testing.T) PriorityDecisionService {
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
	return NewPriorityDecisionService(repository.NewPriorityDecisionRepository(db), security)
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
