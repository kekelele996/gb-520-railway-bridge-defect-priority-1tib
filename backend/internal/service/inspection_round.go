package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/constants"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/repository"
)

type InspectionRoundService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.InspectionRound], error)
	Get(context.Context, uint) (model.InspectionRound, error)
	Create(context.Context, dto.CreateInspectionRound, string, string) (model.InspectionRound, error)
	Update(context.Context, uint, dto.UpdateInspectionRound, string, string) (model.InspectionRound, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.InspectionRound, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type inspectionRoundService struct {
	repository repository.InspectionRoundRepository
	security   SecurityService
}

func NewInspectionRoundService(repo repository.InspectionRoundRepository, security SecurityService) InspectionRoundService {
	return &inspectionRoundService{repository: repo, security: security}
}

func (s *inspectionRoundService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.InspectionRound], error) {
	return s.repository.List(ctx, query)
}

func (s *inspectionRoundService) Get(ctx context.Context, id uint) (model.InspectionRound, error) {
	return s.repository.Get(ctx, id)
}

func (s *inspectionRoundService) Create(ctx context.Context, input dto.CreateInspectionRound, actor, requestID string) (model.InspectionRound, error) {
	if err := validateInspectionRoundBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.InspectionRound{}, err
	}
	item := model.InspectionRound{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.InspectionRoundInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.InspectionRound{}, fmt.Errorf("create 检查批次: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "InspectionRound", item.ID, "", item.Status, "created 检查批次")
	return item, nil
}

func (s *inspectionRoundService) Update(ctx context.Context, id uint, input dto.UpdateInspectionRound, actor, requestID string) (model.InspectionRound, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.InspectionRound{}, err
	}
	if err := validateInspectionRoundBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.InspectionRound{}, err
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
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.InspectionRound{}, fmt.Errorf("update 检查批次: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "InspectionRound", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *inspectionRoundService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.InspectionRound, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.InspectionRound{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.InspectionRoundTransitions, current.Status, target) {
		return model.InspectionRound{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.InspectionRound{}, fmt.Errorf("transition 检查批次: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "InspectionRound", id, before, target, input.Reason); err != nil {
		return model.InspectionRound{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *inspectionRoundService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "InspectionRound", id, current.Status, "deleted", "soft deleted 检查批次")
}

func (s *inspectionRoundService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateInspectionRoundBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
