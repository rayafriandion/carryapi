# 路由配置页与基于日志的健康状态设计

日期:2026-08-12
状态:已确认(待 spec 审阅)

## 背景与问题

carryAPI 的多供应商多模型路由后端已实现(`model_bindings` 表 + `Router.Select/Next` + failover),但前端入口"做了一半":单模型编辑页(`ModelEditView.vue`)只能选一个 provider + upstream_model,列表页(`ModelsView.vue`)虽有"上游绑定"弹窗能挂多个 binding,但入口分散、和编辑表单割裂,导致配完看不到多供应商路由效果。

同时,健康状态判定基于 `proxy/health.go` 的进程内内存计数(连续失败 3 次 → 冷却 30 秒),不基于真实流量数据,管理员看不到路由为何选了这家而非那家。

## 目标

1. 新增独立"路由配置"页,以模型为主视图,展开看/改每个 binding,成为配置多供应商路由的唯一入口。
2. 健康状态判定从"内存连续失败计数"改为"基于 `request_logs` 日志的时间窗口成功率聚合",数据来自真实转发日志。
3. 每个 binding 展示 6 格 × 4 小时(覆盖 24h)的状态时间轴,颜色按成功率分级。
4. 详情页展示性能指标:平均响应时延、TTFT(首字延迟)、吞吐量。

## 不做的事(YAGNI)

- 健康状态持久化到表(多实例共享)——留待以后真要多实例时。
- 主动 ping 探测 loop——状态全来自真实日志,不模拟不预估。
- 模型别名/正则匹配层——请求 model 名仍精确匹配 `custom_models.name`。
- `Router`/`forward.go` 的选择与 failover 核心逻辑——已按 bindings 工作,不动。

## 核心决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 路由配置页形态 | 独立页,以模型为主视图 | 与基本信息/定价解耦,配置入口唯一化 |
| 旧入口处理 | 移除 ModelsView 上游/路由弹窗 + ModelEditView 的 provider/upstream_model 字段 | 消除割裂,避免双入口 |
| 状态判定粒度 | 按 binding(provider_id + upstream_model) | 路由页按 binding 管理,色块也按 binding 才能看出哪个该摘 |
| 数据源 | `request_logs` 真实转发日志 | 不模拟不预估;客户端主动断开不计入失败统计 |
| TTFT | 这次一起加(migration 加列 + 计时) | 架构支持,一步到位 |
| 时间轴 | 24h = 6 格 × 4 小时,等宽 | 紧凑,成功率分级本身粗粒度,4 小时桶足够判定 |
| Router 接入方式 | 后台周期预算 + 内存缓存(每 1 分钟刷新) | 转发热路径不查 DB;1 分钟延迟可接受 |

## 架构总览

```
┌──────────────────────────────────────────────────────────────┐
│ 前端: RoutingView.vue (独立路由配置页)                          │
│  - 模型为主视图,展开看每个 binding                              │
│  - 每 binding: 6格×4h 状态时间轴(绿/黄/红/灰) + 当前延迟       │
│  - 行内增删改 binding + 切路由策略                              │
│  - 点 binding 进详情: 性能指标(平均时延/TTFT/吞吐量)            │
└──────────────────┬───────────────────────────────────────────┘
                   │ GET /api/routing/status  (时间轴+延迟聚合)
                   │ GET /api/routing/bindings/{id}/metrics (详情)
                   │ + 现有 bindings/routing CRUD API
┌──────────────────▼───────────────────────────────────────────┐
│ 后端: catalog/routing_stats.go (日志聚合统计层)                │
│  - 按 (provider_id, upstream_model) 聚合 request_logs          │
│  - 6 格 × 4h 窗口: 算成功率 → 色块                              │
│  - 详情: avg(duration_ms) / avg(ttft_ms) / 吞吐量(req/h)       │
└──────────────────┬───────────────────────────────────────────┘
                   │ 读
┌──────────────────▼───────────────────────────────────────────┐
│ HealthCache (catalog/health_cache.go,后台每 1 分钟预算)       │
│  - 对每个 active binding 查最近 4h 桶成功率                     │
│  - 缓存到内存 map[bindingKey]CachedHealth                      │
│  - Router.Select 读缓存,不查 DB                                │
└──────────────────┬───────────────────────────────────────────┘
                   │ 读
┌──────────────────▼───────────────────────────────────────────┐
│ 数据源: request_logs 表 (加 ttft_ms 列 + 复合索引)             │
│  - 每条真实转发日志: status_code / duration_ms / ttft_ms       │
│  - created_at 按时间分桶                                       │
│  - 按 provider_id + upstream_model 过滤到 binding 粒度         │
└──────────────────────────────────────────────────────────────┘

转发路径改动(轻):
  forward.go / stream.go: 记录 ttft_ms (首字节到达时间)
  stats.go: 写入 ttft_ms 列
  health.go: 废弃旧的连续失败计数逻辑,Router 改读 HealthCache
```

