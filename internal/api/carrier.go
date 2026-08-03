package api

import (
	"net/http"
	"strings"

	carrierconfig "github.com/voorz/vohive/internal/carrier"

	"github.com/gin-gonic/gin"
)

// handleListCarriers GET /api/carrier
func (s *Server) handleListCarriers(c *gin.Context) {
	list, err := carrierconfig.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询运营商列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// handleGetCarrier GET /api/carrier/:mcc/:mnc
func (s *Server) handleGetCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	if mcc == "" || mnc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "mcc 和 mnc 不能为空"})
		return
	}
	detail, err := carrierconfig.Get(mcc, mnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询运营商详情失败: " + err.Error()})
		return
	}
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "运营商未找到"})
		return
	}
	c.JSON(http.StatusOK, detail)
}

// handleAddCarrier POST /api/carrier
func (s *Server) handleAddCarrier(c *gin.Context) {
	var payload carrierconfig.AddPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误: " + err.Error()})
		return
	}
	if err := carrierconfig.Add(payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "添加运营商失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商已添加"})
}

// handleBatchImportCarriers POST /api/carrier/batch
func (s *Server) handleBatchImportCarriers(c *gin.Context) {
	var req struct {
		Mode     string                       `json:"mode"`
		Carriers []carrierconfig.AddPayload   `json:"carriers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误: " + err.Error()})
		return
	}
	if req.Mode == "" {
		req.Mode = "skip_existing"
	}
	result, err := carrierconfig.BatchImport(req.Carriers, req.Mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "批量导入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// handleRemoveCarrier DELETE /api/carrier/:mcc/:mnc
func (s *Server) handleRemoveCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	if err := carrierconfig.Remove(mcc, mnc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "删除运营商失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商已删除"})
}

// handleSaveCarrierConfig PUT /api/carrier/:mcc/:mnc
func (s *Server) handleSaveCarrierConfig(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	var payload carrierconfig.SavePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误: " + err.Error()})
		return
	}
	if err := carrierconfig.Save(mcc, mnc, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "配置已保存"})
}

// handleDeleteCarrierConfig DELETE /api/carrier/:mcc/:mnc/config
func (s *Server) handleDeleteCarrierConfig(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	if err := carrierconfig.DeleteConfig(mcc, mnc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "删除配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "用户配置已删除"})
}

// handleActivateCarrier POST /api/carrier/:mcc/:mnc/activate
func (s *Server) handleActivateCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	if err := carrierconfig.Activate(mcc, mnc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "激活配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商配置已激活"})
}

// handleDeactivateCarrier POST /api/carrier/:mcc/:mnc/deactivate
func (s *Server) handleDeactivateCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	if err := carrierconfig.Deactivate(mcc, mnc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "禁用配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商配置已禁用"})
}

// handleListCarrierDefaults GET /api/carrier/defaults
func (s *Server) handleListCarrierDefaults(c *gin.Context) {
	list, err := carrierconfig.ListSystemDefaults()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询系统默认列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// handleGetCarrierDefault GET /api/carrier/defaults/:mcc/:mnc
func (s *Server) handleGetCarrierDefault(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	p, err := carrierconfig.GetSystemDefault(mcc, mnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询系统默认失败: " + err.Error()})
		return
	}
	if p == nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "系统默认配置未找到"})
		return
	}
	c.JSON(http.StatusOK, p)
}
