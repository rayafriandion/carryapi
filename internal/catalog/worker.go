package catalog

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// KeyRetryWorker 负责冷却期结束后的上游 key 后台自测:
// 对 cooling_down 且 retry_after 到期的 key 连续自测 KeyRetryProbeAttempts(3) 次,
// 成功则恢复为 active,失败则直接删除(软删除,保留审计日志)。
// 即需求中的"过 1 小时再自行重试 3 次,若仍然失败则直接删除"。
type KeyRetryWorker struct {
	providers *ProviderStore
	client    *http.Client
	interval  time.Duration
}

// NewKeyRetryWorker 创建后台自测 worker;client 为空时使用默认 10s 超时客户端。
func NewKeyRetryWorker(providers *ProviderStore, client *http.Client) *KeyRetryWorker {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &KeyRetryWorker{providers: providers, client: client, interval: 1 * time.Minute}
}

// Start 启动后台循环,每 1 分钟检查一次到期 key;ctx 取消时退出。
func (w *KeyRetryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *KeyRetryWorker) runOnce(ctx context.Context) {
	keys, err := w.providers.RetryDueKeys()
	if err != nil {
		return
	}
	for _, k := range keys {
		w.retryKey(ctx, k)
	}
}

func (w *KeyRetryWorker) retryKey(ctx context.Context, k ProviderAPIKey) {
	provider, err := w.providers.Get(k.ProviderID)
	if err != nil {
		return
	}
	_ = w.providers.logKeyEvent(k.ID, k.ProviderID, KeyEventRetryStarted, "background probe started after cooldown")
	prober := NewProber(w.client)
	probeProv := Provider{BaseURL: provider.BaseURL, APIKey: k.APIKey, Protocol: provider.Protocol}
	var lastErr error
	for attempt := 0; attempt < KeyRetryProbeAttempts; attempt++ {
		if _, err := prober.do(ctx, probeProv); err == nil {
			_ = w.providers.logKeyEvent(k.ID, k.ProviderID, KeyEventRetrySuccess, fmt.Sprintf("probe ok on attempt %d", attempt+1))
			_ = w.providers.RecoverKey(k.ID, "background probe succeeded, key restored to service")
			return
		} else {
			lastErr = err
		}
	}
	_ = w.providers.logKeyEvent(k.ID, k.ProviderID, KeyEventRetryFailed, lastErr.Error())
	_ = w.providers.DeleteKey(k.ID, false,
		"retried "+strconv.Itoa(KeyRetryProbeAttempts)+" times after cooldown, still failed: "+lastErr.Error())
}
