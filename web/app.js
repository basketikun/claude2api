"use strict";

function esc(s) {
  return String(s ?? "").replace(
    /[&<>"]/g,
    (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c],
  );
}

const AUTH_TOKEN_KEY = "claude2api_credential";
const AUTH_ROLE_KEY = "claude2api_role";
let authToken = localStorage.getItem(AUTH_TOKEN_KEY) || "";
let authRole = localStorage.getItem(AUTH_ROLE_KEY) || "";

function saveAuth(token, role) {
  authToken = token;
  authRole = role;
  localStorage.setItem(AUTH_TOKEN_KEY, token);
  localStorage.setItem(AUTH_ROLE_KEY, role);
}

function clearAuth() {
  authToken = authRole = "";
  localStorage.removeItem(AUTH_TOKEN_KEY);
  localStorage.removeItem(AUTH_ROLE_KEY);
}

function initAdmin(initialPage = "accounts", currentRole = "admin") {
  const adminRoot = document.querySelector(".view-admin");
  const $ = (sel) => adminRoot.querySelector(sel);

  let ACCOUNTS = [];

let filterStatus = "all";
let searchKw = "";
let curPage = 1;
let pageSize = 10;

const CONFIG_FIELDS = [
  "proxy",
  "retry_count",
  "max_chat_history_length",
  "chat_delete",
  "detailed_api_log",
  "status_check_interval_seconds",
  "remove_invalid_account",
];

const CONFIG_DEFAULTS = {
  retry_count: 0,
  max_chat_history_length: 12000,
  chat_delete: true,
  detailed_api_log: false,
  status_check_interval_seconds: 21600,
  remove_invalid_account: false,
};

async function api(path, method = "GET", body = null, stream = false) {
  const opt = { method, headers: {} };
  if (authToken) opt.headers.Authorization = "Bearer " + authToken;
  if (body) {
    opt.headers["Content-Type"] = "application/json";
    opt.body = JSON.stringify(body);
  }
  const res = await fetch(path, opt);
  if (res.status === 401) {
    clearAuth();
    window.location.href = "/";
    throw new Error("未登录");
  }
  if (stream && res.ok) return res;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

let msgTimer = null;

function showMsg(text, kind = "") {
  const el = $("#msg");
  if (msgTimer) {
    clearTimeout(msgTimer);
    msgTimer = null;
  }
  el.textContent = text;
  el.className = "msg" + (kind ? " " + kind : "");
  if (!text) {
    el.classList.add("hidden");
    return;
  }

  msgTimer = setTimeout(
    () => {
      el.classList.add("hidden");
      msgTimer = null;
    },
    kind === "err" ? 4500 : 3000,
  );
}

function confirmDialog(body, { title = "确认操作", okText = "确认删除" } = {}) {
  return new Promise((resolve) => {
    const mask = $("#confirm-mask");
    $("#confirm-title").textContent = title;
    $("#confirm-body").textContent = body;
    $("#confirm-ok").textContent = okText;
    mask.classList.remove("hidden");

    const cleanup = (result) => {
      mask.classList.add("hidden");
      $("#confirm-ok").removeEventListener("click", onOk);
      $("#confirm-cancel").removeEventListener("click", onCancel);
      mask.removeEventListener("click", onMask);
      document.removeEventListener("keydown", onKey);
      resolve(result);
    };
    const onOk = () => cleanup(true);
    const onCancel = () => cleanup(false);
    const onMask = (e) => {
      if (e.target === mask) cleanup(false);
    };
    const onKey = (e) => {
      if (e.key === "Escape") cleanup(false);
      else if (e.key === "Enter") cleanup(true);
    };

    $("#confirm-ok").addEventListener("click", onOk);
    $("#confirm-cancel").addEventListener("click", onCancel);
    mask.addEventListener("click", onMask);
    document.addEventListener("keydown", onKey);
    $("#confirm-ok").focus();
  });
}

function statusOf(a) {
  return a.status || "unknown";
}

function statusCounts() {
  const c = {
    all: ACCOUNTS.length,
    active: 0,
    expired: 0,
    error: 0,
    unknown: 0,
  };
  for (const a of ACCOUNTS) {
    const st = statusOf(a);
    if (c[st] === undefined) c.unknown++;
    else if (st !== "all") c[st]++;
  }
  return c;
}

function renderStats() {
  const c = statusCounts();
  $("#stats").innerHTML =
    `<span>账号 <b>${c.all}</b></span>` +
    `<span>正常 <b>${c.active}</b></span>` +
    `<span>失效 <b>${c.expired}</b></span>` +
    `<span>错误 <b>${c.error}</b></span>`;

  for (const k of ["all", "active", "expired", "error", "unknown"]) {
    const el = $("#cnt-" + k);
    if (el) el.textContent = c[k];
  }
}

function statusBadge(status) {
  status = status || "unknown";
  const label =
    {
      active: "正常",
      expired: "已失效",
      error: "错误",
      unknown: "未查询",
    }[status] || status;
  return `<span class="badge ${status}">${esc(label)}</span>`;
}

function fmtTime(t) {
  if (!t) return "";
  const d = new Date(t),
    pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

function fmtSurvived(t) {
  if (!t) return "—";
  const ms = Date.parse(t);
  if (isNaN(ms)) return "—";
  let sec = Math.floor((Date.now() - ms) / 1000);
  if (sec < 0) sec = 0;
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分钟`;
  if (m > 0) return `${m} 分钟`;
  return `${sec} 秒`;
}

function filteredAccounts() {
  const kw = searchKw.trim().toLowerCase();
  return ACCOUNTS.filter((a) => {
    if (filterStatus !== "all" && statusOf(a) !== filterStatus) return false;
    if (kw) {
      const hay = String(a.email || "").toLowerCase();
      if (!hay.includes(kw)) return false;
    }
    return true;
  });
}

function renderTable() {
  const body = $("#acc-body");
  const rows = filteredAccounts();

  if (!ACCOUNTS.length) {
    body.innerHTML = `<tr><td colspan="7" class="empty">暂无账号</td></tr>`;
    $("#pager").innerHTML = "";
    return;
  }
  if (!rows.length) {
    body.innerHTML = `<tr><td colspan="7" class="empty">没有符合条件的账号</td></tr>`;
    $("#pager").innerHTML = "";
    return;
  }

  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  if (curPage > totalPages) curPage = totalPages;
  const start = (curPage - 1) * pageSize;
  const pageRows = rows.slice(start, start + pageSize);

  body.innerHTML = pageRows
    .map((a, i) => `<tr data-email="${esc(a.email)}">
      <td>${start + i + 1}</td>
      <td class="mono">${esc(a.email)}</td>
      <td>${statusBadge(a.status)}</td>
      <td class="mono">${esc(fmtTime(a.created_at) || "—")}</td>
      <td>${esc(fmtSurvived(a.created_at))}</td>
      <td class="mono">${esc(fmtTime(a.updated_at) || "—")}</td>
      <td>
        <button class="btn-sm act-refresh">刷新</button>
        <button class="btn-sm act-detail">详情</button>
        <button class="btn-sm btn-danger act-del">删除</button>
      </td>
    </tr>`)
    .join("");

  renderPager(rows.length, totalPages);
}

function renderPager(total, totalPages) {
  const pager = $("#pager");
  if (totalPages <= 1) {
    pager.innerHTML = `<span class="pager-info">共 ${total} 条</span>`;
    return;
  }
  const btn = (label, page, disabled, active) =>
    `<button class="page-btn${active ? " active" : ""}" data-page="${page}"${disabled ? " disabled" : ""}>${label}</button>`;

  const pages = [];
  const add = (p) => {
    if (!pages.includes(p) && p >= 1 && p <= totalPages) pages.push(p);
  };
  add(1);
  add(2);
  for (let p = curPage - 1; p <= curPage + 1; p++) add(p);
  add(totalPages - 1);
  add(totalPages);
  pages.sort((a, b) => a - b);

  let html = btn("上一页", curPage - 1, curPage === 1, false);
  let prev = 0;
  for (const p of pages) {
    if (prev && p - prev > 1) html += `<span class="page-gap">…</span>`;
    html += btn(p, p, false, p === curPage);
    prev = p;
  }
  html += btn("下一页", curPage + 1, curPage === totalPages, false);
  html += `<span class="pager-info">共 ${total} 条 · ${curPage}/${totalPages} 页</span>`;
  pager.innerHTML = html;
}

function rerender() {
  renderStats();
  renderTable();
}

async function loadAccounts() {
  try {
    const data = await api("/api/accounts");
    ACCOUNTS = data.accounts || [];
    rerender();
  } catch (e) {
    showMsg("加载账号失败: " + e.message, "err");
  }
}

async function fetchModels() {
  const res = await fetch("/v1/models", {
    headers: {
      Authorization: "Bearer " + authToken,
    },
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error?.message || `HTTP ${res.status}`);
  return data.data || [];
}

function modelNames(models) {
  return [...new Set(models.map((m) => m.id).filter(Boolean))];
}

async function loadAccountModels() {
  const box = $("#account-models");
  box.textContent = "加载中…";
  try {
    const names = modelNames(await fetchModels());
    box.innerHTML = names.length
      ? names
          .map(
            (name) =>
              `<button type="button" class="model-pill" data-model="${esc(name)}">${esc(name)}</button>`,
          )
          .join("")
      : '<span class="model-empty">暂无可用模型</span>';
  } catch (e) {
    box.innerHTML = `<span class="model-error">加载失败：${esc(e.message)}</span>`;
  }
}

function refreshAccountsPage() {
  loadAccounts();
  loadAccountModels();
}

function setProgress(id, completed, total, text = `${completed}/${total}`) {
  const box = $(id);
  box.classList.remove("hidden");
  box.querySelector("progress").max = total;
  box.querySelector("progress").value = completed;
  box.querySelector(".task-progress-text").textContent = text;
}

async function importAccounts() {
  const text = $("#import-keys").value;
  if (!text.trim()) return showMsg("请输入 sessionKey", "err");
  const btn = $("#btn-import-submit");
  btn.disabled = true;
  try {
    const r = await api("/api/accounts/import", "POST", { session_keys: text }, true);
    const reader = r.body.getReader(), decoder = new TextDecoder();
    let buffer = "", result;
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      const events = buffer.split("\n\n");
      buffer = events.pop();
      for (const event of events) {
        const line = event.split("\n").find((line) => line.startsWith("data:"));
        if (!line) continue;
        result = JSON.parse(line.slice(5));
        btn.textContent = result.done ? "导入完成" : "导入中…";
        setProgress("#import-progress", result.completed ?? result.total, result.total, `成功 ${result.imported}，失败 ${result.failed}`);
      }
      if (done) break;
    }
    $("#import-keys").value = "";
    $("#import-mask").classList.add("hidden");
    await loadAccounts();
    showMsg(`导入完成：成功 ${result.imported}，失败 ${result.failed}`, result.failed ? "err" : "ok");
  } catch (e) {
    showMsg("导入失败: " + e.message, "err");
  } finally {
    btn.disabled = false;
    btn.textContent = "开始导入";
  }
}

async function refreshAccount(email, button) {
  button.disabled = true;
  button.textContent = "刷新中…";
  try {
    const r = await api("/api/accounts/refresh", "POST", { email });
    await loadAccounts();
    showMsg(r.removed ? `已移除 ${email}` : `已更新 ${email}`, "ok");
  } catch (e) {
    showMsg("刷新失败: " + e.message, "err");
  } finally {
    button.disabled = false;
    button.textContent = "刷新";
  }
}

async function refreshAllAccounts() {
  const btn = $("#btn-refresh-all"), count = ACCOUNTS.length;
  if (!count) return showMsg("当前没有可刷新的账号", "ok");
  btn.disabled = true;
  btn.textContent = "刷新中…";
  let completed = 0;
  setProgress("#refresh-all-progress", 0, count);
  try {
    const results = await Promise.allSettled(ACCOUNTS.map(async (a) => {
      try { return await api("/api/accounts/refresh", "POST", { email: a.email }); }
      finally { setProgress("#refresh-all-progress", ++completed, count); }
    }));
    const failed = results.filter((r) => r.status === "rejected").length;
    setProgress("#refresh-all-progress", count, count, failed ? `完成，失败 ${failed}` : "刷新完成");
    await loadAccounts();
    showMsg(failed ? `刷新完成：成功 ${count - failed}，失败 ${failed}` : `已刷新 ${count} 个账号`, failed ? "err" : "ok");
  } catch (e) {
    showMsg("一键刷新失败: " + e.message, "err");
  } finally {
    btn.disabled = false;
    btn.textContent = "一键刷新";
  }
}

async function deleteOne(email) {
  const ok = await confirmDialog(`确认删除账号 ${email} ？`, {
    title: "删除账号",
  });
  if (!ok) return;
  try {
    await api("/api/delete", "POST", { email });
    await loadAccounts();
    showMsg(`已删除 ${email}`, "ok");
  } catch (e) {
    showMsg("删除失败: " + e.message, "err");
  }
}

async function deleteExpired() {
  const n = statusCounts().expired;
  if (!n) {
    showMsg("当前没有失效账号", "ok");
    return;
  }
  const ok = await confirmDialog(
    `确认删除全部 ${n} 个失效账号？此操作不可撤销。`,
    { title: "一键删除失效账号" },
  );
  if (!ok) return;
  const btn = $("#btn-del-expired");
  btn.disabled = true;
  try {
    const data = await api("/api/delete-expired", "POST", {});
    await loadAccounts();
    showMsg(`已删除 ${data.removed} 个失效账号`, "ok");
  } catch (e) {
    showMsg("删除失败: " + e.message, "err");
  } finally {
    btn.disabled = false;
  }
}

function openDetail(email) {
  const a = ACCOUNTS.find((x) => x.email === email);
  if (!a) return;
  const rows = [
    ["邮箱", a.email],
    [
      "状态",
      { active: "正常", expired: "已失效", error: "错误", unknown: "未查询" }[
        a.status || "unknown"
      ] || a.status,
    ],
    ["组织 UUID", a.org_uuid || "—"],
    ["创建时间", fmtTime(a.created_at) || "—"],
    ["已存活", fmtSurvived(a.created_at)],
    ["更新时间", fmtTime(a.updated_at) || "—"],
  ];

  const info = rows
    .map(
      ([k, v]) =>
        `<div class="dl-row"><span class="dl-key">${esc(k)}</span><span class="dl-val mono">${esc(v)}</span></div>`,
    )
    .join("");

  $("#detail-title").textContent = "账号详情";
  $("#detail-body").innerHTML = `<div class="detail-info">${info}</div>`;
  $("#detail-mask").classList.remove("hidden");
}

function closeDetail() {
  $("#detail-mask").classList.add("hidden");
}

async function loadConfig() {
  try {
    const data = await api("/api/config");
    const c = data.config || {};
    for (const k of CONFIG_FIELDS) {
      const el = $("#cfg-" + k);
      if (!el) continue;
      el.value = String(c[k] ?? CONFIG_DEFAULTS[k] ?? "");
    }
  } catch (e) {
    showMsg("加载配置失败: " + e.message, "err");
  }
}

async function saveConfig() {
  const patch = {};
  for (const k of CONFIG_FIELDS) {
    const el = $("#cfg-" + k);
    if (el) patch[k] = el.value;
  }
  const btn = $("#btn-cfg-save");
  btn.disabled = true;
  try {
    await api("/api/config", "POST", patch);
    showMsg("设置已保存，即时生效", "ok");
    await loadConfig();
  } catch (e) {
    showMsg("保存设置失败: " + e.message, "err");
  } finally {
    btn.disabled = false;
  }
}

let chatHistory = [];
let chatImages = [];

function renderMarkdown(value) {
  let s = esc(value || "");
  s = s.replace(
    /```([\w-]*)\n?([\s\S]*?)```/g,
    (_, lang, code) => `<pre class="md-code"><code>${code}</code></pre>`,
  );
  s = s.replace(/`([^`]+)`/g, "<code>$1</code>");
  s = s.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  s = s.replace(
    /\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g,
    '<a href="$2" target="_blank" rel="noreferrer">$1</a>',
  );
  return s.replace(/\n/g, "<br>");
}

function renderChat() {
  const box = $("#chat-messages");
  if (!chatHistory.length) {
    box.innerHTML = '<div class="chat-empty">开始一段新的对话吧</div>';
    return;
  }
  box.innerHTML = chatHistory
    .map((m) => {
      const content = Array.isArray(m.content)
        ? m.content
        : [{ type: "text", text: m.content }];
      const text = content
        .filter((x) => x.type === "text")
        .map((x) => renderMarkdown(x.text))
        .join("\n");
      const imgs = content
        .filter((x) => x.type === "image_url")
        .map(
          (x) =>
            `<img class="chat-image" src="${x.image_url.url}" alt="上传的图片">`,
        )
        .join("");
      const isUser = m.role === "user";
      return `<div class="chat-row ${isUser ? "user" : "assistant"}">
      <div class="chat-avatar">${isUser ? "你" : '<img src="/static/claude.svg" alt="Claude">'}</div>
      <div class="chat-bubble">${imgs}${text ? `<div class="chat-text">${text}</div>` : ""}</div>
    </div>`;
    })
    .join("");
  box.scrollTop = box.scrollHeight;
}

async function loadChatModels() {
  try {
    const select = $("#test-model");
    const models = await fetchModels();
    select.innerHTML = models.length
      ? models
          .map((m) => `<option value="${esc(m.id)}">${esc(m.id)}</option>`)
          .join("")
      : '<option value="">暂无可用模型</option>';
  } catch (e) {
    $("#test-model").innerHTML = '<option value="">加载失败</option>';
    showMsg("加载模型失败: " + e.message, "err");
  }
}

async function readChatImages(files) {
  for (const file of files) {
    const data = await new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = reject;
      reader.readAsDataURL(file);
    });
    chatImages.push({ name: file.name, url: data });
  }
  $("#chat-attachments").innerHTML = chatImages
    .map(
      (x, i) =>
        `<span class="chat-attachment"><img src="${x.url}" alt=""><button type="button" data-index="${i}">×</button></span>`,
    )
    .join("");
}

async function sendTestChat() {
  const model = $("#test-model").value.trim();
  const key = authToken;
  const message = $("#test-message").value.trim();
  if (!message) {
    showMsg("请输入测试消息", "err");
    return;
  }
  const btn = $("#btn-test-send");
  const hint = $("#test-hint");
  btn.disabled = true;
  hint.textContent = "请求中…";
  const content = [
    { type: "text", text: message },
    ...chatImages.map((x) => ({
      type: "image_url",
      image_url: { url: x.url },
    })),
  ];
  chatHistory.push({ role: "user", content });
  chatHistory.push({ role: "assistant", content: "" });
  $("#test-message").value = "";
  chatImages = [];
  $("#chat-attachments").innerHTML = "";
  renderChat();
  const started = performance.now();
  try {
    const res = await fetch("/v1/chat/completions", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + key,
      },
      body: JSON.stringify({
        model,
        stream: true,
        messages: chatHistory.slice(0, -1),
      }),
    });
    if (!res.ok) {
      const data = await res.json().catch(() => ({}));
      throw new Error(data.error?.message || `HTTP ${res.status}`);
    }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const events = buffer.split("\n\n");
      buffer = events.pop() || "";
      for (const event of events) {
        const line = event.split("\n").find((x) => x.startsWith("data: "));
        if (!line) continue;
        const raw = line.slice(6).trim();
        if (raw === "[DONE]") continue;
        const data = JSON.parse(raw);
        const delta = data.choices?.[0]?.delta?.content || "";
        chatHistory[chatHistory.length - 1].content += delta;
        renderChat();
      }
      if (done) break;
    }
    hint.textContent = `耗时 ${Math.round(performance.now() - started)} ms`;
  } catch (e) {
    chatHistory[chatHistory.length - 1].content = "请求失败：" + e.message;
    renderChat();
    hint.textContent = "";
  } finally {
    btn.disabled = false;
  }
}

