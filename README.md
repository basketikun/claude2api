<h1 align="center">Claude2API</h1>

<p align="center">Claude2API 是一个基于 Go 开发的 Claude.ai API 兼容网关、账号池与网页镜像服务。项目通过统一管理多个 Claude.ai 账号并自动轮询可用账号，将 Claude 模型能力以 OpenAI API 和 Anthropic API 兼容接口的形式提供给第三方客户端、自动化脚本与开发工具。Claude2API 支持 OpenAI Chat Completions、OpenAI Responses 和 Anthropic Messages 接口，兼容 Claude Code、Codex CLI 以及支持自定义 Base URL 的应用；同时提供流式输出、多轮对话、System Prompt、多模态图片输入、扩展思考（Thinking）、Function Calling / Tool Use、长上下文处理、会话清理、API Key 鉴权、调用日志和可视化后台管理。项目支持 Docker Compose 一键部署，适合个人学习、接口适配、客户端联调和 Claude API 集成测试。</p>

> [!WARNING]
> 免责声明：
>
> 本项目涉及对 Claude.ai 官网相关能力的逆向研究，仅供个人学习、技术研究与非商业性技术交流使用。
>
> - 严禁将本项目用于任何商业用途、盈利性使用、批量操作、自动化滥用或规模化调用。
> - 严禁将本项目用于破坏市场秩序、恶意竞争、套利倒卖、二次售卖相关服务，以及任何违反 Anthropic 服务条款或当地法律法规的行为。
> - 严禁将本项目用于生成、传播或协助生成违法、暴力、色情、未成年人相关内容，或用于诈骗、欺诈、骚扰等非法或不当用途。
> - 使用者应自行承担全部风险，包括但不限于账号受限、临时封禁、永久封禁以及因违规使用导致的法律责任。
> - 本项目依赖 Claude.ai 上游接口，上游接口、风控策略及页面结构的变化都可能导致部分功能失效。
> - 使用本项目即视为你已充分理解并同意本免责声明；请勿使用重要账号、常用账号或高价值账号进行测试。

## 快速开始

Docker Compose 一键部署

```bash
git clone https://github.com/basketikun/claude2api.git
cd claude2api
cp config.example.yaml config.yaml
docker compose up -d
```

## 核心功能

- **官网镜像**：提供 Claude.ai 官网镜像和账号池选择页面，可随机或指定可用账号进入镜像站，并自动维护访问所需的 Cookie、浏览器指纹与会话信息。
- **号池管理**：支持批量导入 Claude.ai 账号，自动获取邮箱和组织信息，并提供账号状态查看、手动刷新、失效清理与定期巡检。
- **账号轮询**：API 请求自动轮询可用账号，请求失败时可按配置换号重试，避免单个账号异常影响服务。
- **OpenAI 兼容**：支持 `POST /v1/chat/completions` 和 `POST /v1/responses` 接口，可接入 OpenAI SDK 及支持自定义 Base URL 的客户端。
- **Anthropic 兼容**：支持 `POST /v1/messages` 接口，可接入 Anthropic SDK、Claude Code 等兼容客户端。
- **开发工具接入**：支持配置 Codex CLI、Claude Code 使用本服务的兼容 API。
- **模型列表**：提供 `GET /v1/models` 接口，统一返回当前支持的基础模型及 Thinking 模型。
- **流式响应**：OpenAI 与 Anthropic 接口均支持流式和非流式输出。
- **多轮对话**：支持 System Prompt、user / assistant 历史消息以及多轮上下文拼接。
- **多模态输入**：支持 OpenAI 与 Anthropic 格式的 Base64 图片输入，并自动上传至 Claude.ai。
- **扩展思考**：模型名添加 `-thinking` 后缀即可启用扩展思考模式。
- **工具调用**：支持 OpenAI `tools / tool_calls`、Responses `function_call` 和 Anthropic `tools / tool_use`，兼容流式、非流式及工具结果回传。
- **长上下文处理**：提示词超过配置阈值时自动转换为文本附件，减少超长上下文直接提交造成的问题。
- **会话清理**：支持请求完成后自动删除 Claude.ai 上游会话。
- **密钥管理**：可在后台创建、查看和删除多个 API 密钥，用于号池、镜像和 API 接口鉴权。
- **在线测试**：后台内置多轮对话测试页面，支持选择模型、流式输出和图片上传。
- **调用日志**：记录调用时间、接口、模型、使用账号、响应状态、输入输出 Token、总耗时、首字延迟和 TPS。
- **日志详情**：可选保存完整请求与响应内容，支持查看详情、批量删除及仅保留最近指定数量的日志。
- **运行时配置**：可在后台调整出口代理、重试次数、长上下文阈值、会话清理、账号巡检和详细日志等设置。

