package device

// imei_generator.go - PC/SC 读卡器设备的虚拟 IMEI 生成器
// 基于 TAC 数据库生成可通过在线验证的合法 IMEI

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/voorz/vohive/pkg/logger"
)

// TACEntry TAC 数据库条目
type TACEntry struct {
	TAC   string // 8位 TAC 码
	Brand string // 品牌
	Model string // 型号
}

// TAC 数据库 - 包含品牌、型号与前 8 位 TAC 的映射
var tacDatabase = []TACEntry{
	{TAC: "35743593", Brand: "Apple", Model: "iPhone 16 Pro Max"},
	{TAC: "35000977", Brand: "Samsung", Model: "Galaxy S26 Ultra"},
}

// 预设 IMEI 池 - 基于 TAC 数据库中机型的真实 IMEI 示例
var predefinedIMEIs = map[string][]string{
	// APPLE iPhone 16 Pro Max (TAC: 35743593)
	"35743593": {
		"357435938512735", "357435939801178", "357435936018131",
		"357435937139696", "357435938996516", "357435930833956",
		"357435939177397", "357435931355587", "357435939561921",
		"357435933041565",
	},
	// SAMSUNG Galaxy S26 Ultra (TAC: 35000977)
	"35000977": {
		"350009773171639", "350009777976587", "350009773248999",
		"350009776161900", "350009776539709", "350009772715725",
		"350009779314886", "350009772229206", "350009772828569",
		"350009776351949",
	},
}

// imeiPoolMutex 保护 IMEI 池的并发访问
var imeiPoolMutex sync.Mutex

// 分配给各 TAC 的 IMEI 计数器
var imeiCounters = map[string]int{}

// luhnCheckDigit 使用 Luhn 算法计算校验位
// 输入：14位 IMEI 前缀（不含校验位）
// 返回：1位校验位（0-9）
func luhnCheckDigit(prefix string) byte {
	if len(prefix) != 14 {
		return 0
	}

	sum := 0
	for i, ch := range prefix {
		digit := int(ch - '0')
		// Luhn 算法：从右数偶数位（0-based 索引的奇数位）的数字翻倍
		if i%2 == 1 {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}

	checkDigit := (10 - (sum % 10)) % 10
	return byte(checkDigit) + '0'
}

// generateIMEI 从 TAC 生成完整的 15 位 IMEI
// serial: 6 位序列号（十六进制字符串）
// 返回：15 位 IMEI（8位 TAC + 6位序列号 + 1位校验位）
func generateIMEI(tac, serial string) string {
	if len(tac) != 8 || len(serial) != 6 {
		return ""
	}

	prefix := tac + serial
	checkDigit := luhnCheckDigit(prefix)
	return prefix + string(checkDigit)
}

// generateSerial 从设备 ID 生成确定性的 6 位序列号
// 使用 SHA256 哈希确保相同设备 ID 总是生成相同序列号
// 将十六进制字符映射为数字（A-F → 0-5），确保 IMEI 全为数字
func generateSerial(deviceID string) string {
	hash := sha256.Sum256([]byte(deviceID))
	hexStr := hex.EncodeToString(hash[:])
	if len(hexStr) > 6 {
		hexStr = hexStr[:6]
	}

	// 十六进制转数字：0-9 保持不变，A-F 转为 0-5
	serial := strings.ToUpper(hexStr)
	result := make([]byte, len(serial))
	for i, ch := range serial {
		if ch >= '0' && ch <= '9' {
			result[i] = byte(ch)
		} else if ch >= 'A' && ch <= 'F' {
			// A=10→0, B=11→1, ..., F=15→5
			result[i] = byte(ch - 'A' + '0')
		} else {
			result[i] = '0'
		}
	}
	return string(result)
}

// GenerateIMEIForDevice 为指定设备生成确定性 IMEI
// 算法：
// 1. 从 TAC 数据库中选择 TAC（基于设备 ID 哈希）
// 2. 尝试从预设 IMEI 池中分配（未使用过的 IMEI）
// 3. 若池已耗尽，基于 TAC + 设备 ID 生成新 IMEI
func GenerateIMEIForDevice(deviceID string) string {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ""
	}

	imeiPoolMutex.Lock()
	defer imeiPoolMutex.Unlock()

	// 使用设备 ID 哈希选择 TAC（确保同一设备总是选择相同 TAC）
	hash := sha256.Sum256([]byte(deviceID))
	tacIndex := int(hash[0]) % len(tacDatabase)
	tacEntry := tacDatabase[tacIndex]
	tac := tacEntry.TAC

	// 尝试从预设池分配 IMEI
	if pool, ok := predefinedIMEIs[tac]; ok {
		if counter, ok := imeiCounters[tac]; ok && counter < len(pool) {
			imei := pool[counter]
			imeiCounters[tac] = counter + 1
			logger.Debug(fmt.Sprintf("从预设 IMEI 池分配: %s (device: %s, model: %s %s)",
				imei, deviceID, tacEntry.Brand, tacEntry.Model))
			return imei
		}
		// 池已耗尽，记录日志并生成新 IMEI
		logger.Debug(fmt.Sprintf("TAC %s 预设 IMEI 汆已耗尽，生成新 IMEI (device: %s)",
			tac, deviceID))
	}

	// 生成新 IMEI（基于 TAC + 设备 ID 序列号）
	serial := generateSerial(deviceID)
	imei := generateIMEI(tac, serial)

	logger.Debug(fmt.Sprintf("生成新 IMEI: %s (device: %s, model: %s %s, TAC: %s, serial: %s)",
		imei, deviceID, tacEntry.Brand, tacEntry.Model, tac, serial))

	return imei
}