function switchPage(name) {
  if (currentRole !== "admin" && name !== "pool") return;
  document
    .querySelectorAll(".nav-item")
    .forEach((b) => b.classList.toggle("active", b.dataset.page === (name === "log-detail" ? "logs" : name)));
  document
    .querySelectorAll(".page")
    .forEach((p) => p.classList.toggle("hidden", p.id !== "page-" + name));
  if (name === "api") $("#api-base").textContent = window.location.origin;
  if (name === "chat") loadChatModels();
  if (name === "accounts") loadAccountModels();
  if (name === "system") loadConfig();
  if (name === "keys") loadKeys();
  if (name === "logs") loadLogs();
}

async function loadKeys() {
  const body = $("#keys-body");
  try {
    const data = await api("/api/keys");
    const keys = data.keys || [];
    if (!keys.length) {
      body.innerHTML = `<tr><td colspan="5" class="empty">暂无密钥，未创建时号池不校验</td></tr>`;
      return;
    }
    body.innerHTML = keys
      .map(
        (k, i) => `
      <tr data-id="${k.id}">
        <td>${i + 1}</td>
        <td>${esc(k.name || "—")}</td>
        <td><div class="key-cell"><span class="k">${esc(k.key)}</span>
          <button class="btn-sm act-copy">复制</button></div></td>
        <td class="mono">${esc(fmtTime(k.created_at) || "—")}</td>
        <td><button class="btn-sm btn-danger act-key-del">删除</button></td>
      </tr>`,
      )
      .join("");
  } catch (e) {
    body.innerHTML = `<tr><td colspan="5" class="empty">加载失败：${esc(e.message)}</td></tr>`;
  }
}

