package server

import (
	"net/http"
	"strconv"

	"carryapi/internal/api"
)

func (s *Server) handleGatewayInfo(w http.ResponseWriter, r *http.Request) {
	host := "127.0.0.1:" + strconv.Itoa(s.cfg.Port)
	if listenHost(s.deps.Store) == "0.0.0.0" {
		host = r.Host // 已含 host:port
	}
	api.JSON(w, 200, map[string]string{"base_url": "http://" + host + "/v1"})
}
