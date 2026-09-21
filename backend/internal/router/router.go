package router

import (
	"log/slog"
	"net/http"

	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/config"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/handler"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/middleware"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/model"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/repository"
	"github.com/blueship581/railway-bridge-defect-priority/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	engine.Use(middleware.RequestContext(logger))
	if len(cfg.TrustedProxies) > 0 {
		_ = engine.SetTrustedProxies(cfg.TrustedProxies)
	} else {
		_ = engine.SetTrustedProxies(nil)
	}

	securityRepository := repository.NewSecurityRepository(db)
	securityService := service.NewSecurityService(securityRepository, cfg)
	bridgeAssetRepository := repository.NewBridgeAssetRepository(db)
	inspectionRoundRepository := repository.NewInspectionRoundRepository(db)
	defectFindingRepository := repository.NewDefectFindingRepository(db)
	priorityDecisionRepository := repository.NewPriorityDecisionRepository(db)
	bridgeAssetService := service.NewBridgeAssetService(bridgeAssetRepository, securityService)
	inspectionRoundService := service.NewInspectionRoundService(inspectionRoundRepository, securityService)
	defectFindingService := service.NewDefectFindingService(defectFindingRepository, securityService)
	priorityDecisionService := service.NewPriorityDecisionService(priorityDecisionRepository, securityService)
	bridgeAssetHandler := handler.NewBridgeAssetHandler(bridgeAssetService)
	inspectionRoundHandler := handler.NewInspectionRoundHandler(inspectionRoundService)
	defectFindingHandler := handler.NewDefectFindingHandler(defectFindingService)
	priorityDecisionHandler := handler.NewPriorityDecisionHandler(priorityDecisionService)
	systemHandler := handler.NewSystemHandler(securityService, bridgeAssetService, inspectionRoundService, defectFindingService, priorityDecisionService, db, redisClient)

	engine.GET("/healthz", systemHandler.Health)
	engine.POST("/api/auth/login", systemHandler.Login)

	limiter := middleware.NewLimiter(redisClient, cfg.RequestLimit)
	api := engine.Group("/api")
	api.Use(limiter.Middleware(), middleware.Authenticate(cfg))
	api.GET("/overview", systemHandler.Overview)
	api.GET("/audits", middleware.RequireMinimumRole(model.RoleReviewer), systemHandler.Audits)
	api.GET("/session", systemHandler.Session)
	api.GET("/runtime", systemHandler.Runtime)
	api.GET("/audit-summary", middleware.RequireMinimumRole(model.RoleReviewer), systemHandler.AuditSummary)
	api.GET("/audits/:entityType/:id", middleware.RequireMinimumRole(model.RoleReviewer), systemHandler.EntityHistory)
	bridgeAssetHandler.Register(api)
	inspectionRoundHandler.Register(api)
	defectFindingHandler.Register(api)
	priorityDecisionHandler.Register(api)

	engine.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "route_not_found"})
	})
	return engine
}
