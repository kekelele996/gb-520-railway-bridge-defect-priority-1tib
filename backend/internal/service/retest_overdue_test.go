package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/config"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/constants"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/repository"
	"gorm.io/gorm"
)

func TestRetestDeadlineAssignedOnFinalization(t *testing.T) {
	service := newPriorityTestService(t)
	ctx := context.Background()

	cases := []struct {
		code      string
		riskLevel string
		target    string
		window    time.Duration
		wantDue   bool
	}{
		{"PD-RT-HIGH", "high", "restrict", constants.RetestWindowHighRisk, true},
		{"PD-RT-CRITICAL", "critical", "urgent", constants.RetestWindowHighRisk, true},
		{"PD-RT-LOW", "low", "urgent", constants.RetestWindowDefault, true},
		{"PD-RT-MEDIUM", "medium", "restrict", constants.RetestWindowDefault, true},
		{"PD-RT-OBSERVE", "high", "observe", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			input := priorityCreateInput(tc.code, "retest window evidence")
			input.RiskLevel = tc.riskLevel
			created, err := service.Create(ctx, input, "operator", "req-create-"+tc.code)
			if err != nil {
				t.Fatalf("create decision: %v", err)
			}
			if created.RetestDeadline != nil {
				t.Fatalf("draft must not carry a retest deadline: %+v", created.RetestDeadline)
			}
			started := time.Now().UTC()
			finalized, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
				Status: tc.target, ExpectedVersion: created.Version, Reason: "independent review finalizes",
			}, "reviewer", model.RoleReviewer, "req-final-"+tc.code)
			if err != nil {
				t.Fatalf("finalize to %s: %v", tc.target, err)
			}
			if !tc.wantDue {
				if finalized.RetestDeadline != nil {
					t.Fatalf("observe decisions require no retest, got deadline %v", finalized.RetestDeadline)
				}
				return
			}
			if finalized.RetestDeadline == nil {
				t.Fatalf("%s finalization must stamp a retest deadline", tc.target)
			}
			if finalized.RetestOverdue {
				t.Fatalf("fresh deadline must not be overdue: %+v", finalized)
			}
			earliest := started.Add(tc.window)
			latest := time.Now().UTC().Add(tc.window)
			if finalized.RetestDeadline.Before(earliest) || finalized.RetestDeadline.After(latest) {
				t.Fatalf("deadline %v outside expected window [%v, %v]", finalized.RetestDeadline, earliest, latest)
			}
		})
	}
}