## 数据模型与 schema 改动

### Migration v4

```sql
ALTER TABLE request_logs ADD COLUMN ttft_ms INTEGER;
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_model
    ON request_logs(provider_id, upstream_model, created_at);
```

- `ttft_ms`:首字延迟(毫秒),NULL 表示非流式或未捕获。
- 新索引 `(provider_id, upstream_model, created_at)`:所有按 binding + 时间窗口聚合走它。
- 现有索引保留不动。

### requestContext 加字段(`proxy/proxy.go`)

```go
type requestContext struct {
    // ...现有字段...
    start       time.Time  // 已有
    firstByteAt time.Time  // 新增:上游首字节到达时间
}
```

### health.go 处置

`bindingHealth` 内存 map(连续失败计数 + cooldown)**不再用于路由判定**。`RecordSuccess/RecordFailure` 调用点从 forward.go 移除。健康判定完全走日志聚合 → HealthCache。文件可移除或保留空壳。

### 不动的表

`custom_models`、`model_bindings`、`upstream_providers`、`model_prices` 全部不动。`Model.ProviderID`/`UpstreamModel` struct 字段可保留但不再由表单读写(表里列保留,避免破坏)。

## 日志聚合统计层

新增 `internal/catalog/routing_stats.go`。

```go
type RoutingStats struct {
    db *sql.DB
}

// 时间轴:6 格 × 4 小时
type TimeBucket struct {
    BucketStart time.Time
    Total       int
    Success     int
    Status      string  // "healthy" | "warning" | "unhealthy" | "no_data"
}

type BindingTimeline struct {
    ProviderID    int64
    UpstreamModel string
    Buckets       []TimeBucket  // 长度 6
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

func (s *RoutingStats) BindingTimeline(providerID int64, upstreamModel string, now time.Time) (*BindingTimeline, error)
func (s *RoutingStats) BindingHealth(providerID int64, upstreamModel string, now time.Time) (string, error)
func (s *RoutingStats) BindingMetrics(providerID int64, upstreamModel string, now time.Time) (*BindingMetrics, error)
```

### 成功率与状态映射

```
窗口内成功率 = success / total
  ≥ 95%                → "healthy"   🟢
  80% ~ 95%(含80不含95) → "warning"   🟡
  < 80%                → "unhealthy" 🔴
  total = 0(无请求)    → "no_data"   ⚪ 灰
```

### 边界规则

- 某 binding 所有时段都连续失败(无成功请求):即使请求量极低,只要 total > 0 且 success = 0 → 成功率 0% < 80% → 红。
- 无任何请求流量:total = 0 → 灰,不判健康等级。
- 客户端主动断开:`error_type = 'client_disconnect'`,从 total 排除(不计成功也不计失败)。

### 聚合 SQL(6 格 × 4 小时)

时区约定:SQL 内用 `'localtime'` 把 `created_at`(UTC 存储)转本地时区分桶;Go 侧补桶时用 `now.Local()` 对齐,确保前端展示的时间范围与用户本地时钟一致。

```sql
SELECT
  strftime('%Y-%m-%d %H',
    datetime(created_at, 'unixepoch', 'localtime',
             '-' || (strftime('%H', created_at, 'unixepoch', 'localtime') % 4) || ' hours')
  ) AS bucket,
  COUNT(CASE WHEN error_type != 'client_disconnect' THEN 1 END) AS total,
  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END) AS success
FROM request_logs
WHERE provider_id = ? AND upstream_model = ?
  AND created_at >= ? AND created_at < ?
GROUP BY bucket
ORDER BY bucket;
```

Go 侧补齐 6 个 4 小时桶(无数据的桶 total=0 → 灰)。

### 性能指标 SQL

