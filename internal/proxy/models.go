package proxy

import (
	"net/http"

	"carryapi/internal/ir"
)

// handleModels 返回启用的模型列表(OpenAI 格式),需 API Key 鉴权。
// GET /v1/models -> {"object":"list","data":[{"id":"my-gpt4","object":"model","created":...,"owned_by":"carryapi"}]}
func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	if _, _, err := p.authenticate(r); err != nil {
		rc := &requestContext{downstream: "chat", requestID: ""}
		p.writeError(w, rc, asIRError(err))
		return
	}
	models, err := p.deps.Models.ListEnabled()
	if err != nil {
		rc := &requestContext{downstream: "chat", requestID: ""}
		p.writeError(w, rc, ir.NewError("internal", "list_failed", "failed to list models", 500))
		return
	}
	data := make([]map[string]any, 0, len(models))
	for _, m := range models {
		data = append(data, map[string]any{
			"id": m.Name, "object": "model", "created": m.CreatedAt.Unix(), "owned_by": "carryapi",
		})
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, 200, map[string]any{"object": "list", "data": data})
}
