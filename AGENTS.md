# AGENTS.md

## 项目概览

`comp-health` 是一个 Go 编写的轻量健康检查工具，支持两种运行模式：

- `server`：提供 HTTP API、接收 agent 上报、展示内嵌 Web UI，并可运行本地探针。
- `agent`：不提供 UI，只运行本地探针并按周期主动上报到 server。

部署目标是 **单二进制 `health` + 一个 YAML 配置文件**。Web 页面资源通过 Go `embed` 编译进程序，不依赖外部静态文件目录。

## 关键目录

- `cmd/health/main.go`：CLI 入口，支持 `server`、`agent`、`init-config`。
- `internal/app/server`：server 模式启动编排。
- `internal/app/agent`：agent 模式启动编排。
- `internal/config`：YAML 配置结构、默认值和校验。
- `internal/probe`：探针接口、注册表和具体 adapter。
- `internal/scheduler`：定时调度探针执行。
- `internal/report`：agent 到 server 的上报客户端。
- `internal/server`：HTTP API 与 Web UI 服务。
- `internal/store`：状态存储抽象与内存实现。
- `internal/webfs`：通过 `embed` 打包的前端资源。
- `configs/`：示例配置文件。

## 构建与运行

优先使用 `Makefile` 中已有命令：

- `make build`：构建当前平台产物到 `dist/health`
- `make build-linux` / `make build-darwin` / `make build-windows`：交叉编译
- `make build-all`：构建全部平台
- `make tidy`：执行 `go mod tidy`
- `make test`：执行 `go test ./...`
- `make run-server`：使用 `configs/server.yaml` 本地运行 server
- `make run-agent`：使用 `configs/agent.yaml` 本地运行 agent

也可直接运行：

- `./dist/health server -c configs/server.yaml`
- `./dist/health agent -c configs/agent.yaml`
- `./dist/health init-config --mode server --out config.yaml`

## 配置约定

- 顶层 `mode` 只能是 `server` 或 `agent`。
- 探针定义位于 `probes` 数组，通过 `type` 分发到不同 adapter。
- 当前已实现的探针类型：`http`、`tcp`、`shell`。
- server 和 agent 共用同一套探针 schema。
- `storage.driver` 当前仅内存实现，重启后状态会丢失。

示例配置请直接参考：

- [README.md](./README.md)
- [configs/server.yaml](./configs/server.yaml)
- [configs/agent.yaml](./configs/agent.yaml)

## 代码约定

- 新增探针时：
  1. 在 `internal/probe/<name>/` 下实现 `probe.Adapter` 接口。
  2. 在 `internal/app/server/run.go` 和 `internal/app/agent/run.go` 中注册新 adapter。
- 状态值使用 `internal/model/model.go` 中定义的 `up` / `down` 常量。
- 页面资源应保留在 `internal/webfs/web/`，并通过 `internal/webfs/embed.go` 嵌入；不要改成运行时依赖外部文件。
- 若修改 CLI 或构建方式，优先同步更新 `Makefile`。
- 若修改配置结构，必须同步更新 `internal/config/config.go` 中的默认值、校验逻辑和 `WriteExample` 示例内容。

## 常见注意点

- `shell` 探针依赖目标系统 shell；Windows 走 `cmd /C`，非 Windows 走 `sh -c`。
- agent 上报使用 Bearer Token；若启用 token，server 与 agent 配置必须一致。
- Web UI 通过 `/api/v1/status` 获取数据，改动 API 结构时要同步前端脚本。
- 当前 README 很简短；不要把大量实现文档复制进此文件，优先保持本文件简洁、面向 AI 代理执行。
