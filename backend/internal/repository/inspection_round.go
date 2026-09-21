package repository

import (
	"context"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"gorm.io/gorm"
)

// InspectionRoundRepository owns all persistence operations for 检查批次.
type InspectionRoundRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.InspectionRound], error)
	Get(context.Context, uint) (model.InspectionRound, error)
	Create(context.Context, *model.InspectionRound) error
	Update(context.Context, uint, uint, *model.InspectionRound) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type inspectionRoundRepository struct {
	store *Store[model.InspectionRound]
}

func NewInspectionRoundRepository(db *gorm.DB) InspectionRoundRepository {
	return &inspectionRoundRepository{store: NewStore[model.InspectionRound](db)}
}

func (r *inspectionRoundRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.InspectionRound], error) {
	return r.store.List(ctx, q)
}
func (r *inspectionRoundRepository) Get(ctx context.Context, id uint) (model.InspectionRound, error) {
	return r.store.Get(ctx, id)
}
func (r *inspectionRoundRepository) Create(ctx context.Context, item *model.InspectionRound) error {
	return r.store.Create(ctx, item)
}
func (r *inspectionRoundRepository) Update(ctx context.Context, id, version uint, item *model.InspectionRound) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *inspectionRoundRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *inspectionRoundRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
