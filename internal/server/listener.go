package server

import (
	"fmt"
	"net"
	"strconv"

	"carryapi/internal/settings"
)

// listenHost 从 settings 读 listen_host;默认 0.0.0.0(广播开)
func listenHost(store *settings.Store) string {
	v, ok, _ := store.Get("listen_host")
	if !ok || v == "" {
		return "0.0.0.0"
	}
	return v
}

// listenAddr 返回最终监听 host:port
func (s *Server) listenAddr() (string, error) {
	host := listenHost(s.deps.Store)
	// Port=0 让系统分配端口(测试用)
	return net.JoinHostPort(host, strconv.Itoa(s.cfg.Port)), nil
}

// resolveListener 真正开 listener
func (s *Server) resolveListener() (net.Listener, error) {
	addr, err := s.listenAddr()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	s.actualAddr = ln.Addr().String()
	return ln, nil
}
