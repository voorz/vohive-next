package vowifihost

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/voorz/sipgo/sip"
	"github.com/voorz/ims-go/ims"

	"github.com/voorz/vohive/pkg/logger"
)

// deviceIncomingCallHandler 实现 ims.IncomingCallHandler。
// 将入站 VoWiFi INVITE 转发到 Linphone（B2BUA），桥接媒体。
type deviceIncomingCallHandler struct {
	manager  *Manager
	deviceID string
}

func (h *deviceIncomingCallHandler) HandleIncomingCall(ctx context.Context, req ims.IncomingCallRequest) ims.IncomingCallResponse {
	m := h.manager
	deviceID := h.deviceID

	if m == nil || m.sipRegistrar == nil {
		logger.Info("VoWiFi 来电（sipgw 未配置，回复 486）",
			"event", "VOWIFI_INBOUND_CALL_NO_SIPGW",
			"device", deviceID,
			"call_id", req.CallID,
			"caller", req.From)
		return ims.IncomingCallResponse{Accept: false, StatusCode: 486, Reason: "Busy Here"}
	}

	// Find the Linphone user registered for this device
	user := m.sipRegistrar.GetUserByDevice(deviceID)
	if user == nil {
		logger.Info("VoWiFi 来电（Linphone 未在线，回复 480）",
			"event", "VOWIFI_INBOUND_CALL_USER_OFFLINE",
			"device", deviceID,
			"call_id", req.CallID,
			"caller", req.From)
		if m.callEventPub != nil {
			m.callEventPub.OnCallEnded(deviceID, req.CallID)
		}
		return ims.IncomingCallResponse{Accept: false, StatusCode: 480, Reason: "Temporarily Unavailable"}
	}

	// 发布来电事件
	if m.callEventPub != nil {
		m.callEventPub.OnInboundInvite(deviceID, req.CallID, req.From)
	}

	// --- RTP relay media bridge ---
	// Parse IMS SDP and create a relay that bridges media between
	// the IMS network and Linphone.
	var relay *rtpRelay
	relayClosed := false
	if len(req.RemoteSDP) > 0 {
		imsEP, err := parseSDPEndpoint(req.RemoteSDP)
		if err != nil {
			logger.Warn("VoWiFi 来电 IMS SDP 解析失败，回退直通",
				"event", "VOWIFI_INBOUND_CALL_SDP_PARSE_FAIL",
				"device", deviceID,
				"call_id", req.CallID,
				"error", err.Error())
		} else {
			externalIP := m.sipRegistrar.GetExternalIP()
			if externalIP == "" {
				externalIP = "127.0.0.1"
			}
			relay, err = newRTPRelay(externalIP, imsEP)
			if err != nil {
				logger.Warn("VoWiFi 来电 RTP relay 创建失败，回退直通",
					"event", "VOWIFI_INBOUND_CALL_RELAY_FAIL",
					"device", deviceID,
					"call_id", req.CallID,
					"error", err.Error())
				relay = nil
			} else {
				logger.Info("VoWiFi 来电 RTP relay 已创建",
					"event", "VOWIFI_INBOUND_CALL_RELAY_CREATED",
					"device", deviceID,
					"call_id", req.CallID)
			}
		}
	}
	// Ensure relay is closed on error or non-200 response
	defer func() {
		if relay != nil && !relayClosed {
			relay.Close()
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
	inviteReq.AppendHeader(sip.NewHeader("From", fmt.Sprintf("<%s>;tag=%s", req.From, localTag)))
	// To = Linphone user
	inviteReq.AppendHeader(sip.NewHeader("To", fmt.Sprintf("<sip:%s@%s>", user.Username, user.ContactAddr.IP.String())))
	inviteReq.AppendHeader(sip.NewHeader("Call-ID", req.CallID))
	inviteReq.AppendHeader(sip.NewHeader("CSeq", "1 INVITE"))
	inviteReq.AppendHeader(sip.NewHeader("Contact", fmt.Sprintf("<sip:%s>", req.From)))
	inviteReq.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))

	// Set INVITE body: relay-rewritten SDP if relay is active,
	// otherwise passthrough IMS SDP directly.
	if relay != nil {
		inviteBody := rewriteSDPEndpoint(req.RemoteSDP, relay.ClientAdvertiseAddr())
		inviteReq.SetBody([]byte(inviteBody))
	} else if len(req.RemoteSDP) > 0 {
		inviteReq.SetBody([]byte(req.RemoteSDP))
	}

	logger.Info("VoWiFi 来电转发到 Linphone",
		"event", "VOWIFI_INBOUND_CALL_FORWARD",
		"device", deviceID,
		"call_id", req.CallID,
		"caller", req.From,
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
			"device", deviceID,
			"call_id", req.CallID,
			"error", err.Error())
		return ims.IncomingCallResponse{Accept: false, StatusCode: 503, Reason: "Linphone INVITE failed"}
	}

	// Wait for final response (ims-go 已自动回 180 Ringing)
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
			return ims.IncomingCallResponse{Accept: false, StatusCode: 487, Reason: "Request Terminated"}
		case resp, ok := <-tx.Responses():
			if !ok {
				return ims.IncomingCallResponse{Accept: false, StatusCode: 503, Reason: "No response from Linphone"}
			}
			if resp.StatusCode >= 100 && resp.StatusCode < 200 {
				// Provisional — ims-go 已回 180，此处仅记录
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

	if finalResp.StatusCode >= 200 && finalResp.StatusCode < 300 {
		// Call answered — set Linphone as relay client remote, store
		// dialog info and relay for BYE/CANCEL forwarding.
		if m.callEventPub != nil {
			m.callEventPub.OnCallConnected(deviceID, req.CallID)
		}
		var localSDP string
		if relay != nil && len(finalResp.Body()) > 0 {
			linphoneEP, err := parseSDPEndpoint(string(finalResp.Body()))
			if err != nil {
				logger.Warn("VoWiFi 来电 Linphone SDP 解析失败",
					"event", "VOWIFI_INBOUND_CALL_LINPHONE_SDP_FAIL",
					"device", deviceID,
					"call_id", req.CallID,
					"error", err.Error())
			} else {
				relay.SetClientRemote(linphoneEP)
			}
			// Return relay IMS endpoint SDP to IMS
			localSDP = buildSDPAnswer(relay.IMSAdvertiseAddr())
		} else {
			// No relay — passthrough Linphone SDP
			localSDP = string(finalResp.Body())
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
			DeviceID:   deviceID,
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
			"device", deviceID,
			"call_id", req.CallID,
			"status", finalResp.StatusCode,
			"remote_tag", remoteTag,
			"relay", relay != nil)

		return ims.IncomingCallResponse{Accept: true, LocalSDP: localSDP}
	}

	logger.Info("VoWiFi 来电 Linphone 拒绝",
		"event", "VOWIFI_INBOUND_CALL_REJECTED",
		"device", deviceID,
		"call_id", req.CallID,
		"status", finalResp.StatusCode,
		"reason", reason)
	// relay will be closed by defer
	return ims.IncomingCallResponse{Accept: false, StatusCode: finalResp.StatusCode, Reason: reason}
}