async function createKey() {
  const name = $("#key-name").value.trim();
  const value = $("#key-value").value.trim();
  $("#btn-key-create").disabled = true;
  try {
    await api("/api/keys", "POST", { name, key: value });
    $("#key-name").value = "";
    $("#key-value").value = "";
    await loadKeys();
    showMsg("密钥已创建", "ok");
  } catch (e) {
    showMsg("创建密钥失败: " + e.message, "err");
  } finally {
    $("#btn-key-create").disabled = false;
  }
}

async function deleteKey(id) {
  const ok = await confirmDialog(
    "确认删除该密钥？删除后使用它的用户将无法访问号池。",
    { title: "删除密钥" },
  );
  if (!ok) return;
  try {
    await api("/api/keys/delete", "POST", { id: Number(id) });
    await loadKeys();
    showMsg("密钥已删除", "ok");
  } catch (e) {
    showMsg("删除密钥失败: " + e.message, "err");
  }
}

let logsPage = 1;
let logsPageSize = 10;
let logsTotal = 0;
const fmtSeconds = (ms) => ms ? `${(ms / 1000).toFixed(2)} s` : "—";

async function loadLogs() {
  const body = $("#logs-body");
  const offset = (logsPage - 1) * logsPageSize;
  try {
    const data = await api(`/api/logs?limit=${logsPageSize}&offset=${offset}`);
    const logs = data.logs || [];
    logsTotal = data.total || 0;
    const success = logs.filter((l) => l.success).length;
    const avgTPS = logs.length ? logs.reduce((n, l) => n + (l.tps || 0), 0) / logs.length : 0;
    $("#logs-stats").innerHTML = `<span><small>调用</small><b>${logsTotal}</b></span><span><small>成功</small><b>${success}/${logs.length}</b></span><span><small>TPS</small><b>${avgTPS.toFixed(1)}</b></span>`;
    if (!logs.length) {
      body.innerHTML = `<tr><td colspan="8" class="empty">暂无调用日志</td></tr>`;
      updateLogSelection();
      renderLogsPager();
      return;
    }
    body.innerHTML = logs
      .map((l) => {
        const status = l.success
          ? `<span class="log-status ok"><i></i>成功</span>`
          : `<span class="log-status err" title="${esc(l.error || "")}"><i></i>失败</span>`;
        return `<tr data-log-id="${l.id}">
        <td><input class="log-check" type="checkbox" value="${l.id}" aria-label="选择日志 ${l.id}"></td>
        <td class="log-time"><b>${esc(fmtTime(l.created_at).split(" ")[1] || "—")}</b><span>${esc(fmtTime(l.created_at).split(" ")[0] || "")}</span></td>
        <td class="log-request"><b>${esc(l.endpoint)} <em>${l.stream ? "流" : "非流"}</em></b><span class="mono">${esc(l.model || "—")}</span></td>
        <td class="log-account" title="${esc(l.account || "")}">${esc(l.account || "—")}</td>
        <td>${status}<span class="log-code">HTTP ${l.status_code || "—"}</span></td>
        <td class="log-token mono"><b>${l.input_tokens || 0}</b><i>→</i><b>${l.output_tokens || 0}</b></td>
        <td class="log-performance"><b>${fmtSeconds(l.first_token_ms)}</b><span>${Number(l.tps || 0).toFixed(1)} TPS · ${fmtSeconds(l.duration_ms)}</span></td>
        <td><button class="log-open act-log-detail" data-id="${l.id}" aria-label="查看调用 ${l.id}">查看<span>→</span></button></td>
      </tr>`;
      })
      .join("");
    updateLogSelection();
    renderLogsPager();
  } catch (e) {
    body.innerHTML = `<tr><td colspan="8" class="empty">加载失败：${esc(e.message)}</td></tr>`;
    updateLogSelection();
  }
}

