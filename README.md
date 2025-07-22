# golang-qdrant

## 项目简介
本项目演示如何用 Go 语言通过 gRPC 方式接入 [Qdrant](https://qdrant.tech/) 向量数据库，支持向量的插入、检索等基本操作，并提供简单的 HTTP 检索接口。

## 主要功能
- Qdrant 客户端封装（qdrantUtils/）
- 向量批量/单条插入
- 向量相似度检索
- HTTP 检索接口（/search）

## 依赖
- Go 1.18 及以上
- Qdrant 服务端（建议 1.7+，需开启 gRPC，默认端口 6334）
- 主要依赖包：
  - github.com/qdrant/go-client
  - google.golang.org/grpc
  - github.com/webws/go-moda/logger

## 快速开始
1. **拉取依赖**
   ```bash
   go mod tidy
   ```
2. **启动 Qdrant 服务**
   - 默认 gRPC 地址：`10.100.2.1:6334`（可在 `cmd/main.go` 修改）
3. **运行项目**
   ```bash
   go run cmd/main.go
   ```
4. **访问 HTTP 检索接口**
   - GET `http://localhost:8080/search?term=xxx`
   - 当前示例向量为全0（需自行接入真实 embedding）

## 目录结构
- `cmd/main.go`         —— HTTP 服务入口，调用 Qdrant 封装
- `qdrantUtils/`        —— Qdrant 客户端封装，常用操作方法
- `go.mod/go.sum`       —— Go 依赖管理

## Qdrant 配置说明
- 请确保 Qdrant gRPC 端口（默认 6334）可访问
- collection 名称、向量维度等参数需与实际数据一致
- 如需自定义 embedding，请在 main.go 里补充向量化逻辑

## 常见问题
- **依赖拉取失败**：请确认 go-client 版本为 v0.3.2 或最新可用版本
- **端口不通**：请检查 Qdrant 服务端地址与端口
- **向量维度不符**：collection 创建时 size 参数需与实际 embedding 维度一致

---
如有问题欢迎提 issue 或交流！ 