package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/config"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.Config, log *slog.Logger) (*gorm.DB, *redis.Client, error) {
	var dialector gorm.Dialector
	switch cfg.DatabaseDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DatabaseDSN)
	case "mysql":
		dialector = mysql.Open(cfg.DatabaseDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DatabaseDSN)
	default:
		return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.DatabaseDriver)
	}
	logLevel := logger.Warn
	if cfg.Environment == "development" {
		logLevel = logger.Info
	}
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 20; attempt++ {
		db, err = gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logLevel)})
		if err == nil {
			sqlDB, dbErr := db.DB()
			if dbErr == nil && sqlDB.PingContext(ctx) == nil {
				break
			}
			if dbErr != nil {
				err = dbErr
			} else {
				err = sqlDB.PingContext(ctx)
			}
		}
		log.Warn("database not ready", "attempt", attempt, "error", err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect database: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, nil, err
	}
	if err := Seed(ctx, db); err != nil {
		return nil, nil, err
	}
	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, nil, fmt.Errorf("connect redis: %w", err)
		}
	}
	return db, redisClient, nil
}

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{}, &model.AuditLog{},
		&model.BridgeAsset{},
		&model.InspectionRound{},
		&model.DefectFinding{},
		&model.PriorityDecision{},
		&model.PriorityDecisionRevision{},
	)
}

