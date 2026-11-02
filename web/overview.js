"use strict";

function initOverview(root, api, showMsg) {
  const $ = (selector) => root.querySelector(selector);
  const numbers = new Intl.NumberFormat("zh-CN");
  const shortNumbers = new Intl.NumberFormat("zh-CN", { notation: "compact", maximumFractionDigits: 1 });
  const dayFormat = new Intl.DateTimeFormat("sv-SE");
  const colors = ["#36a3cf", "#45a883", "#9181dd", "#d4a45a"];
  const sourceNames = { "chat/completions": "Chat 对话", responses: "Responses 对话", messages: "Claude 对话", mirror: "网页对话" };
  // Canvas text is not in the DOM, so route it through the i18n helper.
  const tl = (text) => (window.i18n ? window.i18n.t(text) : text);
  const chartLabels = ["输入估算", "回答与工具估算", "可见思考估算", "请求数"];
  let days = 30;
  let group = "model";
  const pages = { model: 1, account: 1 };
  const pageSizes = { model: 10, account: 10 };
  let chart;
  let requestId = 0;

  async function load() {
    const currentRequest = ++requestId;
    $("#overview-refresh").disabled = true;
    $("#overview-status").textContent = "正在读取用量…";
    const query = new URLSearchParams({ days, group, timezone_offset: new Date().getTimezoneOffset(), limit: pageSizes[group], offset: (pages[group] - 1) * pageSizes[group] });
    try {
      const { usage } = await api(`/api/usage?${query}`);
      if (currentRequest !== requestId) return;
      const summary = usage.summary;
      const success = summary.requests ? `${(summary.successes / summary.requests * 100).toFixed(1)}%` : "—";
      const metrics = [
        ["账号数", numbers.format(usage.accounts), `${numbers.format(usage.active_accounts)} 个正常账号`],
        ["请求数", numbers.format(summary.requests), `成功率 ${success}`],
        ["总 Token · 估算", numbers.format(summary.total_tokens), `输入 ${shortNumbers.format(summary.input_tokens)} · 输出 ${shortNumbers.format(summary.output_tokens + summary.thinking_tokens)}`],
        ["可见思考 · 估算", numbers.format(summary.thinking_tokens), "缓存用量未提供"],
        ["平均首字", summary.first_token_samples ? `${(summary.first_token_ms / 1000).toFixed(2)} s` : "—", `${numbers.format(summary.first_token_samples)} 个有输出的请求`],
      ];
      $("#overview-metrics").innerHTML = metrics.map(([label, value, detail]) => `<div class="overview-metric"><span>${esc(label)}</span><strong>${esc(value)}</strong><small>${esc(detail)}</small></div>`).join("");
      $("#overview-success").textContent = `${success} 成功率`;
      renderSources(usage.endpoints, summary.requests);
      renderGroups(usage.groups, usage.total);
      renderActivity(usage.activity);
      renderTrend(usage.daily);
      $("#overview-status").textContent = `更新于 ${new Date().toLocaleTimeString("zh-CN", { hour12: false })}`;
    } catch (error) {
      if (currentRequest === requestId) $("#overview-status").textContent = `读取失败：${error.message}`;
    } finally {
      if (currentRequest === requestId) $("#overview-refresh").disabled = false;
    }
  }

  function renderSources(sources, requests) {
    if (!sources.length) {
      $("#overview-sources").innerHTML = '<div class="overview-empty">暂无对话记录</div>';
      return;
    }
    const bar = sources.map((source, index) => `<span style="flex:${source.requests};background:${colors[index]}" title="${esc(sourceNames[source.name])} · ${numbers.format(source.requests)} 次"></span>`).join("");
    const rows = sources.map((source, index) => `<div class="source-row"><i style="background:${colors[index]}"></i><div><b>${esc(sourceNames[source.name])}</b><small>成功率 ${(source.successes / source.requests * 100).toFixed(1)}% · ${shortNumbers.format(source.total_tokens)} 估算 Token</small></div><div class="source-value"><b>${numbers.format(source.requests)}</b><small>${(source.requests / requests * 100).toFixed(1)}%</small></div></div>`).join("");
    $("#overview-sources").innerHTML = `<div class="source-bar">${bar}</div>${rows}`;
  }

  function renderGroups(rows, total) {
    $("#overview-group-label").textContent = group === "model" ? "模型" : "账号";
    $("#overview-rows").innerHTML = rows.length ? rows.map((row) => `<tr><td><button class="usage-copy${group === "model" ? " model-icon-label" : ""}" data-copy="${esc(row.name)}" title="点击复制">${esc(row.name || "未分配账号")}</button></td><td>${numbers.format(row.requests)}</td><td>${(row.successes / row.requests * 100).toFixed(1)}%</td><td>${numbers.format(row.input_tokens)}</td><td>${numbers.format(row.output_tokens)}</td><td>${numbers.format(row.thinking_tokens)}</td><td class="usage-total">${numbers.format(row.total_tokens)}</td></tr>`).join("") : '<tr><td colspan="7" class="empty">暂无对话记录</td></tr>';
    const totalPages = Math.max(1, Math.ceil(total / pageSizes[group]));
    $("#overview-pager").innerHTML = `<button class="page-btn" data-step="-1"${pages[group] === 1 ? " disabled" : ""}>上一页</button><span class="pager-info">${pages[group]} / ${totalPages} 页 · 共 ${numbers.format(total)} 条</span><button class="page-btn" data-step="1"${pages[group] >= totalPages ? " disabled" : ""}>下一页</button>`;
  }

  function renderTrend(rows) {
    if (chart) chart.destroy();
    if (typeof Chart !== "function") throw new Error("图表加载失败，请刷新页面");
    const dates = [];
    const byDay = new Map(rows.map((row) => [row.name, row]));
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    start.setDate(start.getDate() - ((days || 30) - 1));
    if (days === 0 && rows.length) start.setTime(new Date(`${rows[0].name}T00:00:00`).getTime());
    for (const day = new Date(start); day <= new Date(); day.setDate(day.getDate() + 1)) dates.push(dayFormat.format(day));
    $("#overview-chart-empty").classList.toggle("hidden", rows.length > 0);
    chart = new Chart($("#overview-chart"), {
      type: "line",
      data: {
        labels: dates,
        datasets: [
          { label: tl(chartLabels[0]), data: dates.map((date) => byDay.get(date)?.input_tokens ?? 0), borderColor: colors[0], backgroundColor: "#36a3cf15", fill: true, yAxisID: "tokens" },
          { label: tl(chartLabels[1]), data: dates.map((date) => byDay.get(date)?.output_tokens ?? 0), borderColor: colors[1], backgroundColor: "#45a88315", fill: true, yAxisID: "tokens" },
          { label: tl(chartLabels[2]), data: dates.map((date) => byDay.get(date)?.thinking_tokens ?? 0), borderColor: colors[2], yAxisID: "tokens" },
          { label: tl(chartLabels[3]), data: dates.map((date) => byDay.get(date)?.requests ?? 0), borderColor: "#9ca6ad", borderDash: [4, 4], yAxisID: "requests" },
        ],
      },
      options: {
        responsive: true, maintainAspectRatio: false,
        interaction: { intersect: false, mode: "index" },
        elements: { point: { radius: 0, hitRadius: 5 }, line: { borderWidth: 1.7, tension: 0.25 } },
        plugins: { legend: { position: "bottom", labels: { usePointStyle: true, boxWidth: 6, font: { size: 11 } } } },
        scales: {
          x: { grid: { display: false }, ticks: { maxTicksLimit: 8, callback: (value) => dates[value].slice(5) } },
          tokens: { beginAtZero: true, grid: { color: "#80808012" }, ticks: { callback: (value) => tl(shortNumbers.format(value)) } },
          requests: { beginAtZero: true, position: "right", grid: { display: false }, ticks: { precision: 0 } },
        },
      },
    });
  }

  function renderActivity(rows) {
    const byDay = new Map(rows.map((row) => [row.name, row.requests]));
    const max = Math.max(1, ...rows.map((row) => row.requests));
    const start = new Date();
    start.setHours(0, 0, 0, 0);
    start.setDate(start.getDate() - 179);
    let cells = '<span class="activity-pad"></span>'.repeat((start.getDay() + 6) % 7);
    for (let i = 0; i < 180; i++) {
      const date = new Date(start);
      date.setDate(date.getDate() + i);
      const name = dayFormat.format(date);
      const count = byDay.get(name) ?? 0;
      const level = count ? Math.max(1, Math.ceil(count / max * 4)) : 0;
      cells += `<span data-level="${level}" title="${name} · ${numbers.format(count)} 次请求" role="img" aria-label="${name}，${numbers.format(count)} 次请求"></span>`;
    }
    $("#overview-activity").innerHTML = cells;
    $("#overview-activity-total").innerHTML = `<strong>${numbers.format(rows.reduce((sum, row) => sum + row.requests, 0))}</strong><span>次请求</span>`;
  }

  document.addEventListener("claude2api:langchange", () => {
    if (!chart) return;
    chart.data.datasets.forEach((dataset, index) => { dataset.label = tl(chartLabels[index]); });
    chart.update();
    chart.resize();
  });
  $("#overview-refresh").addEventListener("click", load);
  $(".overview-ranges").addEventListener("click", (event) => {
    const button = event.target.closest("[data-days]");
    if (!button) return;
    days = Number(button.dataset.days);
    pages.model = pages.account = 1;
    root.querySelectorAll("[data-days]").forEach((item) => item.classList.toggle("active", item === button));
    load();
  });
  $(".overview-groups").addEventListener("click", (event) => {
    const button = event.target.closest("[data-group]");
    if (!button) return;
    group = button.dataset.group;
    $("#overview-page-size").tomselect.setValue(pageSizes[group], true);
    root.querySelectorAll("[data-group]").forEach((item) => item.classList.toggle("active", item === button));
    load();
  });
  $("#overview-pager").addEventListener("click", (event) => {
    const button = event.target.closest("[data-step]");
    if (!button || button.disabled) return;
    pages[group] += Number(button.dataset.step);
    load();
  });
  $("#overview-page-size").addEventListener("change", (event) => {
    pageSizes[group] = Number(event.target.value);
    pages[group] = 1;
    load();
  });
  $("#overview-rows").addEventListener("click", async (event) => {
    const button = event.target.closest("[data-copy]");
    if (!button) return;
    try {
      await navigator.clipboard.writeText(button.dataset.copy);
      showMsg("已复制", "ok");
    } catch {
      showMsg("复制失败，请手动复制", "err");
    }
  });
  return { load };
}
