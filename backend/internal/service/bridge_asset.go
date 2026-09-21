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

type BridgeAssetService interface {
	List(context.Context, dto.PageQuery) (repository.Page[model.BridgeAsset], error)
	Get(context.Context, uint) (model.BridgeAsset, error)
	Create(context.Context, dto.CreateBridgeAsset, string, string) (model.BridgeAsset, error)
	Update(context.Context, uint, dto.UpdateBridgeAsset, string, string) (model.BridgeAsset, error)
	Transition(context.Context, uint, dto.TransitionRequest, string, string) (model.BridgeAsset, error)
	Delete(context.Context, uint, string, string) error
	StatusCounts(context.Context) (map[string]int64, error)
}

type bridgeAssetService struct {
	repository repository.BridgeAssetRepository
	security   SecurityService
}

func NewBridgeAssetService(repo repository.BridgeAssetRepository, security SecurityService) BridgeAssetService {
	return &bridgeAssetService{repository: repo, security: security}
}

func (s *bridgeAssetService) List(ctx context.Context, query dto.PageQuery) (repository.Page[model.BridgeAsset], error) {
	return s.repository.List(ctx, query)
}

func (s *bridgeAssetService) Get(ctx context.Context, id uint) (model.BridgeAsset, error) {
	return s.repository.Get(ctx, id)
}

func (s *bridgeAssetService) Create(ctx context.Context, input dto.CreateBridgeAsset, actor, requestID string) (model.BridgeAsset, error) {
	if err := validateBridgeAssetBusinessFields(input.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.BridgeAsset{}, err
	}
	item := model.BridgeAsset{
		BaseModel: model.BaseModel{
			Code: strings.ToUpper(strings.TrimSpace(input.Code)), Name: strings.TrimSpace(input.Name),
			Status: model.BridgeAssetInitialStatus, Version: 1, Description: strings.TrimSpace(input.Description),
		},
		Facility: strings.TrimSpace(input.Facility), Owner: strings.TrimSpace(input.Owner),
		Category: strings.TrimSpace(input.Category), RiskLevel: input.RiskLevel,
		MetricValue: input.MetricValue, MetricUnit: strings.TrimSpace(input.MetricUnit),
		EffectiveAt: input.EffectiveAt.UTC(), Evidence: strings.TrimSpace(input.Evidence),
		RelatedCode: strings.ToUpper(strings.TrimSpace(input.RelatedCode)),
	}
	if err := s.repository.Create(ctx, &item); err != nil {
		return model.BridgeAsset{}, fmt.Errorf("create 桥梁资产: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "create", "BridgeAsset", item.ID, "", item.Status, "created 桥梁资产")
	return item, nil
}

func (s *bridgeAssetService) Update(ctx context.Context, id uint, input dto.UpdateBridgeAsset, actor, requestID string) (model.BridgeAsset, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.BridgeAsset{}, err
	}
	if err := validateBridgeAssetBusinessFields(current.Code, input.Name, input.Facility, input.Owner); err != nil {
		return model.BridgeAsset{}, err
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
		return model.BridgeAsset{}, fmt.Errorf("update 桥梁资产: %w", err)
	}
	_ = s.security.Audit(ctx, actor, requestID, "update", "BridgeAsset", id, current.Status, current.Status, "updated business fields")
	return s.repository.Get(ctx, id)
}

func (s *bridgeAssetService) Transition(ctx context.Context, id uint, input dto.TransitionRequest, actor, requestID string) (model.BridgeAsset, error) {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return model.BridgeAsset{}, err
	}
	target := strings.TrimSpace(input.Status)
	if !constants.CanTransition(constants.BridgeAssetTransitions, current.Status, target) {
		return model.BridgeAsset{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, current.Status, target)
	}
	before := current.Status
	current.Status = target
	current.Version = input.ExpectedVersion + 1
	current.UpdatedAt = time.Now().UTC()
	if err := s.repository.Update(ctx, id, input.ExpectedVersion, &current); err != nil {
		return model.BridgeAsset{}, fmt.Errorf("transition 桥梁资产: %w", err)
	}
	if err := s.security.Audit(ctx, actor, requestID, "transition", "BridgeAsset", id, before, target, input.Reason); err != nil {
		return model.BridgeAsset{}, fmt.Errorf("persist transition audit: %w", err)
	}
	return s.repository.Get(ctx, id)
}

func (s *bridgeAssetService) Delete(ctx context.Context, id uint, actor, requestID string) error {
	current, err := s.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, id); err != nil {
		return err
	}
	return s.security.Audit(ctx, actor, requestID, "delete", "BridgeAsset", id, current.Status, "deleted", "soft deleted 桥梁资产")
}

func (s *bridgeAssetService) StatusCounts(ctx context.Context) (map[string]int64, error) {
	return s.repository.CountByStatus(ctx)
}

func validateBridgeAssetBusinessFields(code, name, facility, owner string) error {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(facility) == "" || strings.TrimSpace(owner) == "" {
		return ErrInvalidInput
	}
	return nil
}
