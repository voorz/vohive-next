package server

import (
	"fmt"
	"net"
)

// FindAvailablePort 从 startPort 开始递增尝试 TCP bind，
// 返回第一个可用的端口号。最多尝试 1000 个端口。
func FindAvailablePort(startPort int) (int, error) {
	if startPort < 1 || startPort > 65535 {
		return 0, fmt.Errorf("invalid start port: %d", startPort)
	}
	for port := startPort; port < startPort+1000 && port <= 65535; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}
		_ = ln.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found in range [%d, %d]", startPort, startPort+999)
}