// handleInboundBye forwards an IMS BYE to Linphone, terminating the call.
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

// handleInboundCancel forwards an IMS CANCEL to Linphone.
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

// parseLinphoneURI parses a Linphone Contact URI into a sip.Uri.
func parseLinphoneURI(contactURI, deviceID string) (sip.Uri, error) {
	if strings.TrimSpace(contactURI) != "" {
		var uri sip.Uri
		if err := sip.ParseUri(contactURI, &uri); err == nil {
			return uri, nil
		}
	}
	return sip.Uri{}, fmt.Errorf("cannot parse Linphone contact URI for device %s", deviceID)
}

// --- RTP Relay (minimal, B2BUA use case) ---

// rtpRelay bridges RTP between IMS network and Linphone.
// Simplified 2-socket model (RTP only, no RTCP) for the B2BUA case.
type rtpRelay struct {
	mu         sync.Mutex
	imsConn    *net.UDPConn // IMS side (recv from IMS, send to Linphone)
	clientConn *net.UDPConn // Client side (recv from Linphone, send to IMS)

	imsRemote    *net.UDPAddr // IMS RTP endpoint (from SDP)
	clientRemote *net.UDPAddr // Linphone RTP endpoint (from SDP)

	imsAdvertise    *net.UDPAddr // What we advertise to IMS
	clientAdvertise *net.UDPAddr // What we advertise to Linphone

	closed    chan struct{}
	closeOnce sync.Once
}