function updateLogSelection() {
  const checks = [...document.querySelectorAll(".log-check")];
  const selected = checks.filter((el) => el.checked);
  $("#logs-check-all").checked = checks.length > 0 && selected.length === checks.length;
  $("#btn-logs-delete").disabled = selected.length === 0;
  $("#btn-logs-delete").textContent = selected.length ? `删除选中（${selected.length}）` : "删除选中";
}

async function deleteSelectedLogs() {
  const ids = [...document.querySelectorAll(".log-check:checked")].map((el) => Number(el.value));
  if (!ids.length || !(await confirmDialog(`确认删除选中的 ${ids.length} 条日志？`, { title: "删除日志" }))) return;
  try {
    const data = await api("/api/logs/delete", "POST", { ids });
    logsPage = 1;
    await loadLogs();
    showMsg(`已删除 ${data.removed} 条日志`, "ok");
  } catch (e) {
    showMsg("删除日志失败：" + e.message, "err");
  }
}

async function trimLogs() {
  const keep = Number($("#logs-keep").value);
  if (!Number.isInteger(keep) || keep < 0) return showMsg("保留条数必须是非负整数", "err");
  if (!(await confirmDialog(`确认仅保留最近 ${keep} 条日志？`, { title: "清理日志" }))) return;
  try {
    const data = await api("/api/logs/delete", "POST", { keep });
    logsPage = 1;
    await loadLogs();
    showMsg(`已清理 ${data.removed} 条日志`, "ok");
  } catch (e) {
    showMsg("清理日志失败：" + e.message, "err");
  }
}