## 账号用量

账号列表、详情和号池卡片显示最近记录的 5 小时、7 天已用比例及重置时间，时间按浏览器所在时区展示。数据来自对话的 `message_limit` 事件，通过本服务的 API 或官网镜像发送对话时采集，不额外发送消息查询额度。刷新列表只能读取最近记录；从未收到用量事件时显示“暂无记录”，超过重置时间后保留原值并提示待更新。

字段依据 2026-10-29 用户从 `POST /api/organizations/{组织}/chat_conversations/{会话}/completion` 捕获的[真实响应](internal/service/testdata/message_limit.json)：读取 `message_limit.windows` 中的 `utilization`（0.16 表示已用 16%）和 `resets_at`（Unix 秒）。这份响应的顶层 `resetsAt` 为 `null`，不用于重置时间展示。

已有数据库可在升级启动前执行一次迁移：

```bash
sqlite3 data/app.db < migrations/20261029_add_account_usage.sql
```

启动时的 AutoMigrate 也会补齐这两个字段；已由启动补齐的数据库不要重复执行 SQL。迁移脚本保留到确认线上升级完成。

## Token 用量记录

2026-10-29 实测普通对话、启用思考模式的对话及会话详情，均未返回输入、输出、缓存或思考 Token 计数。保留的[真实对话流](internal/service/testdata/completion.sse)包含模型、文本事件、额度窗口和结束事件，不能从额度百分比反推 Token 数。

系统按可见文本的 UTF-8 字节数除以 4 向下取整，分别估算输入、输出和已返回的思考文本。输入包含本次发送的文本附件；输出包含文本及工具参数；不计图片、缓存、隐藏思考、未发送的历史上下文和上游内部提示。此估算不是 Claude 分词器结果，也不是计费用量。兼容接口返回的 `usage` 同样是文本估算。

API 调用和镜像对话的统计独立保存在 `usage_records`，按账号、模型和调用来源关联；删除或清理调用日志不会删除用量记录。API 调用的记录范围是一轮公开请求，镜像的记录范围是一轮 completion 请求，不回填升级前未记录的历史。账号详情显示全时段累计用量。

管理查询：`GET /api/usage?days=30&timezone_offset=-480&group=model&offset=0&limit=10`，沿用管理鉴权。`days=0` 为全部，正数从当地当天零点回溯指定天数；`timezone_offset` 使用浏览器 `getTimezoneOffset()` 的分钟值；`group` 为 `account` 或 `model`，可传 `account` 精确筛选邮箱。返回汇总、每日趋势、分组分页、来源分布及近 180 天活跃度，明确标注 `token_source: estimated`，`cache_tokens: null` 表示未提供。

已有数据库执行 `migrations/20261029_add_token_statistics.sql`，或由启动 AutoMigrate 补齐，不能重复执行 SQL。本地已执行的迁移仍保留，供其他环境升级。详细日志保留文本和参数，Base64 媒体内容替换为类型及长度信息。

## 效果展示

<table width="100%">
  <tr>
    <td width="50%"><img src="https://i.ibb.co/3yMvBm0R/image.png" alt="image" border="0"></td>
    <td width="50%"><img src="https://i.ibb.co/zhpt12gG/image.png" alt="image" border="0"></td>
  </tr>
  <tr>
    <td width="50%"><img src="https://i.ibb.co/XZ4G85yp/image.png" alt="image" border="0"></td>
    <td width="50%"><img src="https://i.ibb.co/LhxVLkYN/image.png" alt="image" border="0"></td>
  </tr>
  <tr>
    <td width="50%"><img src="https://i.ibb.co/99v19T70/image.png" alt="image" border="0"></td>
    <td width="50%"><img src="https://i.ibb.co/ZpYzPkb9/image.png" alt="image" border="0"></td>
  </tr>
  <tr>
    <td width="50%"><img src="https://i.ibb.co/1Y0C0Nyq/image.png" alt="image" border="0"></td>
    <td width="50%"><img src="https://i.ibb.co/rK773GJR/image.png" alt="image" border="0"></td>
  </tr>
