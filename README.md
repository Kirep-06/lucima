# lucima

lucima 是一个用于本地开发和测试分布式系统网络行为的 Go 库。它在单机上建立数据中心、机架、主机三级虚拟拓扑，通过 API 调整节点状态、延迟和连接可用性。项目也提供代理服务，让其他语言编写的程序使用同一套模拟网络。

## 安装

需要 Go 1.24 或更新版本。在 Go 项目中添加依赖：

```sh
go get github.com/Kirep-06/lucima
```

## 运行 API 服务

在仓库目录运行：

```sh
go run ./lucimaserver
```

默认监听 `localhost:8989`。可以用 `-port=8990` 指定其他端口。启动时加载默认拓扑，其中有 3 个数据中心、9 个机架和 27 台主机。

例如，将 `animal.land.lion` 标记为对外不可达：

```sh
curl -X POST 'http://localhost:8989/nodeState?name=animal.land.lion' \
  -H 'Content-Type: application/json' \
  -d '{"ExternalDown":true}'
```

查询节点状态：

```sh
curl 'http://localhost:8989/nodeState?name=animal.land.lion'
```

`Latency` 的单位是纳秒。更新节点状态时若传入的 `Latency` 为零，该节点保留当前延迟。

## 在 Go 程序中使用

运行 [HTTP 示例](examples/basichttp/basichttp.go)：

```sh
go run ./examples/basichttp
```

示例在本地启动 API 服务和 HTTP 服务，将 HTTP 客户端的拨号交给 `lucima.NewDialFunc`，然后输出请求耗时。默认拓扑下，示例请求的模拟延迟约为 888 毫秒。

若应用已有 `net.Listener`，可以通过 `lucima.NewListener` 登记监听端口；也可以直接使用 `lucima.Listen`。客户端拨号时使用 `lucima.NewDialFunc`，连接会按拓扑状态模拟延迟和失败。

## 代理服务

代理把客户端视为指定的虚拟主机，再将流量转发到原始服务。例如，原始服务监听 `localhost:12345` 时，可创建监听 `8586` 的代理：

```sh
curl -X POST 'http://localhost:8989/proxy?clientName=matter.metal.gold&proxyName=animal.air.eagle&proxyPort=8586&originAddr=localhost:12345'
```

随后连接 `localhost:8586`。代理可通过 `PUT /proxy` 更新客户端主机名，通过 `DELETE /proxy` 关闭。API 还提供 `/config`、`/serverPort`、`/clientPort`、`/dialState` 和 `/connState`。
