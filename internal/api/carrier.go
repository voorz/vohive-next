package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	carrierconfig "github.com/voorz/vohive/internal/carrier"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vohive/pkg/logger"

	"github.com/gin-gonic/gin"
)

// makeKey constructs a profile key from path params + optional brand query param.
func makeKey(mcc, mnc, brand string) string {
	// reuse the same plmnKey logic as the carrier package
	trimmed := strings.TrimLeft(strings.TrimSpace(mnc), "0")
	if trimmed == "" && strings.TrimSpace(mnc) != "" {
		trimmed = "0"
	}
	key := strings.TrimSpace(mcc) + "-" + trimmed
	brand = strings.TrimSpace(brand)
	if brand != "" {
		key = key + "__" + brand
	}
	return key
}

// handleListCarriers GET /api/carrier
// 运营商列表只从 DB (carrier_visible + carrier_index) 读取。
// 嵌入的 profiles/*.json 仅作为配置模板 fallback，不用于构建列表。
func (s *Server) handleListCarriers(c *gin.Context) {
	// 从 carrier_visible 获取所有可见运营商
	visible, err := db.ListCarrierVisible()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "查询可见运营商列表失败: " + err.Error()})
		return
	}

	out := make([]carrierconfig.ListItem, 0, len(visible))
	for _, v := range visible {
		// 从 carrier_index 查运营商元数据
		idx, err := db.GetCarrierIndex(v.PLMN)
		if err != nil {
			logger.Warn("查询运营商索引失败", "plmn", v.PLMN, "err", err)
			continue
		}

		item := carrierconfig.ListItem{
			Key: v.PLMN,
		}

		if idx != nil {
			item.MCC = idx.MCC
			item.MNC = idx.MNC
			// 从 raw_json 提取运营商名称
			name := idx.PLMN
			var raw struct {
				Operators []struct {
					Brand    string `json:"brand"`
					Operator string `json:"operator"`
				} `json:"operators"`
			}
			if json.Unmarshal([]byte(idx.RawJSON), &raw) == nil && len(raw.Operators) > 0 {
				if raw.Operators[0].Brand != "" {
					name = raw.Operators[0].Brand
				} else if raw.Operators[0].Operator != "" {
					name = raw.Operators[0].Operator
				}
			}
			item.Name = name
		} else {
			// index 中没有，尝试从 PLMN key 解析 MCC/MNC
			parts := strings.SplitN(v.PLMN, "-", 2)
			if len(parts) == 2 {
				item.MCC = parts[0]
				item.MNC = parts[1]
			}
			item.Name = v.PLMN
		}

		out = append(out, item)
	}

	c.JSON(http.StatusOK, out)
}

// handleGetCarrier GET /api/carrier/:mcc/:mnc?brand=
func (s *Server) handleGetCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	if mcc == "" || mnc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "mcc 和 mnc 不能为空"})
		return
	}
	detail, err := carrierconfig.Get(mcc, mnc, brand)
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
		Mode     string                     `json:"mode"`
		Carriers []carrierconfig.AddPayload `json:"carriers"`
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

// handleRemoveCarrier DELETE /api/carrier/:mcc/:mnc?brand=
func (s *Server) handleRemoveCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)
	if err := carrierconfig.Remove(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "删除运营商失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商已删除"})
}

// handleSaveCarrierConfig PUT /api/carrier/:mcc/:mnc?brand=
func (s *Server) handleSaveCarrierConfig(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)
	var payload carrierconfig.SavePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误: " + err.Error()})
		return
	}
	if err := carrierconfig.Save(key, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "保存配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "配置已保存"})
}

// handleDeleteCarrierConfig DELETE /api/carrier/:mcc/:mnc/config?brand=
func (s *Server) handleDeleteCarrierConfig(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)
	if err := carrierconfig.DeleteConfig(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "删除配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "用户配置已删除"})
}

// handleActivateCarrier POST /api/carrier/:mcc/:mnc/activate?brand=
func (s *Server) handleActivateCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)
	if err := carrierconfig.Activate(key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "激活配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商配置已激活"})
}

// handleDeactivateCarrier POST /api/carrier/:mcc/:mnc/deactivate?brand=
func (s *Server) handleDeactivateCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)
	if err := carrierconfig.Deactivate(key); err != nil {
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

// handleBatchAddVisible POST /api/carriers/visible/batch
// 从 plmn-index 批量添加运营商到可见列表。
func (s *Server) handleBatchAddVisible(c *gin.Context) {
	var req struct {
		PLMNs []string `json:"plmns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "参数错误: " + err.Error()})
		return
	}
	if len(req.PLMNs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "plmns 不能为空"})
		return
	}
	if err := db.BatchAddCarrierVisible(req.PLMNs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "批量添加失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": fmt.Sprintf("已添加 %d 个运营商", len(req.PLMNs))})
}

// handleRemoveVisible DELETE /api/carriers/visible/:plmn
// 从可见列表移除运营商（软删除，可重新添加）。
func (s *Server) handleRemoveVisible(c *gin.Context) {
	plmn := strings.TrimSpace(c.Param("plmn"))
	if plmn == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "plmn 不能为空"})
		return
	}
	if err := db.RemoveCarrierVisible(plmn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "移除失败: " + err.Error()})
		return
	}
	// 同时清除激活记录
	_ = db.ClearCarrierActivation(plmn)
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "已移除"})
}

// handleSearchCarrierIndex GET /api/carriers/search?q=
// 搜索 plmn-index 中的运营商（从本地 DB carrier_index 表）。
func (s *Server) handleSearchCarrierIndex(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusOK, []db.CarrierIndex{})
		return
	}
	results, err := db.SearchCarrierIndex(q, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "搜索失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}
