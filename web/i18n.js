"use strict";

/*
 * Runtime UI translation (zh -> en). The HTML/JS sources stay Chinese; this file
 * rewrites text nodes and a few attributes in English mode and restores the
 * original text when switching back. No build step, no dependencies.
 */
(function () {
  const STORAGE_KEY = "claude2api_lang";
  const ATTRS = ["placeholder", "title", "aria-label", "alt"];
  const CJK = /[\u3000-\u303f\u3400-\u9fff\uff00-\uffef]/;
  const SKIP_TEXT = "script,style,textarea,pre,[translate=no],.log-message-text,.log-response,.log-nav-item em";
  const SKIP_ATTR = "script,style,[translate=no]";
  const STRICT = ".chat-text";
  const TS = ".ts-wrapper,.ts-dropdown";

  // ---------------------------------------------------------------- exact
  const EXACT = {
    // login
    "登录": "Sign in",
    "输入密码或访问密钥": "Enter your password or access key",
    "访问凭证": "Credential",
    "请输入密码或访问密钥": "Enter your password or access key",
    "继续": "Continue",
    "Claude2API 首页": "Claude2API home",
    "登录失败": "Sign-in failed",
    "未登录": "Not signed in",
    // navigation
    "数据概览": "Overview",
    "号池选择": "Account Pool",
    "账号管理": "Accounts",
    "密钥管理": "API Keys",
    "API 文档": "API Docs",
    "对话测试": "Chat Test",
    "调用日志": "Call Logs",
    "系统管理": "Settings",
    "查看版本更新记录": "View release notes",
    "读取版本…": "Loading version…",
    "退出登录": "Sign out",
    "选择账号": "Select account",
    // overview
    "统计时间": "Time range",
    "今天": "Today",
    "7 天": "7 days",
    "30 天": "30 days",
    "90 天": "90 days",
    "全部": "All",
    "刷新": "Refresh",
    "总量 = 输入 + 回答与工具 + 可见思考，均为文本估算，不代表计费用量。未返回思考文本不代表没有思考消耗；图片、缓存及网页对话历史未计入。":
      "Total = input + answer & tools + visible thinking. All figures are text-based estimates and do not represent billed usage. No returned thinking text does not mean no thinking was consumed; images, cache and web chat history are not counted.",
    "用量趋势": "Usage trend",
    "估算 Token · 请求数": "Estimated tokens · Requests",
    "每日估算 Token 和请求数": "Daily estimated tokens and requests",
    "暂无对话记录": "No conversations yet",
    "调用来源": "Call sources",
    "用量分组": "Usage grouping",
    "模型用量": "Model usage",
    "账号用量": "Account usage",
    "估算 Token": "Estimated tokens",
    "模型": "Model",
    "请求": "Requests",
    "成功率": "Success rate",
    "输入": "Input",
    "输出": "Output",
    "回答与工具": "Output & tools",
    "可见思考": "Visible thinking",
    "总量": "Total",
    "加载中…": "Loading…",
    "每页": "Per page",
    "用量列表每页条数": "Usage rows per page",
    "请求活跃度": "Request activity",
    "近 180 天": "Last 180 days",
    "少": "Less",
    "多": "More",
    "账号数": "Accounts",
    "请求数": "Requests",
    "总 Token · 估算": "Total tokens · estimated",
    "可见思考 · 估算": "Visible thinking · estimated",
    "平均首字": "Avg. first token",
    "缓存用量未提供": "Cache usage not provided",
    "正在读取用量…": "Loading usage…",
    "次请求": "requests",
    "Chat 对话": "Chat",
    "Responses 对话": "Responses",
    "Claude 对话": "Claude",
    "网页对话": "Web chat",
    "未分配账号": "Unassigned account",
    "点击复制": "Click to copy",
    "输入估算": "Input (est.)",
    "回答与工具估算": "Output & tools (est.)",
    "可见思考估算": "Visible thinking (est.)",
    "图表加载失败，请刷新页面": "Failed to load the chart, please refresh the page",
    "已复制": "Copied",
    "复制失败，请手动复制": "Copy failed, please copy manually",
    "复制失败，请手动选择": "Copy failed, please select the text manually",
    "已复制密钥": "Key copied",
    "复制": "Copy",
    // account pool / accounts
    "随机账号": "Random account",
    "一键刷新": "Refresh all",
    "批量导入": "Bulk import",
    "删除失效": "Delete expired",
    "正在刷新账号": "Refreshing accounts",
    "套餐可用模型": "Models available on plan",
    "正常": "Active",
    "已失效": "Expired",
    "失效": "Expired",
    "错误": "Error",
    "未查询": "Not checked",
    "搜索邮箱 / 组织…": "Search email / organization…",
    "邮箱": "Email",
    "状态": "Status",
    "套餐": "Plan",
    "5 小时用量": "5-hour usage",
    "7 天用量": "7-day usage",
    "创建日期": "Created",
    "已存活": "Age",
    "上次刷新": "Last refresh",
    "操作": "Actions",
    "账号列表每页条数": "Accounts per page",
    "暂无记录": "No data",
    "待更新": "Pending update",
    "已到重置时间，待更新": "Reset time passed, pending update",
    "已到重置时间，等待新对话更新用量": "reset time passed, waiting for a new conversation to update usage",
    "暂无账号": "No accounts",
    "没有符合条件的账号": "No matching accounts",
    "详情": "Details",
    "删除": "Delete",
    "上一页": "Previous",
    "下一页": "Next",
    "暂无套餐可用模型，请先刷新账号": "No models available for this plan yet. Refresh the accounts first.",
    "请输入 sessionKey": "Enter a sessionKey",
    "导入完成": "Import complete",
    "导入中…": "Importing…",
    "开始导入": "Start import",
    "刷新中…": "Refreshing…",
    "刷新完成": "Refresh complete",
    "当前没有可刷新的账号": "No accounts to refresh",
    "当前没有失效账号": "No expired accounts",
    "删除账号": "Delete account",
    "一键删除失效账号": "Delete all expired accounts",
    "组织 UUID": "Organization UUID",
    "请刷新账号": "Refresh the account",
    "模型记录时间": "Models recorded at",
    "用量记录时间": "Usage recorded at",
    "创建时间": "Created",
    "更新时间": "Updated",
    "累计对话": "Total conversations",
    "输入 Token（估算）": "Input tokens (estimated)",
    "回答与工具（估算）": "Output & tools (estimated)",
    "可见思考（估算）": "Visible thinking (estimated)",
    "缓存用量": "Cache usage",
    "未提供": "Not provided",
    "累计用量": "Total usage",
    "账号详情": "Account details",
    "关闭账号详情": "Close account details",
    "号池暂无可用账号": "No accounts available in the pool",
    "当前": "Current",
    "进入使用": "Open",
    "订阅套餐": "Subscription plan",
    "（加载失败）": "(failed to load)",
    "请输入有效的访问密钥": "Enter a valid access key",
    "请输入密钥": "Enter a key",
    "密钥无效": "Invalid key",
    "访问密钥": "Access key",
    "请输入号池访问密钥后继续": "Enter the pool access key to continue",
    "Bearer 密钥": "Bearer key",
    "确定": "OK",
    "批量导入 sessionKey": "Bulk import sessionKeys",
    "关闭导入窗口": "Close import dialog",
    "每行一个 sessionKey，导入时会自动查询邮箱和组织 UUID。": "One sessionKey per line. The email and organization UUID are looked up automatically during import.",
    "正在导入账号": "Importing accounts",
    "每行粘贴一个 sessionKey": "Paste one sessionKey per line",
    // keys
    "号池访问密钥": "Pool access keys",
    "前端号池页面需在请求头带": "Accessing the pool page requires this request header:",
    "才能访问；管理员密码也可用于后台联调和号池访问。": ". The admin password can also be used for admin testing and pool access.",
    "备注名称（可选）": "Label (optional)",
    "自定义密钥，留空则随机生成": "Custom key (leave empty to generate one)",
    "新建密钥": "Create key",
    "备注": "Label",
    "密钥": "Key",
    "暂无密钥，未创建时号池不校验": "No keys yet. The pool does not check keys until one is created.",
    "密钥已创建": "Key created",
    "密钥已删除": "Key deleted",
    "确认删除该密钥？删除后使用它的用户将无法访问号池。": "Delete this key? Anyone using it will lose access to the pool.",
    "删除密钥": "Delete key",
    // api docs
    "接口目录": "Endpoints",
    "快速开始": "Quick start",
    "统一接口说明": "Unified API overview",
    "兼容 OpenAI、Anthropic 客户端，服务会自动轮询号池账号并在失败时切换。": "Compatible with OpenAI and Anthropic clients. The service rotates through pool accounts automatically and fails over on errors.",
    "OpenAI 鉴权": "OpenAI auth",
    "Anthropic 鉴权": "Anthropic auth",
    "模型支持": "Append the",
    "后缀开启扩展思考，例如": "suffix to a model name to enable extended thinking, for example",
    "。": ".",
    "OpenAI Chat Completions 兼容接口，支持流式输出。": "OpenAI Chat Completions compatible endpoint with streaming support.",
    "请求参数": "Request parameters",
    "模型名称": "Model name",
    "角色与内容数组": "Array of roles and content",
    "是否以 SSE 流式返回": "Whether to stream the response via SSE",
    "采样温度": "Sampling temperature",
    "响应": "Response",
    "OpenAI Responses API 兼容接口，适用于新一代 OpenAI SDK。": "OpenAI Responses API compatible endpoint for the latest OpenAI SDKs.",
    "用户输入内容": "User input",
    "是否流式返回": "Whether to stream the response",
    "Anthropic Messages 兼容接口，可直接复用 Claude SDK。": "Anthropic Messages compatible endpoint that works directly with the Claude SDK.",
    "最大输出 token 数": "Maximum output tokens",
    "对话消息数组": "Array of conversation messages",
    "获取当前服务可用的模型列表。": "List the models available on this service.",
    "无": "None",
    "仅需携带鉴权请求头": "Only the auth header is required",
    // chat test
    "支持多轮对话、流式输出和图片上传": "Multi-turn chat, streaming output and image upload",
    "开始一段新的对话吧": "Start a new conversation",
    "输入消息，按 Enter 发送，Shift + Enter 换行…": "Type a message. Enter to send, Shift + Enter for a new line…",
    "选择模型": "Select a model",
    "加载模型中…": "Loading models…",
    "图片": "Image",
    "清空": "Clear",
    "发送": "Send",
    "暂无可用模型": "No models available",
    "加载失败": "Failed to load",
    "没有匹配的模型": "No matching models",
    "请先选择可用模型": "Select an available model first",
    "请输入测试消息": "Enter a test message",
    "请求中…": "Requesting…",
    "你": "You",
    "上传的图片": "Uploaded image",
    "请求图片": "Request image",
    // logs
    "删除选中": "Delete selected",
    "仅保留最近": "Keep only the latest",
    "条": "entries",
    "清理": "Prune",
    "全选本页": "Select all on this page",
    "时间": "Time",
    "账号": "Account",
    "响应方式": "Response mode",
    "每秒输出 Token": "Output tokens per second",
    "状态码": "Status code",
    "输入/输出": "In / Out",
    "耗时": "Duration",
    "调用日志每页条数": "Logs per page",
    "← 调用日志": "← Call logs",
    "调用详情": "Call details",
    "暂无调用日志": "No call logs",
    "未分配": "Unassigned",
    "流式": "Streaming",
    "非流式": "Non-streaming",
    "成功": "Success",
    "失败": "Failed",
    "首字": "First token",
    "总计": "Total",
    "查看详情": "View details",
    "保留条数必须是非负整数": "The keep count must be a non-negative integer",
    "删除日志": "Delete logs",
    "清理日志": "Prune logs",
    "结果": "Result",
    "性能": "Performance",
    "未记录": "Not recorded",
    "用量来源": "Usage source",
    "可见文本估算": "Estimated from visible text",
    "请求错误": "Request error",
    "请求消息": "Request messages",
    "模型输出": "Model output",
    "未记录详细内容": "Detailed content not recorded",
    "可在系统管理中开启“记录详细请求日志”。": "You can enable “Log detailed requests” in Settings.",
    // settings
    "基础配置": "Basic settings",
    "出口代理": "Outbound proxy",
    "API 配置": "API settings",
    "换号重试次数": "Account-switch retries",
    "0 = 自动": "0 = auto",
    "超长上下文阈值（字符）": "Long-context threshold (characters)",
    "用完自动删会话": "Delete conversation after use",
    "是": "Yes",
    "否": "No",
    "状态监测": "Status monitoring",
    "后台会定时查询每个账号，更新账号状态并处理已失效账号。": "The backend periodically checks every account, updates its status and handles expired accounts.",
    "刷新间隔（秒）": "Refresh interval (seconds)",
    "21600（6 小时）": "21600 (6 hours)",
    "会话失效时移除账号": "Remove account when its session expires",
    "记录详细请求日志": "Log detailed requests",
    "否，仅记录指标": "No, metrics only",
    "是，记录请求与输出": "Yes, log requests and outputs",
    "保存设置": "Save settings",
    "保存后即时生效，无需重启": "Takes effect immediately after saving, no restart needed",
    "设置已保存，即时生效": "Settings saved and applied",
    // version dialog
    "版本更新": "Release notes",
    "关闭版本更新": "Close release notes",
    "当前版本": "Current version",
    "读取中…": "Loading…",
    "最新版本": "Latest version",
    "检查中…": "Checking…",
    "版本更新记录": "Release history",
    "正在读取更新记录…": "Loading release history…",
    "前往GitHub": "Go to GitHub",
    "有更新": "Update available",
    "未发现更新": "Up to date",
    "检查失败": "Check failed",
    "暂不可用": "Unavailable",
    "读取失败": "Failed to load",
    "版本未知": "Version unknown",
    "版本信息暂时无法读取，点击查看更新记录": "Version info is unavailable right now. Click to view release notes.",
    "更新记录暂时无法读取，请关闭后重试，或前往 GitHub 查看。": "Release notes are unavailable right now. Close this dialog and try again, or check GitHub.",
    "更新记录": "Release notes",
    "新增": "Added",
    "调整": "Changed",
    "修复": "Fixed",
    "增加用量统计、账号套餐和额度进度展示。": "Added usage statistics, account plans and quota progress display.",
    "支持版本检查和更新记录查看。": "Added version checking and release notes viewing.",
    "优化调用日志、模型选择和账号卡片样式。": "Improved call logs, model selection and account card styling.",
    "修正用量估算和套餐可用模型判断。": "Fixed usage estimation and plan model detection.",
    "提供 Claude 网页镜像、账号池和密钥管理。": "Claude web mirror, account pool and key management.",
    "兼容 OpenAI 与 Anthropic 调用，支持图片、思考和工具调用。": "OpenAI and Anthropic compatible calls, with image, thinking and tool-call support.",
    // dialogs
    "确认操作": "Confirm",
    "取消": "Cancel",
    "确认删除": "Delete",
    // backend errors (Go handlers, middleware, services)
    "请求格式错误": "Invalid request format",
    "没有可导入的 sessionKey": "No sessionKey to import",
    "查询账号信息失败": "Failed to look up account info",
    "sessionKey 已失效或格式错误": "sessionKey is expired or malformed",
    "保存账号失败": "Failed to save the account",
    "账号不存在或缺少 sessionKey": "Account does not exist or has no sessionKey",
    "日志不存在": "Log not found",
    "保留条数不能小于 0": "The keep count cannot be less than 0",
    "请选择要删除的日志": "Select the logs to delete",
    "密码或密钥错误": "Incorrect password or key",
    "请选择统计范围和分组": "Select a time range and a grouping",
    "用量读取失败，请稍后重试": "Failed to load usage, please try again later",
    "当前版本信息无法读取": "The current version info cannot be read",
    "未登录或密码错误": "Not signed in or wrong password",
    "无效的 API 密钥": "Invalid API key",
    "无法读取对话内容": "Unable to read the conversation content",
    "对话内容格式错误": "Invalid conversation content format",
    "messages 不能为空": "messages must not be empty",
    "input 不能为空": "input must not be empty",
    "会话响应缺少 uuid": "Conversation response is missing a uuid",
    "请求失败": "Request failed",
    "图片 Data URI 格式错误": "Invalid image Data URI format",
    "图片重定向次数过多": "Too many image redirects",
    "图片 URL 无效": "Invalid image URL",
    "图片 URL 不允许访问内网地址": "Image URLs must not point to internal network addresses",
    "无效 Base64": "Invalid Base64",
    "不支持的图片格式": "Unsupported image format",
  };

  // Same Chinese text, different English depending on where it appears.
  const CONTEXT = [
    ["#stats", "账号", "Accounts"],
    ["#stats", "失效", "Expired"],
    [".log-meta-item small", "请求", "Request"],
  ];

  // --------------------------------------------------------- error prefixes
  // "<prefix>: <detail>" where the detail is translated recursively.
  const PREFIXES = {
    "加载账号失败": "Failed to load accounts",
    "加载失败": "Failed to load",
    "导入失败": "Import failed",
    "刷新失败": "Refresh failed",
    "一键刷新失败": "Refresh all failed",
    "删除失败": "Delete failed",
    "加载配置失败": "Failed to load settings",
    "保存设置失败": "Failed to save settings",
    "加载模型失败": "Failed to load models",
    "创建密钥失败": "Failed to create key",
    "删除密钥失败": "Failed to delete key",
    "删除日志失败": "Failed to delete logs",
    "清理日志失败": "Failed to prune logs",
    "加载日志详情失败": "Failed to load log details",
    "请求失败": "Request failed",
    "读取失败": "Failed to load",
    "网络错误": "Network error",
    "切换账号失败": "Failed to switch account",
    "已复制模型": "Copied model",
    "无效请求体": "Invalid request body",
    "图片处理失败": "Image processing failed",
    "图片上传失败": "Image upload failed",
  };

  // ----------------------------------------------------------------- regex
  const N = "(\\d[\\d,]*)";
  const pl = (count, word) => `${count} ${word}${count === "1" ? "" : "s"}`;
  const REGEX = [
    [/^(\d+(?:\.\d+)?)(万|亿)$/, (m) => compactNumber(m[1], m[2])],
    [/^已用 (.+)$/, (m) => `Used ${m[1]}`],
    [/^(.+?)\s*重置(?:，(.+))?$/, (m) => `Resets ${m[1]}${m[2] ? ", " + t(m[2]) : ""}`],
    [/^记录于 (.+)$/, (m) => `Recorded at ${m[1]}`],
    [/^(5 小时|7 天)已用 (.+?)(，已到重置时间，待更新)?$/, (m) => `${m[1] === "5 小时" ? "5-hour" : "7-day"} usage ${m[2]}${m[3] ? ", reset time passed, pending update" : ""}`],
    [/^(string|array|boolean|number|integer|string \/ array) · (必填|可选)$/, (m) => `${m[1]} · ${m[2] === "必填" ? "required" : "optional"}`],
    [/^(Authorization: Bearer <|x-api-key: <)(号池密钥|密钥)>$/, (m) => `${m[1]}${m[2] === "密钥" ? "key" : "pool key"}>`],
    [/^留空直连，如 (.+)$/, (m) => `Leave empty for a direct connection, e.g. ${m[1]}`],
    [new RegExp(`^${N} 个正常账号$`), (m) => `${pl(m[1], "active account")}`],
    [/^成功率 (.+?) · (.+) 估算 Token$/, (m) => `Success rate ${m[1]} · ${t(m[2])} estimated tokens`],
    [/^成功率 (.+)$/, (m) => `Success rate ${m[1]}`],
    [/^(.+) 成功率$/, (m) => `${m[1]} success rate`],
    [/^输入 (.+) · 输出 (.+)$/, (m) => `Input ${t(m[1])} · Output ${t(m[2])}`],
    [new RegExp(`^${N} 个有输出的请求$`), (m) => `${pl(m[1], "request")} with output`],
    [/^更新于 (.+)$/, (m) => `Updated ${m[1]}`],
    [/^(.+) · (\d[\d,]*) 次$/, (m) => `${t(m[1])} · ${pl(m[2], "call")}`],
    [/^(.+) · (\d[\d,]*) 次请求$/, (m) => `${m[1]} · ${pl(m[2], "request")}`],
    [/^(.+)，(\d[\d,]*) 次请求$/, (m) => `${m[1]}, ${pl(m[2], "request")}`],
    [/^(.+) · 点击复制$/, (m) => `${m[1]} · click to copy`],
    [/^(\d+) \/ (\d+) 页 · 共 (.+) 条$/, (m) => `Page ${m[1]} / ${m[2]} · ${m[3]} total`],
    [/^共 (.+) 条 · (\d+)\/(\d+) 页$/, (m) => `${m[1]} total · page ${m[2]}/${m[3]}`],
    [/^(\d+)\/(\d+) 页$/, (m) => `Page ${m[1]}/${m[2]}`],
    [/^共 (.+) 条$/, (m) => `${m[1]} total`],
    [/^共 (.+) 次调用$/, (m) => `${pl(m[1], "call")}`],
    [/^（共 (\d+) 个）$/, (m) => `(${m[1]} total)`],
    [/^账号 #(\d+)$/, (m) => `Account #${m[1]}`],
    [/^成功 (\d+)，失败 (\d+)$/, (m) => `${m[1]} succeeded, ${m[2]} failed`],
    [/^导入完成：成功 (\d+)，失败 (\d+)$/, (m) => `Import complete: ${m[1]} succeeded, ${m[2]} failed`],
    [/^完成，失败 (\d+)$/, (m) => `Done, ${m[1]} failed`],
    [/^刷新完成：成功 (\d+)，失败 (\d+)$/, (m) => `Refresh complete: ${m[1]} succeeded, ${m[2]} failed`],
    [/^已刷新 (\d+) 个账号$/, (m) => `Refreshed ${pl(m[1], "account")}`],
    [/^已移除 (.+)$/, (m) => `Removed ${m[1]}`],
    [/^已更新 (.+)$/, (m) => `Updated ${m[1]}`],
    [/^确认删除账号 (.+?) ？$/, (m) => `Delete account ${m[1]}?`],
    [/^确认删除全部 (\d+) 个失效账号？此操作不可撤销。$/, (m) => `Delete all ${pl(m[1], "expired account")}? This cannot be undone.`],
    [/^已删除 (\d+) 个失效账号$/, (m) => `Deleted ${pl(m[1], "expired account")}`],
    [/^确认删除选中的 (\d+) 条日志？$/, (m) => `Delete the ${pl(m[1], "selected log")}?`],
    [/^删除选中（(\d+)）$/, (m) => `Delete selected (${m[1]})`],
    [/^确认仅保留最近 (\d+) 条日志？$/, (m) => `Keep only the latest ${pl(m[1], "log")} and delete the rest?`],
    [/^已删除 (\d+) 条日志$/, (m) => `Deleted ${pl(m[1], "log")}`],
    [/^已清理 (\d+) 条日志$/, (m) => `Pruned ${pl(m[1], "log")}`],
    [/^已删除 (.+)$/, (m) => `Deleted ${m[1]}`],
    [/^选择日志 (\d+)$/, (m) => `Select log ${m[1]}`],
    [/^查看调用 (\d+) 详情$/, (m) => `View details of call ${m[1]}`],
    [/^耗时 (\d+) ms$/, (m) => `Took ${m[1]} ms`],
    [/^消息目录 · (\d+)$/, (m) => `Messages · ${m[1]}`],
    [/^(\d+) 个工具定义$/, (m) => `${pl(m[1], "tool definition")}`],
    [/^(\d+) 估算 Token$/, (m) => `${m[1]} estimated tokens`],
    [/^(.+) 估算 Token$/, (m) => `${t(m[1])} estimated tokens`],
    [/^(.+) 首字$/, (m) => `${m[1]} first token`],
    [/^(.+) TPS · (.+) 总耗时$/, (m) => `${m[1]} TPS · ${m[2]} total`],
    [/^(成功|失败) · (HTTP )?(.+)$/, (m) => `${m[1] === "成功" ? "Success" : "Failed"} · ${m[2] || ""}${m[3]}`],
    [/^(\d+) 天 (\d+) 小时$/, (m) => `${m[1]}d ${m[2]}h`],
    [/^(\d+) 小时 (\d+) 分钟$/, (m) => `${m[1]}h ${m[2]}m`],
    [/^(\d+) 分钟$/, (m) => `${m[1]} min`],
    [/^(\d+) 秒$/, (m) => `${m[1]} s`],
    [/^有新版本 (.+)$/, (m) => `New version ${m[1]} available`],
    [/^当前 (.+?)，(暂时无法检查更新|可更新至 (.+?)|未发现新版本)(，点击查看更新记录)?$/, (m) => {
      const detail = m[2] === "暂时无法检查更新" ? "unable to check for updates" : m[2] === "未发现新版本" ? "no new version found" : `update to ${m[3]} available`;
      return `Current ${m[1]}, ${detail}${m[4] ? ", click to view release notes" : ""}`;
    }],
    [/^查看版本更新记录，(.+)$/, (m) => `View release notes, ${t(m[1])}`],
    [/^Claude2API · (.+)$/, (m) => `Claude2API · ${t(m[1])}`],
    [/^(.+) · Claude2API$/, (m) => `${t(m[1])} · Claude2API`],
    [/^没有可使用模型 (.+) 的账号，请刷新账号模型记录$/, (m) => `No account can use model ${m[1]}. Refresh the account model records.`],
    [/^图片大小必须在 1 字节到 (\d+) MB 之间$/, (m) => `Image size must be between 1 byte and ${m[1]} MB`],
    [/^\[错误\] (.+)$/, (m) => `[Error] ${t(m[1])}`],
  ];

  // Fragments for composed backend errors, e.g. "创建会话 HTTP 403: {...}".
  const PHRASES = [
    ["网页会话初始化", "Web session initialization"],
    ["构造请求体失败", "Failed to build request body"],
    ["构造请求失败", "Failed to build request"],
    ["查询账号信息失败", "Failed to look up account info"],
    ["查询账号信息", "Account info lookup"],
    ["读取响应体失败", "Failed to read response body"],
    ["解析账号信息失败", "Failed to parse account info"],
    ["图片 Base64 无效", "Invalid image Base64"],
    ["构造上传表单失败", "Failed to build upload form"],
    ["写入上传内容失败", "Failed to write upload content"],
    ["上传文件失败", "File upload failed"],
    ["上传文件", "File upload"],
    ["更新设置失败", "Failed to update settings"],
    ["更新设置", "Settings update"],
    ["创建会话失败", "Failed to create conversation"],
    ["创建会话", "Create conversation"],
    ["解析会话响应失败", "Failed to parse conversation response"],
    ["会话响应缺少 uuid", "Conversation response is missing a uuid"],
    ["发送消息失败", "Failed to send message"],
    ["发送消息", "Send message"],
    ["读取错误响应失败", "failed to read the error response"],
    ["GitHub VERSION 不是语义化版本", "GitHub VERSION is not a semantic version"],
    ["GitHub VERSION 解析失败", "Failed to parse GitHub VERSION"],
    ["创建数据目录失败", "Failed to create data directory"],
    ["打开数据库失败", "Failed to open database"],
    ["建表失败", "Failed to create tables"],
    ["类型", "type"],
    ["图片上传失败", "Image upload failed"],
    ["流式输出中断", "Stream interrupted"],
    ["请求失败", "Request failed"],
    ["下载图片失败", "Failed to download image"],
    ["下载图片", "Image download"],
    ["读取图片失败", "Failed to read image"],
    ["解析图片域名失败", "Failed to resolve image host"],
    ["图片处理失败", "Image processing failed"],
    ["无效请求体", "Invalid request body"],
    ["不能为空", "must not be empty"],
    ["会话失效", "session expired"],
    ["估算 Token", "estimated tokens"],
    ["媒体内容未保存", "media content not saved"],
    ["图片内容未保存", "image content not saved"],
    ["Base64 字符数", "Base64 length"],
    ["字符数", "characters"],
    ["账号", "account"],
    ["错误", "error"],
    ["失败", "failed"],
  ].sort((a, b) => b[0].length - a[0].length);

  const PUNCTUATION = [
    ["：", ": "], ["，", ", "], ["；", "; "], ["、", ", "], ["。", ". "], ["（", " ("], ["）", ")"], ["？", "?"], ["！", "!"],
  ];

  // ------------------------------------------------------------ translation
  const exact = new Map(Object.entries(EXACT));
  const reverse = new Map();
  const duplicates = new Set();
  for (const [zh, en] of exact) {
    if (reverse.has(en)) duplicates.add(en);
    else reverse.set(en, zh);
  }
  for (const en of duplicates) reverse.delete(en);

  const compactEn = new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 });
  function compactNumber(value, unit) {
    return compactEn.format(parseFloat(value) * (unit === "万" ? 1e4 : 1e8));
  }

  function lookup(core, element, strict) {
    for (const [selector, zh, en] of CONTEXT) {
      if (zh === core && element && element.closest(selector)) return en;
    }
    if (!strict) {
      if (exact.has(core)) return exact.get(core);
      for (const [pattern, build] of REGEX) {
        const m = pattern.exec(core);
        if (m) return build(m);
      }
    }
    const prefix = /^([^:：]+)\s*[:：]\s*([\s\S]*)$/.exec(core);
    if (prefix && PREFIXES[prefix[1].trim()]) {
      const detail = prefix[2].trim();
      return detail ? `${PREFIXES[prefix[1].trim()]}: ${t(detail)}` : PREFIXES[prefix[1].trim()];
    }
    if (strict) return null;
    const collapsed = core.replace(/\s+/g, " ");
    if (collapsed !== core && exact.has(collapsed)) return exact.get(collapsed);
    if (core.includes("\n")) {
      const lines = core.split("\n").map((line) => translateText(line, element, false));
      if (lines.some((line, i) => line !== core.split("\n")[i])) return lines.join("\n");
    }
    let out = core;
    for (const [zh, en] of PHRASES) out = out.split(zh).join(en);
    for (const [zh, en] of PUNCTUATION) out = out.split(zh).join(en);
    out = out.replace(/(\d)\s*(万|亿)/g, (_, v, u) => compactNumber(v, u)).replace(/ {2,}/g, " ").replace(/\s+([.,;:)])/g, "$1").trim();
    return out !== core ? out : null;
  }

  function translateText(raw, element, strict) {
    if (!CJK.test(raw)) return raw;
    const parts = /^(\s*)([\s\S]*?)(\s*)$/.exec(raw);
    const out = lookup(parts[2], element, strict);
    if (out == null) return raw;
    const lead = /^[.,;:!?)]/.test(out) ? "" : parts[1];
    return lead + out + parts[3];
  }

  // ------------------------------------------------------------------ state
  let lang = "en";
  const textStore = new WeakMap();
  const attrStore = new WeakMap();
  let observer = null;
  const nativeDialogs = { alert: window.alert, confirm: window.confirm, prompt: window.prompt };

  function detectLanguage() {
    let saved = null;
    try { saved = localStorage.getItem(STORAGE_KEY); } catch { /* storage unavailable */ }
    if (saved === "en" || saved === "zh") return saved;
    return /^zh/i.test(navigator.language || "") ? "zh" : "en";
  }

  function t(text) {
    if (lang !== "en" || typeof text !== "string") return text;
    return translateText(text, null, false);
  }

  function inSelector(element, selector) {
    return element && element.closest ? element.closest(selector) : null;
  }

  function processText(node) {
    const parent = node.parentElement;
    if (!parent || inSelector(parent, SKIP_TEXT)) return;
    const current = node.nodeValue;
    const record = textStore.get(node);
    if (lang === "en") {
      if (record && record.en === current) return;
      if (CJK.test(current)) {
        const out = translateText(current, parent, !!inSelector(parent, STRICT));
        if (out !== current) {
          textStore.set(node, { zh: current, en: out });
          node.nodeValue = out;
        } else textStore.delete(node);
        return;
      }
      textStore.delete(node);
    }
    // Tom Select copies option text into its own DOM/state; recover the Chinese source.
    const key = current.trim();
    if (reverse.has(key) && inSelector(parent, TS)) {
      const zh = reverse.get(key);
      if (lang === "zh") node.nodeValue = current.replace(key, zh);
      else textStore.set(node, { zh: current.replace(key, zh), en: current });
    }
  }

  function processAttribute(element, name) {
    if (!element.hasAttribute(name) || inSelector(element, SKIP_ATTR)) return;
    const current = element.getAttribute(name);
    const records = attrStore.get(element) || {};
    const record = records[name];
    if (lang === "en") {
      if (record && record.en === current) return;
      if (CJK.test(current)) {
        const out = translateText(current, element, false);
        if (out !== current) {
          records[name] = { zh: current, en: out };
          attrStore.set(element, records);
          element.setAttribute(name, out);
        } else delete records[name];
        return;
      }
      delete records[name];
    }
    const key = current.trim();
    if (reverse.has(key) && inSelector(element, TS)) {
      const zh = reverse.get(key);
      if (lang === "zh") element.setAttribute(name, zh);
      else {
        records[name] = { zh, en: current };
        attrStore.set(element, records);
      }
    }
  }

  function processTree(root) {
    if (root.nodeType === Node.TEXT_NODE) return processText(root);
    if (root.nodeType !== Node.ELEMENT_NODE) return;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
    let node = root;
    do {
      if (node.nodeType === Node.TEXT_NODE) processText(node);
      else for (const name of ATTRS) processAttribute(node, name);
    } while ((node = walker.nextNode()));
  }

  function restoreTree(root) {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT);
    let node = root;
    do {
      if (node.nodeType === Node.TEXT_NODE) {
        const record = textStore.get(node);
        if (record && record.en === node.nodeValue) node.nodeValue = record.zh;
        textStore.delete(node);
        processText(node);
      } else {
        const records = attrStore.get(node);
        if (records) {
          for (const name of Object.keys(records)) {
            if (node.getAttribute(name) === records[name].en) node.setAttribute(name, records[name].zh);
          }
          attrStore.delete(node);
        }
        for (const name of ATTRS) processAttribute(node, name);
      }
    } while ((node = walker.nextNode()));
  }

  function onMutations(records) {
    for (const record of records) {
      if (record.type === "characterData") processText(record.target);
      else if (record.type === "attributes") processAttribute(record.target, record.attributeName);
      else for (const added of record.addedNodes) processTree(added);
    }
    observer.takeRecords();
  }

  function startObserver() {
    if (!observer) observer = new MutationObserver(onMutations);
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true,
      attributeFilter: ATTRS,
    });
  }

  function syncToggles() {
    document.documentElement.lang = lang === "en" ? "en" : "zh-CN";
    for (const button of document.querySelectorAll("[data-lang]")) {
      button.setAttribute("aria-pressed", String(button.dataset.lang === lang));
    }
  }

  function apply() {
    if (observer) observer.disconnect();
    if (lang === "en") processTree(document.documentElement);
    else restoreTree(document.documentElement);
    syncToggles();
    startObserver();
  }

  function setLang(next) {
    if (next !== "en" && next !== "zh") return;
    const changed = next !== lang;
    lang = next;
    try { localStorage.setItem(STORAGE_KEY, lang); } catch { /* storage unavailable */ }
    apply();
    if (changed) document.dispatchEvent(new CustomEvent("claude2api:langchange", { detail: { lang } }));
  }

  // ----------------------------------------------------------------- public
  window.i18n = { t, getLang: () => lang, setLang, translate: (text) => translateText(String(text), null, false) };

  window.alert = (message) => nativeDialogs.alert.call(window, message === undefined ? message : t(String(message)));
  window.confirm = (message) => nativeDialogs.confirm.call(window, message === undefined ? message : t(String(message)));
  window.prompt = (message, value) => nativeDialogs.prompt.call(window, message === undefined ? message : t(String(message)), value);

  lang = detectLanguage();
  document.documentElement.lang = lang === "en" ? "en" : "zh-CN";

  // Hide the page until the first translation to avoid a flash of Chinese text.
  const hider = document.createElement("style");
  if (lang === "en") {
    hider.textContent = "html[data-i18n-pending] body{visibility:hidden}";
    document.head.appendChild(hider);
    document.documentElement.setAttribute("data-i18n-pending", "");
  }
  const reveal = () => {
    document.documentElement.removeAttribute("data-i18n-pending");
    hider.remove();
  };

  function init() {
    document.addEventListener("click", (event) => {
      const button = event.target.closest && event.target.closest("[data-lang]");
      if (button) setLang(button.dataset.lang);
    });
    apply();
    reveal();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
    setTimeout(reveal, 3000);
  } else init();
})();
