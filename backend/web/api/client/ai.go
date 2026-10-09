package client

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/aimonitor"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/web/api"
	"gorm.io/gorm"
	"io"
	"net/http"
	"time"
)

// AI reporting is isolated from ordinary metrics and accepts Agent credentials
// only, even though legacy report routes also allow administrative callers.
func AIReport(c *gin.Context) { AIReportHandler(aimonitor.NewStore(dbcore.GetDBInstance()))(c) }
func AIReportHandler(store *aimonitor.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal := api.GetPrincipal(c)
		if principal == nil || principal.Type != rpc.PrincipalAgent || principal.ClientUUID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Agent node credential required"})
			return
		}
		if c.ContentType() != "application/json" {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "application/json required"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, aimonitor.MaxPayloadBytes)
		raw, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "AI payload is too large or unreadable"})
			return
		}
		var report aimonitor.Report
		if err = aimonitor.DecodeStrict(raw, &report); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid AI report schema"})
			return
		}
		now := time.Now().UTC()
		if err = report.Validate(now); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		s := aimonitor.NewStore(store.DB.WithContext(ctx))
		if report.Enabled || len(report.Samples) > 0 || report.Quota != nil {
			var profile models.ClientDeploymentProfile
			err = s.DB.First(&profile, "client = ?", principal.ClientUUID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI profile unavailable"})
				return
			}
			var config struct {
				EnableHermesAI bool `json:"enable_hermes_ai"`
			}
			if err != nil || json.Unmarshal([]byte(profile.Config), &config) != nil || !config.EnableHermesAI {
				c.JSON(http.StatusForbidden, gin.H{"error": "Hermes AI monitoring must be explicitly enabled in the node profile"})
				return
			}
		}
		accepted, err := s.Ingest(principal.ClientUUID, report, now)
		if errors.Is(err, aimonitor.ErrRateLimited) {
			c.Header("Retry-After", "2")
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI storage unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"version": 1, "accepted_sample_ids": accepted})
	}
}
