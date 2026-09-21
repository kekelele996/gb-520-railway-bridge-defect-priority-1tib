package repository

import (
	"context"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"gorm.io/gorm"
)

// BridgeAssetRepository owns all persistence operations for 桥梁资产.
type BridgeAssetRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.BridgeAsset], error)
	Get(context.Context, uint) (model.BridgeAsset, error)
	Create(context.Context, *model.BridgeAsset) error
	Update(context.Context, uint, uint, *model.BridgeAsset) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type bridgeAssetRepository struct {
	store *Store[model.BridgeAsset]
}

func NewBridgeAssetRepository(db *gorm.DB) BridgeAssetRepository {
	return &bridgeAssetRepository{store: NewStore[model.BridgeAsset](db)}
}

func (r *bridgeAssetRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.BridgeAsset], error) {
	return r.store.List(ctx, q)
}
func (r *bridgeAssetRepository) Get(ctx context.Context, id uint) (model.BridgeAsset, error) {
	return r.store.Get(ctx, id)
}
func (r *bridgeAssetRepository) Create(ctx context.Context, item *model.BridgeAsset) error {
	return r.store.Create(ctx, item)
}
func (r *bridgeAssetRepository) Update(ctx context.Context, id, version uint, item *model.BridgeAsset) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *bridgeAssetRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *bridgeAssetRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
