package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Prober 对上游供应商发起轻量 GET /models 请求,用于拉取模型列表与测连通/延迟。
type Prober struct {
	client *http.Client
}

func NewProber(client *http.Client) *Prober {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Prober{client: client}
}

func (p *Prober) SetClient(client *http.Client) { p.client = client }

// modelsPath 返回该供应商协议对应的模型列表路径。
func modelsPath(protocol string) string {
	if protocol == "anthropic" {
		return "/v1/models"
	}
	return "/models"
}

// authHeaders 按协议设置鉴权头。
func authHeaders(provider Provider) map[string]string {
	if provider.Protocol == "anthropic" {
		return map[string]string{
			"x-api-key":         provider.APIKey,
			"anthropic-version": "2023-06-01",
		}
	}
	return map[string]string{"Authorization": "Bearer " + provider.APIKey}
}

// do 发起 GET /models,返回响应体;非 2xx 返回错误。
func (p *Prober) do(ctx context.Context, provider Provider) ([]byte, error) {
	url := provider.BaseURL + modelsPath(provider.Protocol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range authHeaders(provider) {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, truncateBody(body))
	}
	return body, nil
}

func truncateBody(b []byte) string {
	if len(b) > 200 {
		return string(b[:200])
	}
	return string(b)
}

// FetchModels 返回该供应商的模型名列表(尽力解析;解析失败返回空列表)。
func (p *Prober) FetchModels(ctx context.Context, provider Provider) ([]string, error) {
	body, err := p.do(ctx, provider)
	if err != nil {
		return nil, err
	}
	var v struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		// 解析失败不阻断,返回空列表(调用方走手动添加路径)
		return []string{}, nil
	}
	out := make([]string, 0, len(v.Data))
	for _, d := range v.Data {
		if d.ID != "" {
			out = append(out, d.ID)
		}
	}
	return out, nil
}

// Ping 返回请求耗时(连通成功时),非 2xx/超时/网络错误返回 error。
func (p *Prober) Ping(ctx context.Context, provider Provider) (time.Duration, error) {
	start := time.Now()
	if _, err := p.do(ctx, provider); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}
