package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/constants"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/repository"
)

type PriorityDecisionService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.PriorityDecision], error)
	Get(context.Context, uint) (model.PriorityDecision, error)
	Create(context.Context, dto.CreatePriorityDecision, string, string) (model.PriorityDecision, error)
	Update(context.Context, uint, dto.UpdatePriorityDecision, string, string, string) (model.PriorityDecision, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string, string) (model.PriorityDecision, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type priorityDecisionService struct {
	repository repository.PriorityDecisionRepository
	security   SecurityService
}

func NewPriorityDecisionService(repo repository.PriorityDecisionRepository, security SecurityService) PriorityDecisionService {
	return &priorityDecisionService{repository: repo, security: security}
}

func (s *priorityDecisionService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.PriorityDecision], error) {
	return s.repository.List(ctx, query)
}

func (s *priorityDecisionService) Get(ctx context.Context, id uint) (model.PriorityDecision, error) {
	return s.repository.Get(ctx, id)
}

func (s *priorityDecisionService) Create(ctx context.Context, input dto.CreatePriorityDecision, actor, requestID string) (model.PriorityDecision, error) {
	if err := validatePriorityDecisionBusinessFields(input.Code, input.Name, input.Facility, input.Owner, input.Evidence, input.RelatedCode); err != nil {
		return model.PriorityDecision{}, err
	}
	item := model.PriorityDecision{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.PriorityDecisionInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
		PreparedBy:  actor,
	}
	revision, err := newPriorityRevision(item, "decision draft created", actor, requestID)
	if err != nil {
		return model.PriorityDecision{}, err
	}
	if err := s.repository.CreateWithRevision(ctx, &item, &revision); err != nil {
		return model.PriorityDecision{}, fmt.Errorf("create 优先级决定: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "PriorityDecision", item.ID, "", item.Status, "created 优先级决定")
	return s.repository.Get(ctx, item.ID)
}

func (s *priorityDecisionService) Update(ctx context.Context, id uint, input dto.UpdatePriorityDecision, actor, role, requestID string) (model.PriorityDecision, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.PriorityDecision{}, err
	}
	if current.Status != model.PriorityDecisionInitialStatus {
		return model.PriorityDecision{}, ErrDecisionLocked
	}
	if actor != current.PreparedBy && role != model.RoleAdmin {
		return model.PriorityDecision{}, ErrNotDecisionOwner
	}
	if err := validatePriorityDecisionBusinessFields(current.Code, input.Name, input.Facility, input.Owner, input.Evidence, input.RelatedCode); err != nil {
		return model.PriorityDecision{}, err
	}
	current.Name = strings.TrimSpace(input.Name)
	current.Description = strings.TrimSpace(input.Description)
	current.Facility = strings.TrimSpace(input.Facility)
	current.Owner = strings.TrimSpace(input.Owner)
	current.Category = strings.TrimSpace(input.Category)
	current.RiskLevel = input.RiskLevel
	current.MetricValue = input.MetricValue
	current.MetricUnit = strings.TrimSpace(input.MetricUnit)
	current.EffectiveAt = input.EffectiveAt.UTC()
	current.Evidence = strings.TrimSpace(input.Evidence)
	current.RelatedCode = strings.ToUpper(strings.TrimSpace(input.RelatedCode))
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	revision, err := newPriorityRevision(current, "draft business fields updated", actor, requestID)
	if err != nil {
		return model.PriorityDecision{}, err
	}
	if err := s.repository.UpdateWithRevision(ctx, id, input.ExpectedVersion, &current, &revision); err != nil {
		return model.PriorityDecision{}, fmt.Errorf("update 优先级决定: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "PriorityDecision", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *priorityDecisionService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, role, requestID string) (model.PriorityDecision, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.PriorityDecision{}, err
	}
	if role != model.RoleReviewer && role != model.RoleAdmin {
		return model.PriorityDecision{}, ErrReviewRole
	}
	if actor == current.PreparedBy {
		return model.PriorityDecision{}, ErrSeparationOfDuty
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.PriorityDecisionTransitions, current.Status, target) {
		return model.PriorityDecision{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	revision, err := newPriorityRevision(current, strings.TrimSpace(input.Reason), actor, requestID)
	if err != nil {
		return model.PriorityDecision{}, err
	}
	if err := s.repository.UpdateWithRevision(ctx, id, input.ExpectedVersion, &current, &revision); err != nil {
		return model.PriorityDecision{}, fmt.Errorf("transition 优先级决定: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "PriorityDecision", id, before, target, input.Reason); err != nil {
		return model.PriorityDecision{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *priorityDecisionService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if current.Status != model.PriorityDecisionInitialStatus {
		return ErrDecisionLocked
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "PriorityDecision", id, current.Status, "deleted", "soft deleted 优先级决定")
}

func (s *priorityDecisionService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validatePriorityDecisionBusinessFields(code, name, facility, owner, evidence, relatedCode string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" || strings.TrimSpace(evidence) == "" || strings.TrimSpace(relatedCode) == "" {
		return ErrInvalidInput
	}
	return nil
}

func newPriorityRevision(item model.PriorityDecision, reason, actor, requestID string) (model.PriorityDecisionRevision, error) {
	item.Revisions = nil
	snapshot, err := json.Marshal(item)
	if err != nil {
		return model.PriorityDecisionRevision{}, fmt.Errorf("serialize priority decision revision: %w", err)
	}
	return model.PriorityDecisionRevision{
		Version: item.Version, Status: item.Status, Evidence: item.Evidence,
		Reason: strings.TrimSpace(reason), Actor: actor, RequestID: requestID,
		Snapshot: string(snapshot), CreatedAt: time.Now().UTC(),
	}, nil
}
