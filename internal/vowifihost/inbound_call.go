package vowifihost

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/voorz/sipgo/sip"
	"github.com/voorz/vowifi-core/runtimehost"
	"github.com/voorz/vowifi-core/runtimehost/voicehost"

	"github.com/voorz/vohive/pkg/logger"
)

// handleInboundCall forwards an incoming VoWiFi INVITE to Linphone via
// the sipgw.Registrar. It creates an RTP relay to bridge media between
// the IMS tunnel and the Linphone client, rewrites the SDP so both
// sides send RTP to the relay, and returns the final response to the
// IMS network.
//
// Provisional responses (180 Ringing etc.) from Linphone are forwarded
// to the IMS network via req.Respond before the final response is returned.
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
		if m.callEventPub != nil {
			m.callEventPub.OnCallEnded(req.DeviceID, req.CallID)
		}
		return runtimehost.InboundCallResponse{StatusCode: 480, Reason: "Temporarily Unavailable"}, nil
	}

	// 发布来电事件
	if m.callEventPub != nil {
		m.callEventPub.OnInboundInvite(req.DeviceID, req.CallID, req.CallerURI)
	}

	// --- RTP relay media bridge ---
	// Parse IMS SDP and create a relay that bridges media between
	// the IMS tunnel and Linphone. Both sides advertise the sipgw
	// ExternalIP (router LAN IP) so both can reach the relay.
	var relay *voicehost.RTPRelaySession
	relayClosed := false
	if len(req.RemoteSDP) > 0 {
		imsSDP, err := voicehost.ParseSDP(req.RemoteSDP)
		if err != nil {
			logger.Warn("VoWiFi 来电 IMS SDP 解析失败，回退直通",
				"event", "VOWIFI_INBOUND_CALL_SDP_PARSE_FAIL",
				"device", req.DeviceID,
				"call_id", req.CallID,
				"error", err.Error())
		} else {
			externalIP := m.sipRegistrar.GetExternalIP()
			if externalIP == "" {
				externalIP = "127.0.0.1"
			}
			relayCfg := voicehost.RTPRelayConfig{
				ClientListenIP:    "0.0.0.0",
				ClientAdvertiseIP: externalIP,
				IMSListenIP:       "0.0.0.0",
				IMSAdvertiseIP:     externalIP,
			}
			relay, err = voicehost.NewRTPRelaySessionForIMSRemote(ctx, relayCfg, imsSDP)
			if err != nil {
				logger.Warn("VoWiFi 来电 RTP relay 创建失败，回退直通",
					"event", "VOWIFI_INBOUND_CALL_RELAY_FAIL",
					"device", req.DeviceID,
					"call_id", req.CallID,
					"error", err.Error())
				relay = nil
			} else {
				logger.Info("VoWiFi 来电 RTP relay 已创建",
					"event", "VOWIFI_INBOUND_CALL_RELAY_CREATED",
					"device", req.DeviceID,
					"call_id", req.CallID,
					"client_ep", relay.ClientEndpoint().MediaPort,
					"ims_ep", relay.IMSEndpoint().MediaPort)
			}
		}
	}
	// Ensure relay is closed on error or non-200 response
	defer func() {
		if relay != nil && !relayClosed {
			_ = relay.Close()
		}
	}()

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

	localTag := sip.GenerateTagN(8)

	// From = IMS caller
	inviteReq.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<%s>;tag=%s", req.CallerURI, localTag)))
	// To = Linphone user
	inviteReq.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<sip:%s@%s>", user.Username, user.ContactAddr.IP.String())))
	inviteReq.AppendHeader(sip.NewHeader("Call-ID", req.CallID))
	inviteReq.AppendHeader(sip.NewHeader("CSeq", "1 INVITE"))
	inviteReq.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<sip:%s>", req.CallerURI)))
	inviteReq.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))

	// Set INVITE body: relay-rewritten SDP if relay is active,
	// otherwise passthrough IMS SDP directly.
	if relay != nil {
		clientEP := relay.ClientEndpoint()
		inviteBody := voicehost.RewriteSDPMediaEndpoint(req.RemoteSDP, clientEP)
		inviteReq.SetBody(inviteBody)
	} else if len(req.RemoteSDP) > 0 {
		inviteReq.SetBody(append([]byte(nil), req.RemoteSDP...))
	}

	logger.Info("VoWiFi 来电转发到 Linphone",
		"event", "VOWIFI_INBOUND_CALL_FORWARD",
		"device", req.DeviceID,
		"call_id", req.CallID,
		"caller", req.CallerURI,
		"linphone_user", user.Username,
		"linphone_addr", user.ContactAddr.String(),
		"relay", relay != nil)

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

	// Wait for final response, forwarding provisional responses to IMS
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
				// Provisional response — forward to IMS via Respond callback
				if req.Respond != nil {
					reason := strings.TrimSpace(resp.Reason)
					if reason == "" {
						reason = "Ringing"
					}
					if err := req.Respond(resp.StatusCode, reason, nil); err != nil {
						logger.Warn("VoWiFi 来电临时响应转发到 IMS 失败",
							"event", "VOWIFI_INBOUND_CALL_PROVISIONAL_FAIL",
							"device", req.DeviceID,
							"call_id", req.CallID,
							"status", resp.StatusCode,
							"error", err.Error())
					} else {
						logger.Info("VoWiFi 来电临时响应转发到 IMS",
							"event", "VOWIFI_INBOUND_CALL_PROVISIONAL",
							"device", req.DeviceID,
							"call_id", req.CallID,
							"status", resp.StatusCode)
					}
				}
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
	}

	if finalResp.StatusCode >= 200 && finalResp.StatusCode < 300 {
		// Call answered — set Linphone as relay client remote, store
		// dialog info and relay for BYE/CANCEL forwarding.
		if m.callEventPub != nil {
			m.callEventPub.OnCallConnected(req.DeviceID, req.CallID)
		}
		if relay != nil && len(finalResp.Body()) > 0 {
			linphoneSDP, err := voicehost.ParseSDP(finalResp.Body())
			if err != nil {
				logger.Warn("VoWiFi 来电 Linphone SDP 解析失败",
					"event", "VOWIFI_INBOUND_CALL_LINPHONE_SDP_FAIL",
					"device", req.DeviceID,
					"call_id", req.CallID,
					"error", err.Error())
			} else {
				if err := relay.SetClientRemote(linphoneSDP); err != nil {
					logger.Warn("VoWiFi 来电 relay SetClientRemote 失败",
						"event", "VOWIFI_INBOUND_CALL_RELAY_SETCLIENT_FAIL",
						"device", req.DeviceID,
						"call_id", req.CallID,
						"error", err.Error())
				}
			}
			// Return relay IMS endpoint SDP to IMS
			result.SDP = voicehost.BuildSDPAnswer(relay.IMSEndpoint())
		} else {
			// No relay — passthrough Linphone SDP
			result.SDP = append([]byte(nil), finalResp.Body()...)
		}

		// Extract dialog parameters from Linphone 200 OK and store
		remoteTag := ""
		if to := finalResp.To(); to != nil {
			if tag, ok := to.Params.Get("tag"); ok {
				remoteTag = strings.TrimSpace(tag)
			}
		}
		remoteContact := ""
		if contact := finalResp.Contact(); contact != nil {
			remoteContact = strings.TrimSpace(contact.Address.String())
		}
		var routeSet []string
		for _, rr := range finalResp.GetHeaders("Record-Route") {
			routeSet = append(routeSet, strings.TrimSpace(rr.Value()))
		}

		dialog := &inboundDialogInfo{
			CallID:     req.CallID,
			DeviceID:   req.DeviceID,
			RemoteTag:  remoteTag,
			LocalTag:   localTag,
			ContactURI: remoteContact,
			RouteSet:   routeSet,
			CSeq:       1,
		}
		m.storeInboundDialog(req.CallID, dialog)

		// Store relay for lifecycle management
		if relay != nil {
			m.storeInboundRelay(req.CallID, relay)
			relayClosed = true // relay ownership transferred to Manager
		}

		logger.Info("VoWiFi 来电 Linphone 接听",
			"event", "VOWIFI_INBOUND_CALL_ANSWERED",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"status", finalResp.StatusCode,
			"remote_tag", remoteTag,
			"relay", relay != nil)
	} else {
		logger.Info("VoWiFi 来电 Linphone 拒绝",
			"event", "VOWIFI_INBOUND_CALL_REJECTED",
			"device", req.DeviceID,
			"call_id", req.CallID,
			"status", finalResp.StatusCode,
			"reason", reason)
		// relay will be closed by defer
	}

	return result, nil
}