async function showLogDetail(id) {
  try {
    const data = await api(`/api/logs/${id}`);
    const l = data.log;
    $("#log-detail-title").innerHTML = `调用详情 <span>#${l.id}</span>`;
    $("#log-detail-body").innerHTML = `
      <div class="log-detail-meta">
        ${logMeta("请求", l.endpoint, l.model || "—")}
        ${logMeta("账号", l.account || "—")}
        ${logMeta("结果", `${l.success ? "成功" : "失败"} · HTTP ${l.status_code || "—"}`, fmtTime(l.created_at))}
        ${logMeta("性能", `${fmtSeconds(l.first_token_ms)} 首字`, `${Number(l.tps || 0).toFixed(1)} TPS · ${fmtSeconds(l.duration_ms)} 总耗时`)}
      </div>
      ${l.error ? `<div class="log-error-box"><b>请求错误</b><span>${esc(l.error)}</span></div>` : ""}
      <section class="log-detail-section"><div class="log-section-head"><div><small>REQUEST</small><h3>请求消息</h3></div><span>${l.input_tokens || 0} tokens</span></div>${renderLogRequest(l.request)}</section>
      <section class="log-detail-section"><div class="log-section-head"><div><small>RESPONSE</small><h3>模型输出</h3></div><span>${l.output_tokens || 0} tokens</span></div>${l.response ? `<div class="log-response">${renderMarkdown(l.response)}</div>` : detailDisabled()}</section>`;
    switchPage("log-detail");
    window.scrollTo(0, 0);
  } catch (e) {
    showMsg("加载日志详情失败: " + e.message, "err");
  }
}

function logMeta(label, value, sub = "") {
  return `<div class="log-meta-item"><small>${esc(label)}</small><b>${esc(value)}</b>${sub ? `<span>${esc(sub)}</span>` : ""}</div>`;
}

function detailDisabled() {
  return `<div class="log-detail-off"><b>未记录详细内容</b><span>可在系统管理中开启“记录详细请求日志”。</span></div>`;
}

function renderLogRequest(value) {
  if (!value) return detailDisabled();
  if (typeof value === "string") {
    try { value = JSON.parse(value); } catch { return `<pre class="log-detail-text">${esc(value)}</pre>`; }
  }
  const rows = [];
  if (value.system) rows.push({ role: "system", content: value.system });
  if (value.instructions) rows.push({ role: "system", content: value.instructions });
  const input = value.messages ?? value.input;
  if (Array.isArray(input)) rows.push(...input);
  else if (input != null) rows.push({ role: "user", content: input });
  const messages = rows.map(renderLogMessage).join("");
  const nav = rows.length > 1 ? `<nav class="log-message-nav"><small>消息目录 · ${rows.length}</small>${rows.map((row, i) => `<button class="log-nav-item${i ? "" : " active"}" data-target="log-message-${i}"><b>${String(i + 1).padStart(2, "0")}</b><span>${esc(logRole(row))}</span><em>${esc(logPreview(row))}</em></button>`).join("")}</nav>` : "";
  const tools = Array.isArray(value.tools) && value.tools.length
    ? `<details class="log-tools"><summary>${value.tools.length} 个工具定义</summary><pre>${esc(JSON.stringify(value.tools, null, 2))}</pre></details>` : "";
  return messages || tools ? `<div class="${nav ? "log-request-layout" : ""}">${nav}<div class="log-messages">${messages}</div></div>${tools}` : `<pre class="log-detail-text">${esc(JSON.stringify(value, null, 2))}</pre>`;
}

