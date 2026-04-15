# chatlog_with_sns

这是一个基于 `chatlog` 演进的微信聊天记录工具分支，当前重点补齐了微信 4.x 场景下的本地检索能力，包含聊天记录、联系人、最近会话、朋友圈、微信收藏，以及 MCP 接入。

当前仓库这版已经接通：

- 联系人多标签匹配
- `/api/v1/contact` 标签过滤
- MCP `query_contact` 标签过滤
- `/api/v1/sns` 朋友圈检索
- `/api/v1/favorites` 微信收藏检索
- HTTP 控制台中的“朋友圈检索”“微信收藏”页面

## 功能概览

- 支持微信 4.x 数据解密与本地查询
- 支持 Windows / macOS
- 支持聊天记录、联系人、群聊、最近会话检索
- 支持朋友圈 `sns.db` 自动加载与检索
- 支持微信收藏 `favorite.db` 自动加载与检索
- 支持联系人标签解析，可按多个标签做 `all` / `any` 匹配
- 支持本地 HTTP API
- 支持 Streamable HTTP MCP
- 支持图片、语音等多媒体访问
- 支持 TUI 与命令行两种使用方式

## 这版新增点

### 1. 联系人多标签匹配

`/api/v1/contact` 现在支持：

- `tags=投资人,AI圈`
- `tag_mode=all|any`

含义：

- `all`: 必须同时命中全部标签
- `any`: 命中任一标签即可

MCP 工具 `query_contact` 也同步支持这两个参数。

### 2. 微信收藏检索

新增 `GET /api/v1/favorites`，可按收藏类型、关键词、分页和格式输出检索微信收藏。

已支持的 `type` 过滤值：

- `text`
- `image`
- `video`
- `article`
- `location`
- `file`
- `chat`
- `note`
- `card`
- `finder`

### 3. 朋友圈昵称回退匹配

`GET /api/v1/sns` 查询 `username` 时，除了先按发布者 ID 精确匹配，还会在解析后的朋友圈 XML 昵称上做大小写无关回退匹配。像下面这种请求可以直接返回结果：

```text
/api/v1/sns?username=HEXIN&limit=100&format=json
```

## 快速开始

### 1. 编译

```bash
cd /path/to/chatlog_with_sns
go build -o chatlog .
```

如果你直接用源码运行，也可以：

```bash
go run main.go
```

### 2. 启动 TUI

```bash
./chatlog
```

在界面中可以执行：

- 获取密钥
- 解密数据
- 启动 HTTP 服务
- 开启自动解密

### 3. 直接启动 HTTP 服务

如果你已经有数据目录和密钥配置，也可以直接命令行启动：

```bash
go run main.go server -a 127.0.0.1:5030 -d /path/to/wechat/data
```

启动后浏览器可访问：

- `http://127.0.0.1:5030`

如果页面打不开，先确认服务进程确实已经启动，而且 `5030` 端口没有被别的进程占用。

## HTTP 控制台

首页就是本地控制台，默认地址：

- `http://127.0.0.1:5030`

页面中可直接使用：

- 聊天记录检索
- 联系人检索
- 群聊检索
- 最近会话检索
- 朋友圈检索
- 微信收藏检索
- 数据库浏览和 SQL 查询

## HTTP API

服务默认前缀：

```text
http://127.0.0.1:5030/api/v1
```

### 聊天记录

```text
GET /api/v1/chatlog?time=2026-04-01~2026-04-15&talker=张三&format=json
```

常用参数：

- `time`
- `talker`
- `sender`
- `keyword`
- `limit`
- `offset`
- `format=json|csv|xlsx|chatlab|text`

### 联系人

```text
GET /api/v1/contact?keyword=张三&tags=投资人,AI圈&tag_mode=all&format=json
```

常用参数：

- `keyword`
- `tags`
- `tag_mode=all|any`
- `limit`
- `offset`
- `format=json|csv|xlsx|text`

### 群聊

```text
GET /api/v1/chatroom?keyword=项目群&format=json
```

### 最近会话

```text
GET /api/v1/session?keyword=张三&format=json
```

### 朋友圈

```text
GET /api/v1/sns?username=HEXIN&limit=100&format=json
```

常用参数：

- `username`
- `limit`
- `offset`
- `format=json|csv|raw|text`

说明：

- 优先按发布者 ID 精确匹配
- 未命中时回退按解析出的昵称匹配

### 微信收藏

```text
GET /api/v1/favorites?type=article&keyword=AI&limit=50&format=json
```

常用参数：

- `type`
- `keyword`
- `limit`
- `offset`
- `format=json|csv|raw|text`

## MCP

MCP Streamable HTTP 入口：

```text
http://127.0.0.1:5030/mcp
```

当前这版和本次改动直接相关的 MCP 能力：

- `query_contact`
- `query_chat_room`
- `query_recent_chat`
- `query_chat_log`

其中 `query_contact` 现已支持：

- `keyword`
- `tags`
- `tag_mode`

示例含义：

- `tags="投资人,AI圈", tag_mode="all"`：同时属于“投资人”和“AI圈”
- `tags="投资人,AI圈", tag_mode="any"`：属于任一标签即可

## 常见接口示例

### 查带多个标签的联系人

```text
GET /api/v1/contact?tags=投资人,AI圈&tag_mode=all&format=json
```

### 查某个昵称对应的朋友圈

```text
GET /api/v1/sns?username=HEXIN&limit=100&format=json
```

### 查微信收藏里的文章

```text
GET /api/v1/favorites?type=article&format=text
```

### 打开 MCP

在支持 Streamable HTTP MCP 的客户端里填入：

```text
http://127.0.0.1:5030/mcp
```

## 注意事项

- 当前重点支持微信 4.x
- 所有数据处理都在本地完成
- HTTP 控制台是本地服务，不对公网暴露时更安全
- 如果使用命令行 `server` 模式，建议显式传入 `-a` 和 `-d`

## 免责声明

请仅处理你自己合法拥有，或已获得明确授权的数据。

禁止将本项目用于任何未授权的数据获取、检索、分析或传播行为。使用本项目造成的任何后果，由使用者自行承担。

完整说明见 [DISCLAIMER.md](./DISCLAIMER.md)。

## License

[Apache-2.0](./LICENSE)
