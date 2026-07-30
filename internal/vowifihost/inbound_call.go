package vowifihost

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/voorz/sipgo/sip"
	"github.com/voorz/vowifi-core/runtimehost"

	"github.com/voorz/vohive/pkg/logger"
)

// handleInboundCall forwards an incoming VoWiFi INVITE to Linphone via
// the sipgw.Registrar. It builds a new INVITE with the IMS caller's
// SDP, sends it to the Linphone client, and returns the final response
// (status code + SDP from Linphone) to be sent back to the IMS network.
//
// TODO: RTP relay media bridge — currently SDP is passed through directly
// (IMS ↔ Linphone media direct connect). When Linphone is behind NAT or
// the IMS tunnel IP is unreachable from Linphone, an RTP relay is needed
// to bridge media between the two networks.
func (m *Manager) handleInboundCall(ctx context.Context, req runtimehost.InboundCallRequest) (runtimehost.InboundCallResponse, error) {
	if m == nil || m.sipRegistrar == nil {
		logger.Info("VoWiFi 来电（sipgw 未配置，回复 486）",
			"event", "VOWIFI_INBOUND_CALL_NO_SIPGW",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"caller", req.CallerURI)
		return runtimehost.InboundCallResponse{StatusCode: 486, Reason: "Busy Here"}, nil
	}

	// Find the Linphone user registered for this device
	user := m.sipRegistrar.GetUserByDevice(req.DeviceID)
	if user == nil {
		logger.Info("VoWiFi 来电（Linphone 未在线，回复 480）",
			"event", "VOWIFI_INBOUND_CALL_USER_OFFLINE",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"caller", req.CallerURI)
		return runtimehost.InboundCallResponse{StatusCode: 480, Reason: "Temporarily Unavailable"}, nil
	}

	// Build INVITE to Linphone
	linphoneURI := sip.Uri{
		User: user.Username,
		Host: user.ContactAddr.IP.String(),
		Port: user.ContactAddr.Port,
	}
	inviteReq := sip.NewRequest(sip.INVITE, linphoneURI)
	inviteReq.SetDestination(user.ContactAddr.String())
	if user.Transport != "" {
		inviteReq.SetTransport(user.Transport)
	}

	// From = IMS caller
	inviteReq.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<%s>;tag=%s", req.CallerURI, sip.GenerateTagN(8))))
	// To = Linphone user
	inviteReq.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<sip:%s@%s>", user.Username, user.ContactAddr.IP.String())))
	inviteReq.AppendHeader(sip.NewHeader("Call-ID", req.CallID))
	inviteReq.AppendHeader(sip.NewHeader("CSeq", "1 INVITE"))
	inviteReq.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<sip:%s>", req.CallerURI)))
	inviteReq.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
	// Forward IMS SDP to Linphone (direct passthrough, no RTP relay yet)
	if len(req.RemoteSDP) > 0 {
		inviteReq.SetBody(append([]byte(nil), req.RemoteSDP...))
	}

	logger.Info("VoWiFi 来电转发到 Linphone",
		"event", "VOWIFI_INBOUND_CALL_FORWARD",
		"device", req.DeviceID,
		"call_id", req.CallID,
		"caller", req.CallerURI,
		"linphone_user", user.Username,
		"linphone_addr", user.ContactAddr.String())

	// Send INVITE with timeout
	inviteCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx, err := m.sipRegistrar.GetClient().TransactionRequest(inviteCtx, inviteReq)
	if err != nil {
		logger.Warn("VoWiFi 来电 INVITE 发送到 Linphone 失败",
			"event", "VOWIFI_INBOUND_CALL_SEND_FAIL",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"error", err.Error())
		return runtimehost.InboundCallResponse{StatusCode: 503, Reason: "Linphone INVITE failed"}, err
	}

	// Wait for final response
	var finalResp *sip.Response
	for {
		select {
		case <-inviteCtx.Done():
			// Send CANCEL if context expired
			cancelReq := sip.NewRequest(sip.CANCEL, linphoneURI)
			sip.CopyHeaders("From", inviteReq, cancelReq)
			sip.CopyHeaders("To", inviteReq, cancelReq)
			sip.CopyHeaders("Call-ID", inviteReq, cancelReq)
			sip.CopyHeaders("Via", inviteReq, cancelReq)
			sip.CopyHeaders("Route", inviteReq, cancelReq)
			_ = m.sipRegistrar.GetClient().WriteRequest(cancelReq)
			return runtimehost.InboundCallResponse{StatusCode: 487, Reason: "Request Terminated"}, nil
		case resp, ok := <-tx.Responses():
			if !ok {
				return runtimehost.InboundCallResponse{StatusCode: 503, Reason: "No response from Linphone"}, nil
			}
			if resp.StatusCode >= 100 && resp.StatusCode < 200 {
				// Provisional response — keep waiting
				continue
			}
			finalResp = resp
		}
		if finalResp != nil {
			break
		}
	}

	// Handle final response
	reason := strings.TrimSpace(finalResp.Reason)
	if reason == "" {
		reason = "OK"
	}

	result := runtimehost.InboundCallResponse{
		StatusCode: finalResp.StatusCode,
		Reason:     reason,
		SDP:        append([]byte(nil), finalResp.Body()...),
	}

	if finalResp.StatusCode >= 200 && finalResp.StatusCode < 300 {
		logger.Info("VoWiFi 来电 Linphone 接听",
			"event", "VOWIFI_INBOUND_CALL_ANSWERED",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"status", finalResp.StatusCode)
	} else {
		logger.Info("VoWiFi 来电 Linphone 拒绝",
			"event", "VOWIFI_INBOUND_CALL_REJECTED",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"status", finalResp.StatusCode,
			"reason", reason)
	}

	return result, nil
}
