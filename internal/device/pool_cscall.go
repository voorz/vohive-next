package device

import (
	"fmt"

	"github.com/voorz/vohive/internal/backend"
	"github.com/voorz/vohive/internal/cscall"
	"github.com/voorz/vohive/internal/sipgw"
	"github.com/voorz/vohive/pkg/logger"
)

func newCSCallManagerForWorker(w *Worker, r *sipgw.Registrar) *cscall.Manager {
	if w == nil || r == nil {
		return nil
	}
	switch {
	case w.Backend != nil && w.Backend.Mode() == backend.BackendAT && w.Modem != nil:
		// AT 模式 CS 通话依赖 ALSA AudioDevice 做 PCM↔RTP 桥接
		if w.Config.AudioDevice == "" {
			logger.Debug(fmt.Sprintf("[%s] 跳过 CS 域语音桥接：AT 模式缺少 AudioDevice", w.ID))
			return nil
		}
		return cscall.NewManagerWithController(w.ID, w.Config.AudioDevice, cscall.NewATController(w.Modem), r)
	case w.Backend != nil && w.Backend.Mode() == backend.BackendQMI && w.QMICore != nil:
		// QMI 模式 CS 通话通过 QMI Voice 服务拨号，AudioDevice 可选（为空时仅无法桥接音频）
		mgr := cscall.NewManagerWithController(w.ID, w.Config.AudioDevice, cscall.NewQMIController(w.QMICore), r)
		// QMI 后端需要通过 AT+QPCMV=1,2 开启模组的 PCM over USB 音频通道
		if w.Modem != nil {
			mgr.SetATHelper(w.Modem)
		}
		return mgr
	default:
		logger.Debug(fmt.Sprintf("[%s] 跳过 CS 域语音桥接：缺少可用控制面", w.ID),
			"backend", workerBackendMode(w),
			"audio_device", w.Config.AudioDevice,
			"has_modem", w.Modem != nil,
			"has_qmi_core", w.QMICore != nil)
		return nil
	}
}
