package repository

import (
	"context"
	"strings"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/constants"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/dto"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"gorm.io/gorm"
)

// PriorityDecisionRepository owns all persistence operations for 优先级决定.
type PriorityDecisionRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.PriorityDecision], error)
	Get(context.Context, uint) (model.PriorityDecision, error)
	CreateWithRevision(context.Context, *model.PriorityDecision, *model.PriorityDecisionRevision) error
	UpdateWithRevision(context.Context, uint, uint, *model.PriorityDecision, *model.PriorityDecisionRevision) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
	HasOverdueUnretestedForDefect(context.Context, string, uint, time.Time) (bool, error)
	OverdueDefectCodes(context.Context, []string, time.Time) (map[string]bool, error)
}

type priorityDecisionRepository struct {
	db    *gorm.DB
	store *Store[model.PriorityDecision]
}

func NewPriorityDecisionRepository(db *gorm.DB) PriorityDecisionRepository {
	return &priorityDecisionRepository{db: db, store: NewStore[model.PriorityDecision](db)}
}

func (r *priorityDecisionRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.PriorityDecision], error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)
	db := r.db.WithContext(ctx).Model(&model.PriorityDecision{})
	if search := strings.TrimSpace(strings.ToLower(q.Search)); search != "" {
		wildcard := "%" + search + "%"
		db = db.Where("LOWER(code) LIKE ? OR LOWER(name) LIKE ?", wildcard, wildcard)
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return Page[model.PriorityDecision]{}, err
	}
	items := make([]model.PriorityDecision, 0)
	err := db.Preload("Revisions", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("version ASC")
	}).Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return Page[model.PriorityDecision]{Items: items, Total: total, Page: page, PageSize: pageSize}, err
}
func (r *priorityDecisionRepository) Get(ctx context.Context, id uint) (model.PriorityDecision, error) {
	var item model.PriorityDecision
	err := r.db.WithContext(ctx).Preload("Revisions", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("version ASC")
	}).First(&item, id).Error
	return item, err
}
func (r *priorityDecisionRepository) CreateWithRevision(ctx context.Context, item *model.PriorityDecision, revision *model.PriorityDecisionRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		item.Revisions = nil
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		revision.PriorityDecisionID = item.ID
		return tx.Create(revision).Error
	})
}
func (r *priorityDecisionRepository) UpdateWithRevision(ctx context.Context, id, version uint, item *model.PriorityDecision, revision *model.PriorityDecisionRevision) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.PriorityDecision{}).
			Where("id = ? AND version = ?", id, version).
			Select("*").Omit("ID", "Code", "CreatedAt", "DeletedAt", "PreparedBy", "Revisions").
			Updates(item)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrVersionConflict
		}
		revision.PriorityDecisionID = id
		return tx.Create(revision).Error
	})
}
func (r *priorityDecisionRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *priorityDecisionRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}

// overdueUnretested filters finalized restrict/urgent decisions whose retest
// deadline passed without a registered conclusion. The original decision is
// kept; only the derived overdue flag is affected.
func overdueUnretested(db *gorm.DB, now time.Time) *gorm.DB {
	return db.
		Where("status IN ?", []string{string(constants.PriorityLevelRestrict), string(constants.PriorityLevelUrgent)}).
		Where("retest_deadline IS NOT NULL AND retest_deadline < ?", now).
		Where("COALESCE(retest_conclusion, '') = ''")
}

func (r *priorityDecisionRepository) HasOverdueUnretestedForDefect(ctx context.Context, relatedCode string, excludeID uint, now time.Time) (bool, error) {
	var count int64
	err := overdueUnretested(r.db.WithContext(ctx).Model(&model.PriorityDecision{}), now).
		Where("related_code = ? AND id <> ?", relatedCode, excludeID).
		Count(&count).Error
	return count > 0, err
}

func (r *priorityDecisionRepository) OverdueDefectCodes(ctx context.Context, codes []string, now time.Time) (map[string]bool, error) {
	overdue := make(map[string]bool, len(codes))
	if len(codes) == 0 {
		return overdue, nil
	}
	rows := make([]string, 0)
	err := overdueUnretested(r.db.WithContext(ctx).Model(&model.PriorityDecision{}), now).
		Where("related_code IN ?", codes).
		Distinct().Pluck("related_code", &rows).Error
	for _, code := range rows {
		overdue[code] = true
	}
	return overdue, err
}