func TestRetestOverdueBlocksFinalizationUntilRegistered(t *testing.T) {
	service, db := newPriorityTestEnv(t)
	ctx := context.Background()

	overdueInput := priorityCreateInput("PD-RT-OVERDUE", "overdue evidence")
	overdueInput.RiskLevel = "critical"
	overdueInput.RelatedCode = "DF-OVERDUE"
	created, err := service.Create(ctx, overdueInput, "operator", "req-overdue-create")
	if err != nil {
		t.Fatalf("create overdue candidate: %v", err)
	}
	finalized, err := service.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "urgent", ExpectedVersion: created.Version, Reason: "urgent disposition",
	}, "reviewer", model.RoleReviewer, "req-overdue-final")
	if err != nil {
		t.Fatalf("finalize overdue candidate: %v", err)
	}
	if finalized.RetestOverdue {
		t.Fatal("decision with a future deadline must not be overdue")
	}
	backdateRetestDeadline(t, db, finalized.ID, time.Now().UTC().Add(-time.Hour))

	stale, err := service.Get(ctx, finalized.ID)
	if err != nil {
		t.Fatalf("get overdue decision: %v", err)
	}
	if !stale.RetestOverdue || stale.Status != "urgent" {
		t.Fatalf("expected 逾期待复测 with the original decision kept, got %+v", stale)
	}
	page, err := service.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list decisions: %v", err)
	}
	if len(page.Items) != 1 || !page.Items[0].RetestOverdue {
		t.Fatalf("list must surface the derived overdue flag: %+v", page.Items)
	}

	blockedInput := priorityCreateInput("PD-RT-BLOCKED", "blocked evidence")
	blockedInput.RelatedCode = "DF-OVERDUE"
	blocked, err := service.Create(ctx, blockedInput, "operator", "req-blocked-create")
	if err != nil {
		t.Fatalf("create blocked candidate: %v", err)
	}
	if _, err := service.Transition(ctx, blocked.ID, dto.TransitionRequest{
		Status: "restrict", ExpectedVersion: blocked.Version, Reason: "must be refused while defect is overdue",
	}, "reviewer", model.RoleReviewer, "req-blocked-final"); !errors.Is(err, ErrDefectRetestOverdue) {
		t.Fatalf("finalization for an overdue defect must fail, got %v", err)
	}

	retest := dto.RegisterRetestConclusion{ExpectedVersion: stale.Version, Conclusion: "复测合格，位移已收敛"}
	if _, err := service.RegisterRetest(ctx, stale.ID, retest, "operator", model.RoleOperator, "req-retest-role"); !errors.Is(err, ErrReviewRole) {
		t.Fatalf("operator must not register retest conclusions, got %v", err)
	}
	if _, err := service.RegisterRetest(ctx, stale.ID, retest, "operator", model.RoleReviewer, "req-retest-self"); !errors.Is(err, ErrSeparationOfDuty) {
		t.Fatalf("preparer must not register the retest conclusion, got %v", err)
	}
	registered, err := service.RegisterRetest(ctx, stale.ID, retest, "reviewer", model.RoleReviewer, "req-retest")
	if err != nil {
		t.Fatalf("independent reviewer registers retest: %v", err)
	}
	if registered.RetestConclusion != "复测合格，位移已收敛" || registered.RetestReviewedBy != "reviewer" || registered.RetestReviewedAt == nil {
		t.Fatalf("retest conclusion not persisted: %+v", registered)
	}
	if registered.RetestOverdue || registered.Status != "urgent" {
		t.Fatalf("registration must clear overdue and keep the decision, got %+v", registered)
	}
	if registered.Version != stale.Version+1 || len(registered.Revisions) != len(stale.Revisions)+1 {
		t.Fatalf("registration must append an immutable revision, got version=%d revisions=%d", registered.Version, len(registered.Revisions))
	}
	if _, err := service.RegisterRetest(ctx, stale.ID, dto.RegisterRetestConclusion{
		ExpectedVersion: registered.Version, Conclusion: "重复登记",
	}, "reviewer", model.RoleReviewer, "req-retest-again"); !errors.Is(err, ErrRetestAlreadyRegistered) {
		t.Fatalf("duplicate retest registration must fail, got %v", err)
	}

	cleared, err := service.Transition(ctx, blocked.ID, dto.TransitionRequest{
		Status: "restrict", ExpectedVersion: blocked.Version, Reason: "retest registered, finalization allowed",
	}, "reviewer", model.RoleReviewer, "req-blocked-final-2")
	if err != nil {
		t.Fatalf("finalization must succeed once the retest is registered: %v", err)
	}
	if cleared.Status != "restrict" || cleared.RetestDeadline == nil {
		t.Fatalf("expected restrict decision with a fresh retest deadline, got %+v", cleared)
	}
}

