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

type DefectFindingService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.DefectFinding], error)
	Get(context.Context, uint) (model.DefectFinding, error)
	Create(context.Context, dto.CreateDefectFinding, string, string) (model.DefectFinding, error)
	Update(context.Context, uint, dto.UpdateDefectFinding, string, string) (model.DefectFinding, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.DefectFinding, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type defectFindingService struct {
	repository repository.DefectFindingRepository
	security   SecurityService
}

func NewDefectFindingService(repo repository.DefectFindingRepository, security SecurityService) DefectFindingService {
	return &defectFindingService{repository: repo, security: security}
}

func (s *defectFindingService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.DefectFinding], error) {
	return s.repository.List(ctx, query)
}

func (s *defectFindingService) Get(ctx context.Context, id uint) (model.DefectFinding, error) {
	return s.repository.Get(ctx, id)
}

func (s *defectFindingService) Create(ctx context.Context, input dto.CreateDefectFinding, actor, requestID string) (model.DefectFinding, error) {
	if err := validateDefectFindingBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.DefectFinding{}, err
	}
	item := model.DefectFinding{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.DefectFindingInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.DefectFinding{}, fmt.Errorf("create 缺陷发现: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "DefectFinding", item.ID, "", item.Status, "created 缺陷发现")
	return item, nil
}

func (s *defectFindingService) Update(ctx context.Context, id uint, input dto.UpdateDefectFinding, actor, requestID string) (model.DefectFinding, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.DefectFinding{}, err
	}
	if err := validateDefectFindingBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.DefectFinding{}, err
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
		return model.DefectFinding{}, fmt.Errorf("update 缺陷发现: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "DefectFinding", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *defectFindingService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.DefectFinding, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.DefectFinding{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.DefectFindingTransitions, current.Status, target) {
		return model.DefectFinding{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.DefectFinding{}, fmt.Errorf("transition 缺陷发现: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "DefectFinding", id, before, target, input.Reason); err != nil {
		return model.DefectFinding{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *defectFindingService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "DefectFinding", id, current.Status, "deleted", "soft deleted 缺陷发现")
}

func (s *defectFindingService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateDefectFindingBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