</table>

## 账号导入

进入管理后台的「账号管理」，点击「批量导入」，每行填写一个 `sessionKey`：

```text
sk-ant-sid01-xxxxxxxx
sk-ant-sid01-yyyyyyyy
```

导入任务会在后台执行，并自动查询账号信息。`sessionKey` 等同于账号登录凭据，请妥善保管。

### 支持的模型

`GET /v1/models` 汇总可用账号最近查询到的模型，不再使用固定名单。来源是 Claude.ai `GET /api/account` 的 `memberships[].organization.claude_ai_bootstrap_models_config`：过滤 `inactive: true`，仅为 `paprika_modes` 包含 `extended` 的模型提供 `-thinking` 版本。字段依据 2026-10-29 捕获的[真实模型配置](internal/service/testdata/account_models.json)。

导入账号、手动刷新账号和定期巡检都会更新模型记录。升级后先刷新已有账号；没有模型记录时返回空列表，不使用旧名单兜底。调用按模型选择对应账号，实际可用性仍取决于账号权限及当前限制。

已有数据库的模型字段迁移为 `migrations/20261029_add_account_models.sql`；启动时 AutoMigrate 也会补齐字段，两种方式选择一种，不能重复执行 SQL。

<details>
<summary><code>GET /v1/models</code></summary>
<br>

获取当前服务暴露的模型列表：

```bash
curl http://localhost:8787/v1/models \
  -H "Authorization: Bearer <api-key>"
```

```json
{
  "object": "list",
  "data": [
    {
      "id": "claude-sonnet-4-6",
      "object": "model",
      "created": 1750000000,
      "owned_by": "anthropic"
    }
  ]
}
```

</details>

<details>
<summary><code>POST /v1/chat/completions</code></summary>
<br>

OpenAI Chat Completions 兼容接口，支持多轮对话、图片输入以及流式输出。图片使用 Data URI：

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-6",
    "messages": [
      {
        "role": "system",
        "content": "回答尽量简短"
      },
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "描述这张图片"},
          {
            "type": "image_url",
            "image_url": {"url": "data:image/png;base64,iVBORw0KGgo..."}
          }
        ]
      }
    ],
    "stream": true
  }'
```

</details>

<details>
<summary><code>POST /v1/responses</code></summary>
<br>

OpenAI Responses API 兼容接口，`input` 支持字符串或消息数组：

```bash
curl http://localhost:8787/v1/responses \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-6",
    "instructions": "回答尽量简短",
    "input": [
      {
        "role": "user",
        "content": [
          {"type": "input_text", "text": "描述这张图片"},
          {
            "type": "input_image",
            "image_url": "data:image/png;base64,iVBORw0KGgo..."
          }
        ]
      }
    ],
    "stream": false
  }'
```

</details>

<details>
<summary><code>POST /v1/messages</code></summary>
<br>

Anthropic Messages 兼容接口，可用于接入支持自定义 Anthropic Base URL 的客户端：

```bash
curl http://localhost:8787/v1/messages \
  -H "x-api-key: <api-key>" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 1024,
    "system": "回答尽量简短",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "text", "text": "描述这张图片"},
          {
            "type": "image",
            "source": {
              "type": "base64",
              "media_type": "image/png",
              "data": "iVBORw0KGgo..."
            }
          }
        ]
      }
    ],
    "stream": true
  }'
```

</details>

### 客户端接入

Codex CLI 可通过自定义 OpenAI Base URL 使用 `/v1/responses`，Claude Code 可通过自定义 Anthropic Base URL 使用 `/v1/messages`；两者均填写本服务地址和后台创建的 API Key 即可。

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=basketikun/claude2api&type=Date)](https://www.star-history.com/#basketikun/claude2api&Date)

## 社区支持

学 AI，上 L 站：[Linux.do](https://linux.do)

交流群：点击链接加入群聊【开源无限画布(2群)】：https://qm.qq.com/q/Shgvco7XEu