func TestRetestRegistrationRequiresFinalizedRetestStatus(t *testing.T) {
	service := newPriorityTestService(t)
	ctx := context.Background()

	draft, err := service.Create(ctx, priorityCreateInput("PD-RT-DRAFT", "draft evidence"), "operator", "req-draft")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := service.RegisterRetest(ctx, draft.ID, dto.RegisterRetestConclusion{
		ExpectedVersion: draft.Version, Conclusion: "草稿无需复测",
	}, "reviewer", model.RoleReviewer, "req-draft-retest"); !errors.Is(err, ErrRetestNotRequired) {
		t.Fatalf("draft retest registration must fail, got %v", err)
	}

	observed, err := service.Transition(ctx, draft.ID, dto.TransitionRequest{
		Status: "observe", ExpectedVersion: draft.Version, Reason: "keep monitoring",
	}, "reviewer", model.RoleReviewer, "req-observe")
	if err != nil {
		t.Fatalf("finalize observe: %v", err)
	}
	if _, err := service.RegisterRetest(ctx, observed.ID, dto.RegisterRetestConclusion{
		ExpectedVersion: observed.Version, Conclusion: "观察无需复测",
	}, "reviewer", model.RoleReviewer, "req-observe-retest"); !errors.Is(err, ErrRetestNotRequired) {
		t.Fatalf("observe retest registration must fail, got %v", err)
	}
}

func TestDefectFindingFlagsOverdueRetest(t *testing.T) {
	priorities, db := newPriorityTestEnv(t)
	defects := NewDefectFindingService(
		repository.NewDefectFindingRepository(db),
		repository.NewPriorityDecisionRepository(db),
		NewSecurityService(repository.NewSecurityRepository(db), config.Config{}),
	)
	ctx := context.Background()

	defect, err := defects.Create(ctx, dto.CreateDefectFinding{
		Code: "DF-RET", Name: "支座位移缺陷", Facility: "K42 bridge", Owner: "infrastructure team",
		RiskLevel: "high", EffectiveAt: time.Now().UTC(), RelatedCode: "IR-001",
	}, "operator", "req-defect")
	if err != nil {
		t.Fatalf("create defect: %v", err)
	}
	if defect.RetestOverdue {
		t.Fatal("defect without finalized decisions must not be overdue")
	}

	input := priorityCreateInput("PD-RT-DEFECT", "defect retest evidence")
	input.RelatedCode = "DF-RET"
	created, err := priorities.Create(ctx, input, "operator", "req-defect-decision")
	if err != nil {
		t.Fatalf("create decision: %v", err)
	}
	finalized, err := priorities.Transition(ctx, created.ID, dto.TransitionRequest{
		Status: "restrict", ExpectedVersion: created.Version, Reason: "speed limit disposition",
	}, "reviewer", model.RoleReviewer, "req-defect-final")
	if err != nil {
		t.Fatalf("finalize restrict: %v", err)
	}
	backdateRetestDeadline(t, db, finalized.ID, time.Now().UTC().Add(-2*time.Hour))

	flagged, err := defects.Get(ctx, defect.ID)
	if err != nil {
		t.Fatalf("get defect: %v", err)
	}
	if !flagged.RetestOverdue {
		t.Fatalf("defect must show 逾期待复测 while the linked decision is overdue: %+v", flagged)
	}
	page, err := defects.List(ctx, dto.PageQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list defects: %v", err)
	}
	if len(page.Items) != 1 || !page.Items[0].RetestOverdue {
		t.Fatalf("defect list must surface the overdue flag: %+v", page.Items)
	}

	if _, err := priorities.RegisterRetest(ctx, finalized.ID, dto.RegisterRetestConclusion{
		ExpectedVersion: finalized.Version, Conclusion: "复测合格",
	}, "reviewer", model.RoleReviewer, "req-defect-retest"); err != nil {
		t.Fatalf("register retest: %v", err)
	}
	cleared, err := defects.Get(ctx, defect.ID)
	if err != nil {
		t.Fatalf("get defect after retest: %v", err)
	}
	if cleared.RetestOverdue {
		t.Fatalf("registering the retest conclusion must clear the defect flag: %+v", cleared)
	}
}

func backdateRetestDeadline(t *testing.T, db *gorm.DB, id uint, deadline time.Time) {
	t.Helper()
	if err := db.Exec("UPDATE priority_decisions SET retest_deadline = ? WHERE id = ?", deadline, id).Error; err != nil {
		t.Fatalf("backdate retest deadline: %v", err)
	}
}