function logRole(message) {
  const role = message.role || message.type || "input";
  return { system: "SYSTEM", user: "USER", assistant: "ASSISTANT", tool: "TOOL", function_call: "FUNCTION", function_call_output: "RESULT" }[role] || role.toUpperCase();
}

function logPreview(message) {
  const content = message.content ?? message.output ?? message.arguments ?? message.name ?? "";
  const text = typeof content === "string" ? content : Array.isArray(content) ? content.map((part) => part.text || part.content || part.name || part.type || "").join(" ") : JSON.stringify(content);
  return text.replace(/\s+/g, " ").trim().slice(0, 28) || "—";
}

function renderLogMessage(message, index) {
  const role = message.role || message.type || "input";
  const content = message.content ?? message.output ?? message.arguments ?? message;
  return `<article id="log-message-${index}" class="log-message role-${esc(role)}"><header><span>${esc(logRole(message))}</span>${message.name ? `<b>${esc(message.name)}</b>` : ""}${message.call_id || message.tool_call_id ? `<code>${esc(message.call_id || message.tool_call_id)}</code>` : ""}</header><div>${renderLogContent(content)}</div></article>`;
}

function renderLogContent(content) {
  if (typeof content === "string") return `<div class="log-message-text">${renderMarkdown(content)}</div>`;
  if (!Array.isArray(content)) return `<pre>${esc(JSON.stringify(content, null, 2))}</pre>`;
  return content.map((part) => {
    if (part.type === "text" || part.type === "input_text" || part.type === "output_text") return `<div class="log-message-text">${renderMarkdown(part.text || "")}</div>`;
    const url = part.image_url?.url || part.image_url || (part.type === "image" && part.source?.data ? `data:${part.source.media_type};base64,${part.source.data}` : "");
    if (url && /^data:image\//.test(url)) return `<figure class="log-image"><img src="${esc(url)}" alt="请求图片"><figcaption>${esc(part.type)}</figcaption></figure>`;
    return `<div class="log-block"><span>${esc(part.type || "block")}</span><pre>${esc(JSON.stringify(part, null, 2))}</pre></div>`;
  }).join("");
}

function renderLogsPager() {
  const pager = $("#logs-pager");
  const totalPages = Math.max(1, Math.ceil(logsTotal / logsPageSize));
  if (totalPages <= 1) {
    pager.innerHTML = `<span class="pager-info">共 ${logsTotal} 条</span>`;
    return;
  }
  const btn = (label, page, disabled, active) =>
    `<button class="page-btn${active ? " active" : ""}" data-lpage="${page}"${disabled ? " disabled" : ""}>${label}</button>`;
  const pages = [];
  const add = (p) => {
    if (!pages.includes(p) && p >= 1 && p <= totalPages) pages.push(p);
  };
  add(1);
  add(2);
  for (let p = logsPage - 1; p <= logsPage + 1; p++) add(p);
  add(totalPages - 1);
  add(totalPages);
  pages.sort((a, b) => a - b);

  let html = btn("上一页", logsPage - 1, logsPage === 1, false);
  let prev = 0;
  for (const p of pages) {
    if (prev && p - prev > 1) html += `<span class="page-gap">…</span>`;
    html += btn(p, p, false, p === logsPage);
    prev = p;
  }
  html += btn("下一页", logsPage + 1, logsPage === totalPages, false);
  html += `<span class="pager-info">共 ${logsTotal} 条 · ${logsPage}/${totalPages} 页</span>`;
  pager.innerHTML = html;
}

async function logout() {
  await fetch("/api/logout", { method: "POST" });
  clearAuth();
  window.location.href = "/";
}

$("#nav").addEventListener("click", (e) => {
  const item = e.target.closest(".nav-item");
  if (item) switchPage(item.dataset.page);
});

$("#account-models").addEventListener("click", (e) => {
  const btn = e.target.closest(".model-pill");
  if (!btn) return;
  const model = btn.dataset.model || btn.textContent.trim();
  navigator.clipboard?.writeText(model).then(
    () => showMsg(`已复制模型：${model}`, "ok"),
    () => showMsg("复制失败，请手动选择", "err"),
  );
});

$("#btn-refresh").addEventListener("click", refreshAccountsPage);
$("#btn-refresh-all").addEventListener("click", refreshAllAccounts);
$("#btn-import").addEventListener("click", () => {
  $("#import-progress").classList.add("hidden");
  $("#import-mask").classList.remove("hidden");
  $("#import-keys").focus();
});
$("#btn-import-submit").addEventListener("click", importAccounts);
for (const id of ["#btn-import-cancel", "#import-close"]) {
  $(id).addEventListener("click", () =>
    $("#import-mask").classList.add("hidden"),
  );
}
$("#import-mask").addEventListener("click", (e) => {
  if (e.target === $("#import-mask")) $("#import-mask").classList.add("hidden");
});
$("#btn-del-expired").addEventListener("click", deleteExpired);
$("#btn-cfg-save").addEventListener("click", saveConfig);
$("#btn-logout").addEventListener("click", logout);
$("#btn-key-create").addEventListener("click", createKey);
$("#btn-test-send").addEventListener("click", sendTestChat);
$("#btn-chat-clear").addEventListener("click", () => {
  chatHistory = [];
  chatImages = [];
  $("#chat-attachments").innerHTML = "";
  renderChat();
});
$("#chat-image").addEventListener("change", (e) => {
  readChatImages(e.target.files);
  e.target.value = "";
});
$("#chat-attachments").addEventListener("click", (e) => {
  const btn = e.target.closest("button[data-index]");
  if (!btn) return;
  chatImages.splice(Number(btn.dataset.index), 1);
  readChatImages([]);
});
$("#test-message").addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    sendTestChat();
  }
});
$("#test-message").addEventListener("paste", (e) => {
  const files = Array.from(e.clipboardData?.items || [])
    .filter((item) => item.kind === "file" && item.type.startsWith("image/"))
    .map((item) => item.getAsFile())
    .filter(Boolean);
  if (!files.length) return;
  e.preventDefault();
  readChatImages(files);
});
$("#btn-logs-refresh").addEventListener("click", () => {
  logsPage = 1;
  loadLogs();
});
$("#btn-logs-delete").addEventListener("click", deleteSelectedLogs);
$("#btn-logs-trim").addEventListener("click", trimLogs);
$("#logs-check-all").addEventListener("change", (e) => {
  document.querySelectorAll(".log-check").forEach((el) => (el.checked = e.target.checked));
  updateLogSelection();
});
$("#logs-page-size").addEventListener("change", (e) => {
  logsPageSize = Number(e.target.value) || 10;
  logsPage = 1;
  loadLogs();
});
$("#logs-pager").addEventListener("click", (e) => {
  const btn = e.target.closest(".page-btn");
  if (!btn || btn.disabled) return;
  const p = Number(btn.dataset.lpage);
  if (p && p !== logsPage) {
    logsPage = p;
    loadLogs();
  }
});
$("#logs-body").addEventListener("click", (e) => {
  if (e.target.classList.contains("log-check")) updateLogSelection();
  const btn = e.target.closest(".act-log-detail");
  const row = e.target.closest("tr[data-log-id]");
  if (btn || (row && !e.target.closest("input"))) showLogDetail(btn?.dataset.id || row.dataset.logId);
});
$("#log-detail-back").addEventListener("click", () => switchPage("logs"));
$("#log-detail-body").addEventListener("click", (e) => {
  const item = e.target.closest(".log-nav-item");
  if (!item) return;
  item.parentElement.querySelector(".active")?.classList.remove("active");
  item.classList.add("active");
  document.getElementById(item.dataset.target).scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "start" });
});