```sql
SELECT
  AVG(duration_ms) AS avg_latency,
  AVG(ttft_ms)     AS avg_ttft,
  COUNT(*) * 1.0 / 24 AS throughput,
  COUNT(*) AS total,
  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END) * 1.0 / COUNT(*) AS success_rate
FROM request_logs
WHERE provider_id = ? AND upstream_model = ?
  AND created_at >= ? AND created_at < ?;
```

## HealthCache + 后台预算

新增 `internal/catalog/health_cache.go`。

```
HealthCache
  - states: map[string]CachedHealth  (key = provider_id:upstream_model)
  - mu: sync.RWMutex
  - Start(ctx): 每 1 分钟预算一次
      - 遍历所有 active provider 的所有 binding(Bindings.ListAll)
      - 对每个 binding 调 RoutingStats.BindingHealth(最近 4h 桶成功率)
      - 写入缓存
  - Get(providerID, upstreamModel) -> string: 并发安全读
```

- 生命周期:随服务启动 `go healthCache.Start(ctx)`,ctx 来自 main 的 shutdown 信号。
- 缓存有最多 1 分钟延迟——可接受(健康状态是统计性指标,不需要毫秒级实时)。
- 新 binding 从未有过请求 → 缓存里 `no_data` → Router 当作"无数据可用"对待(不排除,等同 healthy,让首个请求试探)。

## Router 接入

`catalog/router.go` 的 `activeBindings` 改读 `HealthCache.Get`:

```go
st := p.healthCache.Get(b.ProviderID, b.UpstreamModel)
if st == "unhealthy" {
    continue  // 只排除明确不健康;no_data/warning/healthy 都保留
}
```

- `RoutingStrategyAuto` + `AutoModeHealth`:`unhealthy` 放最后,其余按 priority/weight。
- `AutoModePriority`/`AutoModeFailover`/`Random`:不受健康影响(按原逻辑),`unhealthy` 在 failover 时仍会被尝试(符合 failover "挨个试"的语义)。
- `Router` 构造函数改签名:`NewRouter(providers, health)` → `NewRouter(providers, healthCache)`,对应改 `proxy.go:43` 的 `getRouter`。

## 转发路径改动

### TTFT 计时

**流式路径**(`stream.go`):
- `Client.Do` 返回时记 `firstByteAt`(响应头到达,保底)。
- 首次 `scanner.Scan()` 成功时更新 `firstByteAt`(第一个 SSE 事件块到达,更准)。

```go
// stream.go,streamResponse 内
rc.firstByteAt = time.Now()  // Client.Do 返回后,保底

scanner := bufio.NewScanner(upResp.Body)
firstByte := true
for scanner.Scan() {
    if firstByte {
        rc.firstByteAt = time.Now()  // 首事件块到达,更准
        firstByte = false
    }
    // ...已有处理...
}
```

**非流式路径**(`forward.go` `forwardNonStreaming`):
```go
rc.firstByteAt = time.Now()  // Client.Do 返回后,响应头到达 = 非流式 TTFT 近似
```

### 客户端断开判定(`stream.go`)

现状:客户端断开和上游断开都标 `error_type='upstream'` + `status_code=200`,无法区分。

**改动**:`scanner.Err()` 处检测客户端断开:

```go
if err := scanner.Err(); err != nil {
    if isClientDisconnect(err) {
        rc.errorType = "client_disconnect"  // 不计入失败统计
    } else {
        rc.errorType = "upstream"
    }
    rc.errorMessage = "stream read error: " + err.Error()
}

func isClientDisconnect(err error) bool {
    return errors.Is(err, context.Canceled) ||
           errors.Is(err, io.ErrClosedPipe) ||
           strings.Contains(err.Error(), "broken pipe")
}
```

### stats.go 写入 ttft_ms

```go
var ttftMs int64
if !rc.firstByteAt.IsZero() && !rc.start.IsZero() {
    ttftMs = rc.firstByteAt.Sub(rc.start).Milliseconds()
}
// INSERT 加 ttft_ms 列
```

非流式且上游失败(`Client.Do` 返回 err)时 `firstByteAt` 为零值 → `ttft_ms` 写 NULL。

### 成功/失败/客户端断开的统计定义

- `error_type = 'none'`:正常完成(含流式完整结束)→ 成功。
- `error_type = 'client_disconnect'`:客户端断开 → 从 total 排除(既不算成功也不算失败)。
- `error_type = 'upstream'` / 其他:上游失败 → 算失败。

