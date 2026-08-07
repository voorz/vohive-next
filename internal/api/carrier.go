package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	carrierconfig "github.com/voorz/vohive/internal/carrier"
	"github.com/voorz/vohive/internal/db"
	"github.com/voorz/vowifi-core/profiles"
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
		// 解析 key：可能是 PLMN（234-30）或 PLMN__brand（234-30__BT Mobile）
		plmnKey := v.PLMN
		brandSuffix := ""
		if idx := strings.Index(plmnKey, "__"); idx > 0 {
			brandSuffix = plmnKey[idx+2:]
			plmnKey = plmnKey[:idx]
		}

		// 从 carrier_index 查运营商元数据
		carrierIdx, err := db.GetCarrierIndex(plmnKey)
		if err != nil {
			logger.Warn("查询运营商索引失败", "plmn", plmnKey, "err", err)
			continue
		}

		item := carrierconfig.ListItem{
			Key: v.PLMN,
		}

		if carrierIdx != nil {
			item.MCC = carrierIdx.MCC
			item.MNC = carrierIdx.MNC
			// 从 raw_json 提取运营商名称
			name := carrierIdx.PLMN
			var raw struct {
				Operators []struct {
					Brand    string `json:"brand"`
					Operator string `json:"operator"`
				} `json:"operators"`
			}
			if json.Unmarshal([]byte(carrierIdx.RawJSON), &raw) == nil && len(raw.Operators) > 0 {
				if raw.Operators[0].Brand != "" {
					name = raw.Operators[0].Brand
				} else if raw.Operators[0].Operator != "" {
					name = raw.Operators[0].Operator
				}
			}
			// 如果有 brand 后缀（子品牌），用 brand 作为名称
			if brandSuffix != "" {
				name = brandSuffix
			}
			item.Name = name
		} else {
			// index 中没有，尝试从 PLMN key 解析 MCC/MNC
			parts := strings.SplitN(plmnKey, "-", 2)
			if len(parts) == 2 {
				item.MCC = parts[0]
				item.MNC = parts[1]
			}
			if brandSuffix != "" {
				item.Name = brandSuffix
			} else {
				item.Name = plmnKey
			}
		}

		out = append(out, item)
	}

	c.JSON(http.StatusOK, out)
}

// handleGetCarrier GET /api/carrier/:mcc/:mnc?brand=
// 从 carrier_index 获取元数据，从 carrier_templates 获取用户配置，从 profiles 获取系统默认。
func (s *Server) handleGetCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	if mcc == "" || mnc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "mcc 和 mnc 不能为空"})
		return
	}
	key := makeKey(mcc, mnc, brand)

	// 系统默认
	var sysDefault *profiles.CarrierProfile
	if p, err := profiles.LookupWithSPN(mcc, mnc, brand); err == nil && p != nil {
		sysDefault = p
	}

	// 从 carrier_index 获取元数据
	name := ""
	plmnKey := key
	if idx := strings.Index(plmnKey, "__"); idx > 0 {
		plmnKey = plmnKey[:idx]
	}
	if carrierIdx, _ := db.GetCarrierIndex(plmnKey); carrierIdx != nil {
		var raw struct {
			Operators []struct {
				Brand    string `json:"brand"`
				Operator string `json:"operator"`
			} `json:"operators"`
		}
		if json.Unmarshal([]byte(carrierIdx.RawJSON), &raw) == nil && len(raw.Operators) > 0 {
			if raw.Operators[0].Brand != "" {
				name = raw.Operators[0].Brand
			} else if raw.Operators[0].Operator != "" {
				name = raw.Operators[0].Operator
			}
		}
	}
	if brand != "" {
		name = brand
	}
	if name == "" && sysDefault != nil {
		name = sysDefault.Name
	}
	if name == "" {
		name = plmnKey
	}

	// 用户配置
	tpl, _ := db.GetCarrierTemplateByKey(key)
	var userConfig *profiles.CarrierProfile
	if tpl != nil && tpl.ProfileJSON != "" {
		var p profiles.CarrierProfile
		if err := json.Unmarshal([]byte(tpl.ProfileJSON), &p); err == nil {
			userConfig = &p
		}
	}

	// 激活状态
	act, _ := db.GetCarrierActivation(key)
	active := act != nil && act.TemplateID != nil

	// ePDG 地址、设备信息
	ikeAddr := ""
	deviceIMSTAC := 0
	deviceIMSCellID := 0
	if userConfig != nil {
		ikeAddr = userConfig.IKE.Addr
		deviceIMSTAC = userConfig.Device.IMSTAC
		deviceIMSCellID = userConfig.Device.IMSCellID
	} else if sysDefault != nil {
		ikeAddr = sysDefault.IKE.Addr
		deviceIMSTAC = sysDefault.Device.IMSTAC
		deviceIMSCellID = sysDefault.Device.IMSCellID
	}

	d := &carrierconfig.Detail{
		Key:             key,
		MCC:             mcc,
		MNC:             mnc,
		Name:            name,
		IKEAddr:         ikeAddr,
		DeviceIMSTAC:    deviceIMSTAC,
		DeviceIMSCellID: deviceIMSCellID,
		SystemDefault:   sysDefault,
		UserConfig:      userConfig,
		Active:          active,
	}
	c.JSON(http.StatusOK, d)
}

