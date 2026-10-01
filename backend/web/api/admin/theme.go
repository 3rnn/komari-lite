package admin

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/web/api"
	"github.com/komari-monitor/komari/web/public"
)

// UpdateThemeSettings saves the supported display options for the fixed local
// Glass theme. Theme packages are intentionally not installable, replaceable,
// selectable, or remotely updateable in the monitoring-only build.
func UpdateThemeSettings(c *gin.Context) {
	theme := c.Query("theme")
	if theme != public.DefaultTheme {
		api.RespondError(c, http.StatusNotFound, "Theme not found")
		return
	}

	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "Invalid parameters: "+err.Error())
		return
	}

	db := dbcore.GetDBInstance()
	if err := validatePingThemeSettings(req, func(id uint) bool {
		var count int64
		return db.Model(&models.PingTask{}).Where("id = ?", id).Count(&count).Error == nil && count > 0
	}); err != nil {
		api.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	data, err := json.Marshal(&req)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to generate theme configuration: "+err.Error())
		return
	}

	var themeCfg models.ThemeConfiguration
	if err := db.Where("short = ?", public.DefaultTheme).
		Assign(models.ThemeConfiguration{Short: public.DefaultTheme, Data: string(data)}).
		FirstOrCreate(&themeCfg).Error; err != nil {
		api.RespondError(c, http.StatusInternalServerError, "Failed to save theme configuration: "+err.Error())
		return
	}
	api.RespondSuccess(c, nil)
}

func validatePingThemeSettings(req map[string]any, taskExists func(uint) bool) error {
	if value, present := req["showCarrierPing"]; present {
		if _, ok := value.(bool); !ok {
			return errors.New("Invalid ping task display switch")
		}
	}
	if value, present := req["preferredPingTaskId"]; present {
		id, ok := value.(float64)
		if !ok || math.IsNaN(id) || math.IsInf(id, 0) || id < 0 || id > float64(^uint(0)) || math.Trunc(id) != id {
			return errors.New("Invalid ping task ID")
		}
		if id > 0 && !taskExists(uint(id)) {
			return errors.New("Selected ping task does not exist")
		}
	}
	return nil
}
