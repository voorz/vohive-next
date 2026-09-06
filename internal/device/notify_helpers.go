package device

import "github.com/voorz/vohive/pkg/notify"

// broadcastDeviceOnline 设备已连接气泡通知（已添加设备）。
func broadcastDeviceOnline(deviceID, deviceName string) {
	notify.GlobalNotificationBroadcaster.Broadcast(notify.FrontendNotification{
		Level:      "low",
		Event:      "device_online",
		Title:      "设备已连接，VoWiFi实例恢复中",
		Body:       deviceID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
	})
}

// broadcastDeviceOffline 设备离线气泡通知（已添加设备）。
func broadcastDeviceOffline(deviceID, deviceName string) {
	notify.GlobalNotificationBroadcaster.Broadcast(notify.FrontendNotification{
		Level:      "low",
		Event:      "device_offline",
		Title:      "设备已断开，VoWiFi实例即将停止",
		Body:       deviceID,
		DeviceID:   deviceID,
		DeviceName: deviceName,
	})
}