func newRTPRelay(advertiseIP string, imsRemote *net.UDPAddr) (*rtpRelay, error) {
	// IMS side socket
	imsConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		return nil, fmt.Errorf("rtp relay ims listen: %w", err)
	}
	// Client side socket
	clientConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0})
	if err != nil {
		imsConn.Close()
		return nil, fmt.Errorf("rtp relay client listen: %w", err)
	}

	r := &rtpRelay{
		imsConn:    imsConn,
		clientConn: clientConn,
		imsRemote:  imsRemote,
		closed:     make(chan struct{}),
	}

	// Advertise addresses (what we tell each side to send to)
	advIP := net.ParseIP(advertiseIP)
	if advIP == nil {
		advIP = net.ParseIP("127.0.0.1")
	}
	r.imsAdvertise = &net.UDPAddr{IP: advIP, Port: imsConn.LocalAddr().(*net.UDPAddr).Port}
	r.clientAdvertise = &net.UDPAddr{IP: advIP, Port: clientConn.LocalAddr().(*net.UDPAddr).Port}

	// Start forwarding
	go r.forwardLoop(imsConn, func() *net.UDPAddr {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.clientRemote
	})
	go r.forwardLoop(clientConn, func() *net.UDPAddr {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.imsRemote
	})

	return r, nil
}

func (r *rtpRelay) forwardLoop(src *net.UDPConn, dstFn func() *net.UDPAddr) {
	buf := make([]byte, 2048)
	for {
		select {
		case <-r.closed:
			return
		default:
		}
		src.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, _, err := src.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}
		dst := dstFn()
		if dst == nil {
			continue
		}
		// Use the appropriate conn for sending (src conn can send too)
		_, _ = src.WriteToUDP(buf[:n], dst)
	}
}

func (r *rtpRelay) SetClientRemote(addr *net.UDPAddr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clientRemote = addr
}

func (r *rtpRelay) IMSAdvertiseAddr() *net.UDPAddr {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.imsAdvertise
}

func (r *rtpRelay) ClientAdvertiseAddr() *net.UDPAddr {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clientAdvertise
}

func (r *rtpRelay) Close() error {
	r.closeOnce.Do(func() {
		close(r.closed)
		r.imsConn.Close()
		r.clientConn.Close()
	})
	return nil
}

// --- SDP utilities (minimal) ---

// parseSDPEndpoint extracts the RTP endpoint (IP:port) from SDP.
// Looks for c= and m=audio lines.
func parseSDPEndpoint(sdp string) (*net.UDPAddr, error) {
	var ip string
	var port int
	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "c=IN IP4 ") {
			ip = strings.TrimPrefix(line, "c=IN IP4 ")
			ip = strings.Fields(ip)[0]
		} else if strings.HasPrefix(line, "m=audio ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				p, err := strconv.Atoi(fields[1])
				if err == nil {
					port = p
				}
			}
		}
	}
	if ip == "" || port == 0 {
		return nil, fmt.Errorf("no audio endpoint in SDP")
	}
	return &net.UDPAddr{IP: net.ParseIP(ip), Port: port}, nil
}

// rewriteSDPEndpoint replaces the c= and m=audio port with the given address.
func rewriteSDPEndpoint(sdp string, addr *net.UDPAddr) string {
	var out []string
	for _, line := range strings.Split(sdp, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "c=IN IP4 ") {
			out = append(out, "c=IN IP4 "+addr.IP.String())
		} else if strings.HasPrefix(trimmed, "m=audio ") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				fields[1] = strconv.Itoa(addr.Port)
			}
			out = append(out, strings.Join(fields, " "))
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\r\n")
}

// buildSDPAnswer builds a minimal SDP answer with the given endpoint.
func buildSDPAnswer(addr *net.UDPAddr) string {
	return fmt.Sprintf("v=0\r\n"+
		"o=- 0 0 IN IP4 %s\r\n"+
		"s=VoWiFi\r\n"+
		"c=IN IP4 %s\r\n"+
		"t=0 0\r\n"+
		"m=audio %d RTP/AVP 8 0 101\r\n"+
		"a=rtpmap:8 PCMA/8000\r\n"+
		"a=rtpmap:0 PCMU/8000\r\n"+
		"a=rtpmap:101 telephone-event/8000\r\n",
		addr.IP.String(), addr.IP.String(), addr.Port)
}