## 前端 RoutingView

### 页面结构(以模型为主视图)

```
路由配置                                              [刷新]
┌────────────────────────────────────────────────────────────────────┐
│ ▼ my-gpt4   策略:自动路由 / 健康感知              [路由策略] [收起]    │
│   ┌────────────┬─────────────┬──────┬──────┬─────┬────────────┬──┐│
│   │ 供应商      │ 上游模型     │ 优先级│ 权重 │ 启用 │ 状态(6格×4h)  │ ││
│   ├────────────┼─────────────┼──────┼──────┼─────┼────────────┼──┤│
│   │ openai-prod│ gpt-4o      │ 100  │ 1    │ ✓   │🟢🟢🟡🟢🟢🟢 420│ ›││
│   │ deepseek   │ deepseek-v3 │ 200  │ 1    │ ✓   │🟢🔴🔴⚪⚪⚪   —│ ›││
│   │ azure-fail│ gpt-4       │ 300  │ 1    │ ✗   │⚪⚪⚪⚪⚪⚪    —│ ›││
│   └────────────┴─────────────┴──────┴──────┴─────┴────────────┴──┘│
│   [+ 添加上游绑定]                                                  │
├────────────────────────────────────────────────────────────────────┤
│ ▶ my-claude  策略:随机                                              │
└────────────────────────────────────────────────────────────────────┘
```

### 状态列(6 格时间轴)

- 6 格 × 4 小时,等宽,从左(24h 前)到右(最近 4h)。
- 🟢 绿(≥95%) / 🟡 黄(80-95%) / 🔴 红(<80%) / ⚪ 灰(无请求)。
- 悬停 tooltip:4 小时时间范围 + 总请求数 + 成功率百分比。
- 时间轴右侧紧跟当前延迟(latency_ms,24h 平均)。

### 详情(点 binding 行尾 `›`)

展示该 binding 的 24h 性能指标(不影响色块颜色):

```
openai-prod / gpt-4o  —  最近 24 小时
┌──────────────────┬──────────────────┬──────────────────┐
│ 平均响应时延       │ 平均首字延迟(TTFT)│ 吞吐量           │
│ 1,240 ms         │ 420 ms           │ 85 req/h         │
└──────────────────┴──────────────────┴──────────────────┘
```

### 数据加载

- `GET /api/routing/status`:一次返回所有 model + 每个 binding 的 6 格时间轴 + 24h 平均延迟。数据量 = 模型数 × binding 数 × 6 格,可接受。
- 详情指标:点开时按需 `GET /api/routing/bindings/{bindingID}/metrics`,不随列表加载。
- 不前端轮询,要更新点"刷新"。

### 交互(复用现有 API)

- 增删改 binding:现有 `/api/models/{id}/bindings` CRUD。
- 切路由策略:现有 `PUT /api/models/{id}/routing`。
- 添加 binding:provider 下拉来自现有 `GET /api/providers`。

### 移除的旧入口

- `ModelsView.vue`:删"上游"弹窗组件 + "路由"按钮 + 对应逻辑。列表行只留基本信息 + 从供应商导入。
- `ModelEditView.vue`:移除 provider_id(单选) + upstream_model 字段 + 相关提交逻辑。表单只留 name/enabled/定价。提交 `PUT /api/models/{id}` 时不再带 provider/upstream。
- 导航加"路由配置" → `/routing` → `RoutingView.vue`。

## API 接口

### 新增

**`GET /api/routing/status`(admin only,只读)**

```jsonc
{
  "models": [
    {
      "model_id": 1,
      "name": "my-gpt4",
      "enabled": true,
      "routing_strategy": "auto",
      "auto_mode": "health",
      "bindings": [
        {
          "binding_id": 3,
          "provider_id": 2,
          "provider_name": "openai-prod",
          "provider_status": "active",
          "upstream_model": "gpt-4o",
          "priority": 100,
          "weight": 1,
          "enabled": true,
          "timeline": ["healthy","healthy","warning","healthy","healthy","healthy"],
          "avg_latency_ms": 1240,
          "last_request_at": "2026-08-12T14:03:00Z"
        }
      ]
    }
  ]
}
```

实现:`catalog.Handler.GetRoutingStatus` —— 遍历所有 enabled model → `Bindings.ListEnabledByModel` → 对每个 binding 调 `RoutingStats.BindingTimeline` → 合并 provider 信息。

