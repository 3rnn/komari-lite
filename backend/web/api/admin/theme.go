package admin

import (
	"encoding/json"
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
		api.RespondError(c, http.StatusNotFound, "主题不存在")
		return
	}

	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		api.RespondError(c, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	data, err := json.Marshal(&req)
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, "生成主题配置失败: "+err.Error())
		return
	}

	db := dbcore.GetDBInstance()
	var themeCfg models.ThemeConfiguration
	if err := db.Where("short = ?", public.DefaultTheme).
		Assign(models.ThemeConfiguration{Short: public.DefaultTheme, Data: string(data)}).
		FirstOrCreate(&themeCfg).Error; err != nil {
		api.RespondError(c, http.StatusInternalServerError, "保存主题配置失败: "+err.Error())
		return
	}
	api.RespondSuccess(c, nil)
}
