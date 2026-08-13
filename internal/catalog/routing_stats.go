package catalog

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	StatusHealthy   = "healthy"
	StatusWarning   = "warning"
	StatusUnhealthy = "unhealthy"
	StatusNoData    = "no_data"
)

type TimeBucket struct {
	BucketStart time.Time
	Total       int
	Success     int
	Status      string
}

type BindingTimeline struct {
	ProviderID    int64
	UpstreamModel string
	Buckets       []TimeBucket
	AvgLatencyMs  int64
	LastRequestAt *time.Time
}

type BindingMetrics struct {
	AvgLatencyMs      int64
	AvgTtftMs         int64
	ThroughputPerHour float64
	TotalRequests24h  int
	SuccessRate       float64
}

type RoutingStats struct {
	db *sql.DB
}

func NewRoutingStats(db *sql.DB) *RoutingStats {
	return &RoutingStats{db: db}
}

// statusFromRate 按成功率映射状态色。
func statusFromRate(success, total int) string {
	if total == 0 {
		return StatusNoData
	}
	rate := float64(success) / float64(total)
	switch {
	case rate >= 0.95:
		return StatusHealthy
	case rate >= 0.80:
		return StatusWarning
	default:
		return StatusUnhealthy
	}
}

// BindingTimeline 返回最近 24h 的 6 个 4 小时桶 + 24h 平均延迟。
func (s *RoutingStats) BindingTimeline(providerID int64, upstreamModel string, now time.Time) (*BindingTimeline, error) {
	localNow := now.Local()
	start := localNow.Add(-24 * time.Hour)

	rows, err := s.db.Query(`
		SELECT
		  strftime('%Y-%m-%d %H',
		    datetime(created_at, 'localtime',
		             '-' || (strftime('%H', created_at, 'localtime') % 4) || ' hours')
		  ) AS bucket,
		  COUNT(CASE WHEN error_type != 'client_disconnect' THEN 1 END) AS total,
		  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END) AS success
		FROM request_logs
		WHERE provider_id = ? AND upstream_model = ?
		  AND created_at >= ? AND created_at < ?
		GROUP BY bucket
		ORDER BY bucket`,
		providerID, upstreamModel,
		start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query timeline: %w", err)
	}
	defer rows.Close()

	bucketMap := map[string]TimeBucket{}
	for rows.Next() {
		var bk string
		var total, success int
		if err := rows.Scan(&bk, &total, &success); err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		bucketMap[bk] = TimeBucket{Total: total, Success: success}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 补齐 6 个 4 小时桶
	buckets := make([]TimeBucket, 6)
	for i := 0; i < 6; i++ {
		bStart := localNow.Add(-time.Duration(6-1-i) * 4 * time.Hour).Truncate(4 * time.Hour)
		key := bStart.Format("2006-01-02 15")
		b := bucketMap[key]
		b.BucketStart = bStart
		b.Status = statusFromRate(b.Success, b.Total)
		buckets[i] = b
	}

	// 24h 平均延迟
	var avgLatency sql.NullInt64
	_ = s.db.QueryRow(`SELECT AVG(duration_ms) FROM request_logs WHERE provider_id=? AND upstream_model=? AND created_at>=? AND created_at<? AND duration_ms IS NOT NULL`,
		providerID, upstreamModel, start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339)).Scan(&avgLatency)

	// 最近请求时间
	var lastReq sql.NullString
	_ = s.db.QueryRow(`SELECT created_at FROM request_logs WHERE provider_id=? AND upstream_model=? ORDER BY created_at DESC LIMIT 1`,
		providerID, upstreamModel).Scan(&lastReq)

	tl := &BindingTimeline{
		ProviderID:    providerID,
		UpstreamModel: upstreamModel,
		Buckets:       buckets,
		AvgLatencyMs:  avgLatency.Int64,
	}
	if lastReq.Valid {
		t, err := time.Parse(time.RFC3339, lastReq.String)
		if err == nil {
			tl.LastRequestAt = &t
		}
	}
	return tl, nil
}

// BindingHealth 返回最近 4 小时桶的状态(供 Router 用)。
func (s *RoutingStats) BindingHealth(providerID int64, upstreamModel string, now time.Time) (string, error) {
	localNow := now.Local()
	start := localNow.Add(-4 * time.Hour)
	var total, success int
	err := s.db.QueryRow(`
		SELECT
		  COUNT(CASE WHEN error_type != 'client_disconnect' THEN 1 END),
		  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END)
		FROM request_logs
		WHERE provider_id = ? AND upstream_model = ?
		  AND created_at >= ? AND created_at < ?`,
		providerID, upstreamModel,
		start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339)).Scan(&total, &success)
	if err != nil {
		return StatusNoData, fmt.Errorf("query health: %w", err)
	}
	return statusFromRate(success, total), nil
}

// BindingMetrics 返回 24h 性能详情。
func (s *RoutingStats) BindingMetrics(providerID int64, upstreamModel string, now time.Time) (*BindingMetrics, error) {
	localNow := now.Local()
	start := localNow.Add(-24 * time.Hour)
	var avgLatency, avgTtft sql.NullInt64
	var total, success int
	err := s.db.QueryRow(`
		SELECT
		  AVG(duration_ms), AVG(ttft_ms),
		  COUNT(*),
		  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END)
		FROM request_logs
		WHERE provider_id = ? AND upstream_model = ?
		  AND created_at >= ? AND created_at < ?`,
		providerID, upstreamModel,
		start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339)).Scan(&avgLatency, &avgTtft, &total, &success)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	m := &BindingMetrics{
		AvgLatencyMs:     avgLatency.Int64,
		AvgTtftMs:        avgTtft.Int64,
		TotalRequests24h: total,
	}
	if total > 0 {
		m.SuccessRate = float64(success) / float64(total)
		m.ThroughputPerHour = float64(total) / 24.0
	}
	return m, nil
}
