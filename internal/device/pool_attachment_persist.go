package device

import (
	"strings"

	"github.com/voorz/vohive/internal/config"
	"github.com/voorz/vohive/pkg/logger"
)

// deviceIMEIBackfillNeeded 判断是否需要把运行时学到的 IMEI 回填进配置。
// 仅当运行时已学到非空 IMEI、且与配置记录不同(含配置侧为空)时才回填。
// 空 IMEI 绝不触发,确保永不擦除配置里已有的身份。
func deviceIMEIBackfillNeeded(stored, current config.DeviceConfig) bool {
	if strings.TrimSpace(current.ModemIMEI) == "" {
		return false
	}
	return config.NormalizeIMEI(stored.ModemIMEI) != config.NormalizeIMEI(current.ModemIMEI)
}

// deviceUSBMetadataBackfillNeeded 判断是否需要把运行时从 sysfs 学到的 USB 元数据
// (manufacturer/product) 持久化到配置文件。仅当运行时值非空且与配置不同时才回填。
func deviceUSBMetadataBackfillNeeded(stored, current config.DeviceConfig) bool {
	if mfr := strings.TrimSpace(current.USBManufacturer); mfr != "" && mfr != strings.TrimSpace(stored.USBManufacturer) {
		return true
	}
	if prod := strings.TrimSpace(current.USBProduct); prod != "" && prod != strings.TrimSpace(stored.USBProduct) {
		return true
	}
	return false
}

// persistDeviceAttachmentsIfChanged 设备启动/恢复完成后,把运行时学到的身份信息回填进配置文件:
//   - IMEI: 一次性身份锚定
//   - USBManufacturer/USBProduct: 从 sysfs 学到的 USB 描述符元数据,持久化后设备离线仍可读取
//
// 绝不写回 control_device / interface / at_port / qmi_device / usb_path /
// audio_device 等易变路径——这些只活在内存,每次按 IMEI 现解析(见 spec 第 5 节)。
// 失败只记日志,不影响设备已成功启动这一事实。
func (p *Pool) persistDeviceAttachmentsIfChanged(cfg config.DeviceConfig) {
	if p == nil || strings.TrimSpace(cfg.ID) == "" {
		return
	}
	stored, err := config.GetDeviceByID(cfg.ID)
	if err != nil || stored == nil {
		return
	}
	path := config.GetConfigPath()
	if strings.TrimSpace(path) == "" {
		return
	}

	changed := false

	// 1. IMEI 回填
	if deviceIMEIBackfillNeeded(*stored, cfg) {
		imei := strings.TrimSpace(cfg.ModemIMEI)
		if err := config.UpdateDeviceIMEIInFile(path, map[string]string{cfg.ID: imei}); err != nil {
			logger.Warn("回填设备 IMEI 到配置文件失败", "device", cfg.ID, "err", err)
		} else {
			changed = true
			logger.Info("已回填设备 IMEI 到配置文件", "device", cfg.ID, "imei", imei)
		}
	}

	// 2. USB 元数据回填 (manufacturer/product)
	if deviceUSBMetadataBackfillNeeded(*stored, cfg) {
		meta := config.USBMetadataUpdate{
			Manufacturer: strings.TrimSpace(cfg.USBManufacturer),
			Product:      strings.TrimSpace(cfg.USBProduct),
		}
		if err := config.UpdateDeviceUSBMetadataInFile(path, map[string]config.USBMetadataUpdate{cfg.ID: meta}); err != nil {
			logger.Warn("回填设备 USB 元数据到配置文件失败", "device", cfg.ID, "err", err)
		} else {
			changed = true
			logger.Info("已回填设备 USB 元数据到配置文件",
				"device", cfg.ID,
				"manufacturer", meta.Manufacturer,
				"product", meta.Product)
		}
	}

	if !changed {
		return
	}

	if err := config.ReloadFromFile(); err != nil {
		logger.Warn("回填后重新加载配置文件失败", "device", cfg.ID, "err", err)
	}
}