// handleInboundBye forwards an IMS BYE to Linphone, terminating the call.
// The dialog info is retrieved from the inbound dialogs map (stored during
// handleInboundCall) and used to build the BYE request to Linphone.
// The RTP relay is also closed.
func (m *Manager) handleInboundBye(ctx context.Context, deviceID, callID string) error {
	// Close relay first to stop media forwarding
	m.closeInboundRelay(callID)

	dialog, ok := m.loadInboundDialog(callID)
	if !ok {
		logger.Info("VoWiFi 入站 BYE（对话未找到，可能已清理）",
			"event", "VOWIFI_INBOUND_BYE_NO_DIALOG",
			"device", deviceID,
			"call_id", callID)
		return nil
	}
	defer m.deleteInboundDialog(callID)

	// Build BYE to Linphone
	byeURI, err := parseLinphoneURI(dialog.ContactURI, dialog.DeviceID)
	if err != nil {
		return fmt.Errorf("parse Linphone URI: %w", err)
	}

	byeReq := sip.NewRequest(sip.BYE, byeURI)
	byeReq.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<sip:%s>;tag=%s", dialog.DeviceID, dialog.LocalTag)))
	byeReq.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<sip:%s>;tag=%s", dialog.DeviceID, dialog.RemoteTag)))
	byeReq.AppendHeader(sip.NewHeader("Call-ID", dialog.CallID))
	byeReq.AppendHeader(sip.NewHeader("CSeq", strconv.Itoa(dialog.CSeq+1)+" BYE"))
	byeReq.AppendHeader(sip.NewHeader("Via", sip.GenerateBranch()))
	for _, route := range dialog.RouteSet {
		byeReq.AppendHeader(sip.NewHeader("Route", route))
	}

	logger.Info("VoWiFi 入站 BYE 转发到 Linphone",
		"event", "VOWIFI_INBOUND_BYE_FORWARD",
		"device", deviceID,
		"call_id", callID)

	byeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := m.sipRegistrar.GetClient().TransactionRequest(byeCtx, byeReq)
	if err != nil {
		return fmt.Errorf("send BYE to Linphone: %w", err)
	}

	// Wait for 200 OK
	for {
		resp, ok := <-tx.Responses()
		if !ok {
			break
		}
		if resp.StatusCode >= 200 {
			logger.Info("VoWiFi 入站 BYE Linphone 响应",
				"event", "VOWIFI_INBOUND_BYE_RESPONSE",
				"device", deviceID,
				"call_id", callID,
				"status", resp.StatusCode)
			break
		}
	}
	return nil
}