$("#keys-body").addEventListener("click", (e) => {
  const tr = e.target.closest("tr[data-id]");
  if (!tr) return;
  if (e.target.classList.contains("act-key-del")) deleteKey(tr.dataset.id);
  else if (e.target.classList.contains("act-copy")) {
    const key = tr.querySelector(".k")?.textContent || "";
    navigator.clipboard?.writeText(key).then(
      () => showMsg("已复制密钥", "ok"),
      () => showMsg("复制失败，请手动选择", "err"),
    );
  }
});

$("#detail-close").addEventListener("click", closeDetail);
$("#detail-mask").addEventListener("click", (e) => {
  if (e.target === $("#detail-mask")) closeDetail();
});
document.addEventListener("keydown", (e) => {
  if (e.key === "Escape" && !$("#detail-mask").classList.contains("hidden"))
    closeDetail();
  if (e.key === "Escape") $("#import-mask").classList.add("hidden");
});

$("#status-tabs").addEventListener("click", (e) => {
  const tab = e.target.closest(".tab");
  if (!tab) return;
  filterStatus = tab.dataset.status;
  curPage = 1;
  $("#status-tabs")
    .querySelectorAll(".tab")
    .forEach((t) => t.classList.remove("active"));
  tab.classList.add("active");
  renderTable();
});

let searchTimer = null;
$("#search").addEventListener("input", (e) => {
  searchKw = e.target.value;
  curPage = 1;
  clearTimeout(searchTimer);
  searchTimer = setTimeout(renderTable, 150);
});

$("#page-size").addEventListener("change", (e) => {
  pageSize = parseInt(e.target.value, 10) || 20;
  curPage = 1;
  renderTable();
});

$("#pager").addEventListener("click", (e) => {
  const btn = e.target.closest(".page-btn");
  if (!btn || btn.disabled) return;
  const p = parseInt(btn.dataset.page, 10);
  if (p >= 1) {
    curPage = p;
    renderTable();
  }
});

$("#acc-body").addEventListener("click", (e) => {
  const tr = e.target.closest("tr[data-email]");
  if (!tr) return;
  const email = tr.dataset.email;
  if (e.target.classList.contains("act-refresh"))
    refreshAccount(email, e.target);
  else if (e.target.classList.contains("act-detail")) openDetail(email);
  else if (e.target.classList.contains("act-del")) deleteOne(email);
});

initPool($("#page-pool"));
if (currentRole !== "admin") {
  adminRoot
    .querySelectorAll('.nav-item:not([data-page="pool"])')
    .forEach((el) => el.classList.add("hidden"));
} else {
  refreshAccountsPage();
  loadConfig();
}
switchPage(initialPage);
}

function initLogin() {
  const loginRoot = document.querySelector(".view-login");
  const form = loginRoot.querySelector("#form");
  const errEl = loginRoot.querySelector("#err");
  const btn = loginRoot.querySelector("#btn");
  const credentialEl = loginRoot.querySelector("#credential");

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    errEl.textContent = "";
    btn.disabled = true;
    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          credential: credentialEl.value,
        }),
      });
      if (res.ok) {
        const { role } = await res.json();
        saveAuth(credentialEl.value, role);
        boot();
        return;
      }
      const data = await res.json().catch(() => ({}));
      errEl.textContent = data.error || "登录失败";
    } catch (e) {
      errEl.textContent = "网络错误：" + e.message;
    } finally {
      btn.disabled = false;
    }
  });
}

