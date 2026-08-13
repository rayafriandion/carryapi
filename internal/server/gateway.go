package server

import (
	"net"
	"net/http"
	"strconv"

	"carryapi/internal/api"
)

func (s *Server) handleGatewayInfo(w http.ResponseWriter, r *http.Request) {
	lc := s.listenerConfig()
	host := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.cfg.Port))
	if lc.broadcastOn() && r.Host != "" {
		host = r.Host
	}
	api.JSON(w, 200, map[string]string{"base_url": "http://" + host + "/v1"})
}