// handleInboundCancel forwards an IMS CANCEL to Linphone, cancelling the
// ringing call before it's answered. The RTP relay is also closed.
func (m *Manager) handleInboundCancel(ctx context.Context, deviceID, callID string) error {
	// Close relay first to stop media forwarding
	m.closeInboundRelay(callID)

	dialog, ok := m.loadInboundDialog(callID)
	if !ok {
		logger.Info("VoWiFi 入站 CANCEL（对话未找到，可能已清理）",
			"event", "VOWIFI_INBOUND_CANCEL_NO_DIALOG",
			"device", deviceID,
			"call_id", callID)
		return nil
	}
	defer m.deleteInboundDialog(callID)

	// Build CANCEL to Linphone (same CSeq as INVITE)
	byeURI, err := parseLinphoneURI(dialog.ContactURI, dialog.DeviceID)
	if err != nil {
		return fmt.Errorf("parse Linphone URI: %w", err)
	}

	cancelReq := sip.NewRequest(sip.CANCEL, byeURI)
	cancelReq.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<sip:%s>;tag=%s", dialog.DeviceID, dialog.LocalTag)))
	cancelReq.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<sip:%s>", dialog.DeviceID)))
	cancelReq.AppendHeader(sip.NewHeader("Call-ID", dialog.CallID))
	cancelReq.AppendHeader(sip.NewHeader("CSeq", strconv.Itoa(dialog.CSeq)+" CANCEL"))
	cancelReq.AppendHeader(sip.NewHeader("Via", sip.GenerateBranch()))
	for _, route := range dialog.RouteSet {
		cancelReq.AppendHeader(sip.NewHeader("Route", route))
	}

	logger.Info("VoWiFi 入站 CANCEL 转发到 Linphone",
		"event", "VOWIFI_INBOUND_CANCEL_FORWARD",
		"device", deviceID,
		"call_id", callID)

	_ = m.sipRegistrar.GetClient().WriteRequest(cancelReq)
	return nil
}

// parseLinphoneURI parses a Linphone Contact URI into a sip.Uri. If
// parsing fails, it falls back to a URI derived from the device's
// registered user contact address.
func parseLinphoneURI(contactURI, deviceID string) (sip.Uri, error) {
	if strings.TrimSpace(contactURI) != "" {
		var uri sip.Uri
		if err := sip.ParseUri(contactURI, &uri); err == nil {
			return uri, nil
		}
	}
	// Fallback: empty URI — caller should set destination separately.
	// This path is unlikely to succeed but prevents a hard crash.
	return sip.Uri{}, fmt.Errorf("cannot parse Linphone contact URI for device %s", deviceID)
}