function initPool(poolRoot) {
  const $pool = (sel) => poolRoot.querySelector(sel);
  const listEl = $pool("#pool-list");
  const countEl = $pool("#pool-count");
  const pagerEl = $pool("#pool-pager");

  let accounts = [];
  let current = "";
  let curPage = 1;
  const pageSize = 24;

  const statusLabel = {
    active: "正常",
    expired: "已失效",
    error: "错误",
    unknown: "未查询",
  };
  const iconArrow =
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M5 12h14M13 6l6 6-6 6"/></svg>';

  function render() {
    if (!accounts.length) {
      countEl.textContent = "";
      listEl.innerHTML = `<div class="empty">号池暂无可用账号</div>`;
      pagerEl.innerHTML = "";
      return;
    }
    countEl.textContent = `（共 ${accounts.length} 个）`;

    const totalPages = Math.max(1, Math.ceil(accounts.length / pageSize));
    if (curPage > totalPages) curPage = totalPages;
    const start = (curPage - 1) * pageSize;
    const pageRows = accounts.slice(start, start + pageSize);

    listEl.innerHTML = pageRows
      .map((a, i) => {
        const st = a.status || "unknown";
        const isCur = a.email === current;
        const no = start + i + 1;
        return `<div class="pool-card${isCur ? " current" : ""}" data-email="${esc(a.email)}">
          <div class="pool-card-head">
            <span class="card-logo"><img src="/static/claude.svg" alt="Claude"></span>
            <div class="acc-no">账号 #${no}${isCur ? ` <span class="curtag">当前</span>` : ""}</div>
            <div class="card-state">
              <span class="st ${esc(st)}"><span class="dot"></span>${esc(statusLabel[st] || st)}</span>
            </div>
          </div>
          <div class="pool-card-body">
            <div class="meta">
              <span class="pool-email">${esc(a.email)}</span>
            </div>
          </div>
          <div class="pool-card-foot">
            <span class="enter">进入使用 ${iconArrow}</span>
          </div>
        </div>`;
      })
      .join("");

    renderPager(totalPages);
  }

  function renderPager(totalPages) {
    if (totalPages <= 1) {
      pagerEl.innerHTML = "";
      return;
    }
    const btn = (label, page, disabled, active) =>
      `<button class="page-btn${active ? " active" : ""}" data-page="${page}"${disabled ? " disabled" : ""}>${label}</button>`;
    let html = btn("上一页", curPage - 1, curPage === 1, false);
    for (let p = 1; p <= totalPages; p++)
      html += btn(p, p, false, p === curPage);
    html += btn("下一页", curPage + 1, curPage === totalPages, false);
    html += `<span class="pager-info">${curPage}/${totalPages} 页</span>`;
    pagerEl.innerHTML = html;
  }

  let poolKey = authToken;
  const keyMask = document.querySelector("#pool-key-mask");
  const keyInput = document.querySelector("#pool-key-input");
  const keySubmit = document.querySelector("#pool-key-submit");
  const keyErr = document.querySelector("#pool-key-err");

  async function apiFetch(url, opts = {}) {
    const headers = Object.assign({}, opts.headers);
    if (poolKey) headers.Authorization = "Bearer " + poolKey;
    const res = await fetch(
      url,
      Object.assign({ cache: "no-store" }, opts, { headers }),
    );
    if (res.status === 401) {
      clearAuth();
      poolKey = "";
      showKeyPrompt("请输入有效的访问密钥");
      throw new Error("unauthorized");
    }
    return res;
  }

  function showKeyPrompt(msg) {
    keyErr.textContent = msg || "";
    keyMask.classList.remove("hidden");
    setTimeout(() => keyInput.focus(), 30);
  }

  async function submitKey() {
    const val = keyInput.value.trim();
    if (!val) {
      keyErr.textContent = "请输入密钥";
      return;
    }
    try {
      const res = await fetch("/api/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ credential: val }),
      });
      if (res.status === 401) {
        keyErr.textContent = "密钥无效";
        return;
      }
      const { role } = await res.json();
      poolKey = val;
      saveAuth(val, role);
      if (role === "admin") {
        boot();
        return;
      }
      keyMask.classList.add("hidden");
      load();
    } catch (e) {
      keyErr.textContent = "网络错误：" + e.message;
    }
  }

  async function load() {
    try {
      const res = await apiFetch("/pool/api/list");
      const data = await res.json();
      accounts = data.accounts || [];
      current = data.current || "";
      render();
    } catch (e) {
      if (e.message !== "unauthorized") countEl.textContent = "（加载失败）";
    }
  }

  async function selectAccount(email) {
    try {
      await apiFetch("/pool/api/select?email=" + encodeURIComponent(email), {
        redirect: "manual",
      });
      window.location.href = "/new";
    } catch (e) {
      if (e.message !== "unauthorized") alert("切换账号失败：" + e.message);
    }
  }

  $pool("#btn-pool-refresh").addEventListener("click", load);
  $pool("#btn-pool-random")
    .addEventListener("click", () => selectAccount(""));
  keySubmit.addEventListener("click", submitKey);
  keyInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") submitKey();
  });

  listEl.addEventListener("click", (e) => {
    const card = e.target.closest(".pool-card");
    if (!card) return;
    const email = card.dataset.email;
    if (email === current) window.location.href = "/new";
    else selectAccount(email);
  });

  pagerEl.addEventListener("click", (e) => {
    const b = e.target.closest(".page-btn");
    if (!b || b.disabled) return;
    curPage = parseInt(b.dataset.page, 10) || 1;
    render();
  });

  load();
}

function resolvePage() {
  if (!authRole || !authToken) return { page: "login", role: "guest" };
  return {
    page: "admin",
    role: authRole,
    initialPage: "pool",
  };
}

function boot() {
  const { page, role, initialPage } = resolvePage();
  document.body.dataset.page = page;
  document.body.dataset.role = role;
  document
    .querySelectorAll(".view")
    .forEach((el) =>
      el.classList.toggle("hidden", !el.classList.contains("view-" + page)),
    );

  if (page === "admin")
    document.title = "Claude2API · 号池选择";
  else if (page === "login") document.title = "登录 · Claude2API";
  else document.title = "Claude2API · 选择账号";

  if (page === "admin") initAdmin(initialPage, role);
  else if (page === "login") initLogin();
}

boot();