**`GET /api/routing/bindings/{bindingID}/metrics`(admin only,只读)**

```jsonc
{
  "provider_id": 2,
  "provider_name": "openai-prod",
  "upstream_model": "gpt-4o",
  "avg_latency_ms": 1240,
  "avg_ttft_ms": 420,
  "throughput_per_hour": 85.3,
  "total_requests_24h": 2048,
  "success_rate": 0.982
}
```

实现:`catalog.Handler.GetBindingMetrics` → 用 bindingID 查 `model_bindings` 拿 provider_id + upstream_model → 调 `RoutingStats.BindingMetrics`。

### 现有 API 复用(不动后端)

- bindings CRUD:`GET/POST /api/models/{id}/bindings`、`PUT/DELETE /api/models/{id}/bindings/{bindingID}`
- 路由策略:`PUT /api/models/{id}/routing`
- provider 列表:`GET /api/providers`

### 路由注册(`server/router.go`)

```go
r.With(adminGuard).Get("/api/routing/status", h.GetRoutingStatus)
r.With(adminGuard).Get("/api/routing/bindings/{bindingID}/metrics", h.GetBindingMetrics)
```

## 依赖装配(`cmd/carryapi/main.go`)

```go
routingStats := catalog.NewRoutingStats(db)
healthCache := catalog.NewHealthCache(catBindings, routingStats)
go healthCache.Start(ctx)

proxyDeps := proxy.Deps{
    // ...现有...
    HealthCache: healthCache,  // 替换原 health
}

catHandler := catalog.NewHandler(/*...*/, routingStats)
```

## 后端小调整

### `ModelEditView` 后端对应调整(`catalog/model.go`)

- `updateInTx`:只改 model 行(name/enabled/routing_strategy/auto_mode),不再碰 binding。
- `createInTx`:创建 model 时不自动建首条 binding,改为创建空 binding 的 model。
- `Model.ProviderID`/`UpstreamModel` struct 字段:可保留但不再读写(表里列保留)。
- `PUT /api/models/{id}` handler:即使前端误传 provider_id/upstream_model 字段,后端忽略不写入(不再有对应列写入逻辑)。`GET /api/models` 返回时这两个字段置空或移除。

## 改动文件总览

| 文件 | 改动类型 |
|------|---------|
| `internal/db/migrations.go` | 加 v4:ttft_ms 列 + 复合索引 |
| `internal/catalog/routing_stats.go` | 新增:聚合统计层 |
| `internal/catalog/health_cache.go` | 新增:后台预算缓存 |
| `internal/catalog/router.go` | 改:activeBindings 读 HealthCache |
| `internal/catalog/handler.go` | 加:GetRoutingStatus + GetBindingMetrics |
| `internal/catalog/model.go` | 改:updateInTx/createInTx 不再双写 binding |
| `internal/proxy/proxy.go` | 改:requestContext 加 firstByteAt;Router 注入换 HealthCache |
| `internal/proxy/forward.go` | 改:非流式 firstByteAt + 移除 4 处 Record* 调用 |
| `internal/proxy/stream.go` | 改:流式 firstByteAt + isClientDisconnect |
| `internal/proxy/stats.go` | 改:算 ttftMs + 写 ttft_ms 列 |
| `internal/proxy/health.go` | 废弃/移除 |
| `internal/server/router.go` | 加:两个新路由 |
| `cmd/carryapi/main.go` | 改:装配 routingStats + healthCache |
| `web/src/views/RoutingView.vue` | 新增:路由配置页 |
| `web/src/views/ModelsView.vue` | 改:移除上游/路由弹窗 |
| `web/src/views/ModelEditView.vue` | 改:移除 provider/upstream_model 字段 |
| 导航/路由表 | 加:/routing 入口 |

## 测试策略

- `routing_stats.go`:单元测试,用内存 SQLite + 预置 request_logs 数据,验证 6 格时间轴补齐、成功率映射、边界(total=0→灰、success=0→红、client_disconnect 排除)。
- `health_cache.go`:单元测试,验证预算写入缓存、Get 并发安全。
- `router.go`:补充多 binding 测试(priority/weight/health 过滤),现状只有单 binding 间接覆盖。
- `forward.go`/`stream.go`:TTFT 计时点 + client_disconnect 判定的集成测试。
- 前端:手动验证列表加载、时间轴渲染、binding CRUD、详情指标。
