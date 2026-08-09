# carryAPI

自托管 API 聚合路由服务。多上游聚合、三种协议互转(OpenAI Chat / Responses / Anthropic)、可视化配置、用量与费用统计、成功率监控、多用户认证。

## 状态

子项目 1(项目骨架与基础设施)已完成。后续子项目:认证、协议适配层(IR)、上游代理、统计与管理 API、前端。

## 构建

需要 Go 1.22+ 与 Node.js。

```bash
# 前端
cd web && npm install && npm run build

# 后端(内嵌前端)
go build -o carryapi ./cmd/carryapi
```

交叉编译:

```bash
GOOS=linux GOARCH=amd64 go build -o carryapi-linux-amd64 ./cmd/carryapi
GOOS=windows GOARCH=amd64 go build -o carryapi-windows-amd64.exe ./cmd/carryapi
```

## 运行

```bash
./carryapi
```

默认监听 `0.0.0.0:8080`(广播开,其他设备可访问)。数据文件 `./carryapi.db`,主密钥 `./carryapi.key`(首次自动生成)。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `CARRYAPI_PORT` | 8080 | 监听端口 |
| `CARRYAPI_DB_PATH` | ./carryapi.db | 数据库路径 |
| `CARRYAPI_MASTER_KEY` | (自动生成) | 敏感字段加密主密钥,32 字节 |

### 广播开关

广播开 = 监听 `0.0.0.0`(局域网/公网可访问);广播关 = 监听 `127.0.0.1`(仅本机)。存于数据库 `settings` 表 `listen_host` 键。后续子项目的管理后台提供可视化切换(改值后需重启进程)。

## 开发

```bash
# 前端(热更新)
cd web && npm run dev

# 后端
go run ./cmd/carryapi
```

前端开发服务器把 `/api` 和 `/v1` 代理到后端 `127.0.0.1:8080`。

## 测试

```bash
go test ./...
```