func Seed(ctx context.Context, db *gorm.DB) error {
	var users int64
	if err := db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return err
	}
	if users == 0 {
		password, err := bcrypt.GenerateFromPassword([]byte("Admin123!"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		seedUsers := []model.User{
			{Username: "admin", DisplayName: "系统管理员", PasswordHash: string(password), Role: model.RoleAdmin, Active: true},
			{Username: "reviewer", DisplayName: "质量复核员", PasswordHash: string(password), Role: model.RoleReviewer, Active: true},
			{Username: "operator", DisplayName: "现场操作员", PasswordHash: string(password), Role: model.RoleOperator, Active: true},
			{Username: "viewer", DisplayName: "只读观察员", PasswordHash: string(password), Role: model.RoleViewer, Active: true},
		}
		if err := db.WithContext(ctx).Create(&seedUsers).Error; err != nil {
			return err
		}
	}

	if err := seedBridgeAsset(ctx, db); err != nil {
		return err
	}

	if err := seedInspectionRound(ctx, db); err != nil {
		return err
	}

	if err := seedDefectFinding(ctx, db); err != nil {
		return err
	}

	if err := seedPriorityDecision(ctx, db); err != nil {
		return err
	}

	return nil
}

func seedBridgeAsset(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.BridgeAsset{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.BridgeAsset{

		{BaseModel: model.BaseModel{Code: "BA-001", Name: "桥梁资产示例一", Status: "active", Version: 1,
			Description: "用于启动验证和主要流程演示的桥梁资产记录"}, Facility: "铁路桥梁缺陷处置优先级区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-01"},

		{BaseModel: model.BaseModel{Code: "BA-002", Name: "桥梁资产示例二", Status: "restricted", Version: 1,
			Description: "用于启动验证和主要流程演示的桥梁资产记录"}, Facility: "铁路桥梁缺陷处置优先级区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-02"},

		{BaseModel: model.BaseModel{Code: "BA-003", Name: "桥梁资产示例三", Status: "closed", Version: 1,
			Description: "用于启动验证和主要流程演示的桥梁资产记录"}, Facility: "铁路桥梁缺陷处置优先级区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedInspectionRound(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.InspectionRound{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.InspectionRound{

		{BaseModel: model.BaseModel{Code: "IR-001", Name: "检查批次示例一", Status: "planned", Version: 1,
			Description: "用于启动验证和主要流程演示的检查批次记录"}, Facility: "铁路桥梁缺陷处置优先级区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-01"},

		{BaseModel: model.BaseModel{Code: "IR-002", Name: "检查批次示例二", Status: "running", Version: 1,
			Description: "用于启动验证和主要流程演示的检查批次记录"}, Facility: "铁路桥梁缺陷处置优先级区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-02"},

		{BaseModel: model.BaseModel{Code: "IR-003", Name: "检查批次示例三", Status: "review", Version: 1,
			Description: "用于启动验证和主要流程演示的检查批次记录"}, Facility: "铁路桥梁缺陷处置优先级区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedDefectFinding(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.DefectFinding{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.DefectFinding{

		{BaseModel: model.BaseModel{Code: "DF-001", Name: "缺陷发现示例一", Status: "new", Version: 1,
			Description: "用于启动验证和主要流程演示的缺陷发现记录"}, Facility: "铁路桥梁缺陷处置优先级区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-01"},

		{BaseModel: model.BaseModel{Code: "DF-002", Name: "缺陷发现示例二", Status: "verified", Version: 1,
			Description: "用于启动验证和主要流程演示的缺陷发现记录"}, Facility: "铁路桥梁缺陷处置优先级区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-02"},

		{BaseModel: model.BaseModel{Code: "DF-003", Name: "缺陷发现示例三", Status: "monitoring", Version: 1,
			Description: "用于启动验证和主要流程演示的缺陷发现记录"}, Facility: "铁路桥梁缺陷处置优先级区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "REL-520-03"},
	}
	return db.WithContext(ctx).Create(&items).Error
}

func seedPriorityDecision(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.PriorityDecision{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	now := time.Now().UTC()
	items := []model.PriorityDecision{

		{BaseModel: model.BaseModel{Code: "PD-001", Name: "优先级决定示例一", Status: "draft", Version: 1,
			Description: "用于启动验证和主要流程演示的优先级决定记录"}, Facility: "铁路桥梁缺陷处置优先级区域1", Owner: "运行一组",
			Category: "常规", RiskLevel: "low", MetricValue: 12.5, MetricUnit: "unit",
			EffectiveAt: now.Add(0 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "DF-001", PreparedBy: "operator"},

		{BaseModel: model.BaseModel{Code: "PD-002", Name: "优先级决定示例二", Status: "observe", Version: 2,
			Description: "用于启动验证和主要流程演示的优先级决定记录"}, Facility: "铁路桥梁缺陷处置优先级区域2", Owner: "质量复核组",
			Category: "重点", RiskLevel: "medium", MetricValue: 25.0, MetricUnit: "%",
			EffectiveAt: now.Add(3 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "DF-002", PreparedBy: "operator"},

		{BaseModel: model.BaseModel{Code: "PD-003", Name: "优先级决定示例三", Status: "restrict", Version: 2,
			Description: "用于启动验证和主要流程演示的优先级决定记录"}, Facility: "铁路桥梁缺陷处置优先级区域3", Owner: "安全主管组",
			Category: "复核", RiskLevel: "high", MetricValue: 37.5, MetricUnit: "score",
			EffectiveAt: now.Add(6 * time.Hour), Evidence: "已完成基础证据核对", RelatedCode: "DF-003", PreparedBy: "operator"},
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index := range items {
			item := &items[index]
			if err := tx.Create(item).Error; err != nil {
				return err
			}
			revisions := []model.PriorityDecisionRevision{{
				PriorityDecisionID: item.ID, Version: 1, Status: "draft", Evidence: item.Evidence,
				Reason: "seeded decision draft", Actor: "operator", RequestID: fmt.Sprintf("seed-%s-create", item.Code),
				Snapshot: fmt.Sprintf(`{"code":%q,"status":"draft","evidence":%q}`, item.Code, item.Evidence), CreatedAt: now,
			}}
			if item.Status != "draft" {
				revisions = append(revisions, model.PriorityDecisionRevision{
					PriorityDecisionID: item.ID, Version: item.Version, Status: item.Status, Evidence: item.Evidence,
					Reason: "seeded independent review", Actor: "reviewer", RequestID: fmt.Sprintf("seed-%s-review", item.Code),
					Snapshot: fmt.Sprintf(`{"code":%q,"status":%q,"evidence":%q}`, item.Code, item.Status, item.Evidence), CreatedAt: now.Add(time.Minute),
				})
			}
			if err := tx.Create(&revisions).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