// handleSaveCarrierConfig PUT /api/carrier/:mcc/:mnc?brand=
// 保存用户配置到 carrier_templates + carrier_activation。
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
	if payload.Config == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "config 不能为空"})
		return
	}

	// 注入 MCC/MNC/Name
	payload.Config.MCC = mcc
	payload.Config.MNC = mnc
	payload.Config.Name = strings.TrimSpace(payload.Name)
	if payload.Config.IKE.Addr == "" {
		payload.Config.IKE.Addr = strings.TrimSpace(payload.IKEAddr)
	}
	if payload.Config.Device.IMSTAC == 0 {
		payload.Config.Device.IMSTAC = payload.DeviceIMSTAC
	}
	if payload.Config.Device.IMSCellID == 0 {
		payload.Config.Device.IMSCellID = payload.DeviceIMSCellID
	}

	jsonBytes, err := json.Marshal(payload.Config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "序列化配置失败: " + err.Error()})
		return
	}

	// upsert carrier_templates
	tpl, _ := db.GetCarrierTemplateByKey(key)
	if tpl != nil {
		tpl.Name = strings.TrimSpace(payload.Name)
		tpl.ProfileJSON = string(jsonBytes)
		if err := db.UpdateCarrierTemplate(tpl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "更新模板失败: " + err.Error()})
			return
		}
	} else {
		tpl = &db.CarrierTemplate{
			Key:         key,
			Name:        strings.TrimSpace(payload.Name),
			ProfileJSON: string(jsonBytes),
		}
		if err := db.CreateCarrierTemplate(tpl); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "创建模板失败: " + err.Error()})
			return
		}
	}

	// 激活状态
	var templateID *int64
	if payload.Active {
		templateID = &tpl.ID
	}
	_ = db.SetCarrierActivation(key, templateID)

	// 热更新
	if payload.Active {
		profiles.SetUserOverrideByKey(key, payload.Config)
		logger.Info("运营商配置已热更新", "key", key, "event", "CARRIER_CONFIG_HOT_RELOAD")
	} else {
		profiles.SetUserOverrideByKey(key, nil)
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "配置已保存"})
}

// handleDeleteCarrierConfig DELETE /api/carrier/:mcc/:mnc/config?brand=
// 删除 carrier_templates 中的用户模板，清除激活记录。
func (s *Server) handleDeleteCarrierConfig(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)

	tpl, _ := db.GetCarrierTemplateByKey(key)
	if tpl != nil {
		_ = db.DeleteCarrierTemplate(tpl.ID)
	}
	_ = db.ClearCarrierActivation(key)
	profiles.SetUserOverrideByKey(key, nil)

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "用户配置已删除"})
}

// handleActivateCarrier POST /api/carrier/:mcc/:mnc/activate?brand=
// 激活用户模板。
func (s *Server) handleActivateCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)

	tpl, _ := db.GetCarrierTemplateByKey(key)
	if tpl == nil || tpl.ProfileJSON == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "无用户模板可激活"})
		return
	}
	_ = db.SetCarrierActivation(key, &tpl.ID)

	var p profiles.CarrierProfile
	if err := json.Unmarshal([]byte(tpl.ProfileJSON), &p); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "解析模板失败: " + err.Error()})
		return
	}
	profiles.SetUserOverrideByKey(key, &p)
	logger.Info("运营商配置已激活", "key", key, "event", "CARRIER_CONFIG_ACTIVATED")

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商配置已激活"})
}

// handleDeactivateCarrier POST /api/carrier/:mcc/:mnc/deactivate?brand=
// 禁用用户模板，回退到系统默认。
func (s *Server) handleDeactivateCarrier(c *gin.Context) {
	mcc := strings.TrimSpace(c.Param("mcc"))
	mnc := strings.TrimSpace(c.Param("mnc"))
	brand := strings.TrimSpace(c.Query("brand"))
	key := makeKey(mcc, mnc, brand)

	_ = db.ClearCarrierActivation(key)
	profiles.SetUserOverrideByKey(key, nil)
	logger.Info("运营商配置已禁用", "key", key, "event", "CARRIER_CONFIG_DEACTIVATED")

	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "运营商配置已禁用"})
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
