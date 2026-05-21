// LAN Proxy Gateway · Control Panel
//
// 设计要点：
//   - 状态单一来源：每次 mutation 后都重 GET /api/status 并整体重渲染。
//   - Dashboard 实时数据走 Mihomo Controller API（/traffic、/connections、/proxies）。
//     CORS 由 mihomo 模板里的 allow-origins: "*" 保证；走 fetch 直连，
//     不经过 gateway 自己的 :19091 减少耦合。
//   - 切节点也直接 PUT 到 mihomo /proxies/<group>。

const $  = (id) => document.getElementById(id);
const $$ = (sel) => Array.from(document.querySelectorAll(sel));

let cachedState = null;
let activeRuleVerdict = "direct";
let activeRuleGroup = "";
const panels = ["overview", "routing", "profile", "devices", "advanced"];

// ─────────── Auth token (P0-1) ───────────
// 后端 /api/* 全部要 Bearer token。token 通过 URL fragment `#token=...` 一次性带
// 进来：fragment 不会进 HTTP 请求行 / access log / referrer，足够安全。读到后
// 立刻 sessionStorage 存住，然后 replaceState 把 token 从 URL 抹掉，避免被
// 浏览器历史 / 截图 / 收藏夹泄漏。
//
// 关掉一次浏览器（sessionStorage 失效）后用户需要重新从 CLI 启动横幅拷一次新 URL。
const TOKEN_KEY = "lpg.webui.token";

function readAndStashToken() {
  const hash = location.hash || "";
  const m = hash.match(/[#&]token=([^&]+)/);
  if (m && m[1]) {
    try { sessionStorage.setItem(TOKEN_KEY, decodeURIComponent(m[1])); } catch {}
    // 抹掉 token，保留 panel 名（如 #routing）
    const panelMatch = hash.match(/^#([a-z]+)(?:[&#]|$)/);
    const newHash = panelMatch ? "#" + panelMatch[1] : "";
    history.replaceState(null, "", location.pathname + location.search + newHash);
  }
}

function getToken() {
  try { return sessionStorage.getItem(TOKEN_KEY) || ""; } catch { return ""; }
}

// 调用 readAndStashToken 必须在任何 fetch 之前。
readAndStashToken();

function activatePanel(name, push = true) {
  const panel = panels.includes(name) ? name : "overview";
  $$(".panel").forEach(el => el.classList.toggle("active", el.dataset.panel === panel));
  $$("#workspaceNav .nav-item").forEach(btn => {
    const active = btn.dataset.panel === panel;
    btn.classList.toggle("active", active);
    btn.setAttribute("aria-current", active ? "page" : "false");
  });
  if (push) history.replaceState(null, "", `#${panel}`);
}

// ─────────── HTTP helpers ───────────
async function api(method, path, body) {
  const opts = { method };
  const headers = {};
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    opts.body = JSON.stringify(body);
  }
  // 给 /api/* 自动加 Authorization: Bearer <token>。token 缺失时也发，让后端
  // 返回 401 + 友好提示让前端弹"请用带 #token= 的 URL 访问"。
  const tok = getToken();
  if (tok && path.startsWith("/api/")) {
    headers["Authorization"] = "Bearer " + tok;
  }
  opts.headers = headers;
  const res = await fetch(path, opts);
  if (res.status === 401) {
    showAuthError();
    throw new Error("unauthorized");
  }
  if (!res.ok) {
    let detail = res.statusText;
    try {
      const j = await res.json();
      if (j.error) detail = j.error;
    } catch {
      try { detail = (await res.text()) || detail; } catch {}
    }
    throw new Error(detail);
  }
  if (res.status === 204) return null;
  return res.json();
}

// showAuthError 在 401 时整个页面盖一层"请用带 token 的 URL"提示，让用户立刻明白。
// 用 dataset.shown 防重复创建。
function showAuthError() {
  if (document.body.dataset.authBlocked === "1") return;
  document.body.dataset.authBlocked = "1";
  const overlay = document.createElement("div");
  overlay.className = "auth-overlay";
  overlay.innerHTML = `
    <div class="auth-card">
      <h2>需要 Token</h2>
      <p>请使用 CLI 启动时打印的<strong>完整 URL（含 <code>#token=…</code>）</strong>访问。</p>
      <p>查看命令：<code>gateway webui</code> · 或重启 <code>gateway start --foreground</code> 看横幅。</p>
    </div>`;
  document.body.appendChild(overlay);
}

function mihomoBase() {
  if (!cachedState) return null;
  const host = cachedState.local_ip || location.hostname;
  return `http://${host}:${cachedState.mihomo_api_port}`;
}

async function mihomoFetch(path, init) {
  const base = mihomoBase();
  if (!base) throw new Error("尚未获取到 mihomo 端口");
  const res = await fetch(base + path, init);
  if (!res.ok) throw new Error(`mihomo ${path} → ${res.status}`);
  if (res.status === 204) return null;
  return res.json();
}

function toast(msg, kind = "ok", ms = 2400) {
  const t = document.createElement("div");
  t.className = `toast ${kind}`;
  // a11y：bad 用 role=alert 让 SR 抢断当前朗读；ok 用 role=status 弱通知
  t.setAttribute("role", kind === "bad" ? "alert" : "status");
  t.setAttribute("aria-live", kind === "bad" ? "assertive" : "polite");
  t.setAttribute("aria-atomic", "true");
  t.textContent = msg;
  document.body.appendChild(t);
  setTimeout(() => t.remove(), ms);
}

function humanBytes(n, perSec) {
  if (!Number.isFinite(n) || n < 0) n = 0;
  const u = ["B", "KB", "MB", "GB"];
  let i = 0;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  const val = n < 10 ? n.toFixed(2) : n.toFixed(1);
  return [val, perSec ? `${u[i]}/s` : u[i]];
}

async function copyText(text, btn) {
  try {
    await navigator.clipboard.writeText(text);
    btn.classList.add("copied");
    setTimeout(() => btn.classList.remove("copied"), 1200);
  } catch {
    toast("复制失败：浏览器拒绝", "bad");
  }
}

// ─────────── Main render ───────────
async function refresh() {
  try {
    const state = await api("GET", "/api/status");
    cachedState = state;
    render(state);
    $("updatedAt").textContent = "更新于 " + new Date().toLocaleTimeString();
  } catch (e) {
    toast("加载失败：" + e.message, "bad", 3600);
  }
}

function render(s) {
  // Hero
  const dot = $("heroDot");
  dot.dataset.status = s.running ? "running" : "stopped";
  $("heroState").textContent = s.running ? "服务运行中" : "服务未运行";
  $("heroIP").textContent = s.local_ip || "—";
  $("heroIP").classList.toggle("loading", !s.local_ip);
  $("sideIP").textContent = s.local_ip || "-";
  $("heroMode").textContent = s.gateway_mode_label || "—";
  $("metaMixed").textContent = (s.local_ip || "本机") + ":" + (s.mixed_port || "—");
  $("metaAPI").textContent   = (s.local_ip || "本机") + ":" + (s.mihomo_api_port || "—");
  $("hintMixedPort").textContent = `mixed-port (${s.mixed_port})`;
  renderSourceRuntime(s.source_runtime || {});

  // Banner
  renderBanner(s);

  renderCapabilities(s);

  // Traffic seg + switches
  $$("#trafficModeSeg button").forEach(b => {
    b.setAttribute("aria-checked", b.dataset.tmode === s.traffic_mode ? "true" : "false");
  });
  $("chkAdblock").checked    = !!s.adblock;
  $("chkAutoGroups").checked = !!s.auto_groups;
  $("chkDNS").checked        = !!s.dns_enabled;

  // Rulesets · 动态从 descriptors 渲染：开关 + 折叠详情。
  renderRulesets(s.ruleset_descriptors || [], s.rulesets || {});

  // Source
  $("selSourceType").value = s.source_type || "none";
  showSourcePanel(s.source_type);
  fillSourceForm(s.source);

  // Custom rules
  $("cntDirect").textContent = (s.rules?.direct || []).length;
  $("cntProxy").textContent  = (s.rules?.proxy  || []).length;
  $("cntReject").textContent = (s.rules?.reject || []).length;
  $("cntGroup").textContent  = (s.rules?.groups || []).reduce((n, g) => n + ((g.rules || []).length), 0);
  renderRulesPane(s);

  // Script
  renderScript(s.script || { mode: "none" });

  // Ports
  $("portMixed").placeholder = String(s.mixed_port || "");
  $("portRedir").placeholder = String(s.redir_port || "");
  $("portAPI").placeholder   = String(s.mihomo_api_port || "");
  $("portDNS").placeholder   = String(s.dns_port || "");
  // 只在用户尚未输入时填默认值
  if (!$("portMixed").value) $("portMixed").value = s.mixed_port || "";
  if (!$("portRedir").value) $("portRedir").value = s.redir_port || "";
  if (!$("portAPI").value)   $("portAPI").value   = s.mihomo_api_port || "";
  if (!$("portDNS").value)   $("portDNS").value   = s.dns_port || "";

  // Connectivity
  renderConnectivity(s.connectivity);

  // Footer & mihomo link
  const ver = "v" + (s.version || "—");
  $("version").textContent = ver;
  $("btnVersion").textContent = ver;
  const mihomoUrl = `${mihomoBase() || ""}/ui/`;
  $("mihomoLink").href = mihomoUrl;
  $("mihomoFullLink").href = mihomoUrl;
}

function renderSourceRuntime(rt) {
  const status = rt.status || "unknown";
  $("sourceDot").dataset.status = status === "ok" ? "running" : status === "bad" ? "stopped" : status === "warn" ? "warn" : "unknown";
  $("sourceLabel").textContent = rt.label || "—";
  $("sourceDetail").textContent = rt.detail || "—";
  $("sourceHealth").textContent = rt.status_text || "等待检测";
  const err = rt.last_error || "";
  $("sourceHealth").title = err;
  renderSourceQuickSwitch();
}

function sourceKey(p) {
  if (!p) return "";
  if (p.type === "subscription" && p.subscription) return `subscription:${p.subscription.url}`;
  if (p.type === "file" && p.file) return `file:${p.file.path}`;
  if (p.type === "external" && p.external) return `external:${p.external.kind}:${p.external.server}:${p.external.port}`;
  if (p.type === "remote" && p.remote) return `remote:${p.remote.kind}:${p.remote.server}:${p.remote.port}:${p.remote.username || ""}`;
  return p.type || "";
}

function isCurrentSource(p) {
  return sourceKey(p) === sourceKey(cachedState?.source);
}

function renderSourceQuickSwitch() {
  const box = $("sourceQuickSwitch");
  if (!box || !cachedState) return;
  const profiles = cachedState.source_profiles || [];
  const alternatives = profiles.filter(p => !isCurrentSource(p));
  box.innerHTML = "";
  if (!alternatives.length) {
    const empty = document.createElement("div");
    empty.className = "source-switch-empty";
    empty.innerHTML = `<span>暂无其它可切换源</span><button class="text-btn" type="button">添加 / 修改</button>`;
    empty.querySelector("button").addEventListener("click", () => activatePanel("profile"));
    box.appendChild(empty);
    return;
  }
  for (const p of alternatives.slice(0, 4)) {
    const btn = document.createElement("button");
    btn.className = "source-switch-btn";
    btn.type = "button";
    btn.innerHTML = `<strong>${escapeHTML(sourceTitle(p))}</strong><span>${escapeHTML(sourceSubtitle(p) || "直连")}</span>`;
    btn.addEventListener("click", () => switchSourceFromButton(p, btn));
    box.appendChild(btn);
  }
  if (alternatives.length > 4) {
    const more = document.createElement("button");
    more.className = "source-switch-more";
    more.type = "button";
    more.textContent = `更多 ${alternatives.length - 4} 个`;
    more.addEventListener("click", openSourceModal);
    box.appendChild(more);
  }
}

async function switchSourceFromButton(p, btn) {
  if (!p || btn.dataset.busy === "1") return false;
  btn.dataset.busy = "1";
  btn.disabled = true;
  try {
    await api("POST", "/api/source", p);
    toast("已切换代理源，正在重载...");
    setTimeout(() => { refresh(); loadProxies(); }, 1500);
    return true;
  } catch (e) {
    toast("切换失败：" + e.message, "bad", 4000);
    btn.disabled = false;
    btn.dataset.busy = "0";
    return false;
  }
}

function renderCapabilities(s) {
  const tunOn = !!s.tun_enabled;
  const proxyOn = !s.proxy_service || !!s.proxy_service.enabled;
  const authOn = !!(s.proxy_service && s.proxy_service.username);
  $("capTun").classList.toggle("active", tunOn);
  $("capTun").classList.add("drill-card");
  $("capTun").setAttribute("role", "button");
  $("capTun").setAttribute("tabindex", "0");
  $("capTunState").textContent = tunOn ? "TUN · 已启用" : "TUN · 未启用";
  $("capProxy").classList.toggle("active", proxyOn);
  $("capProxy").classList.add("drill-card");
  $("capProxy").setAttribute("role", "button");
  $("capProxy").setAttribute("tabindex", "0");
  $("capProxyState").textContent = proxyOn
    ? `HTTP / SOCKS5 · ${s.mixed_port || "—"}${authOn ? " · 需认证" : ""}`
    : "HTTP / SOCKS5 · 已关闭";
}

function renderBanner(s) {
  const alert = $("globalAlert");
  alert.hidden = true;
  alert.className = "banner";
  if (!s.running) {
    alert.className = "banner bad";
    alert.hidden = false;
    alert.textContent = "mihomo 未运行。请先 gateway install 安装内核，或检查启动日志。";
  } else if (s.mixed_port_down) {
    alert.className = "banner bad";
    alert.hidden = false;
    alert.textContent = `代理端口 ${s.mixed_port} 不可达 —— LAN 设备无法连接。可能被其它进程占用或防火墙拦截。`;
  } else if (s.platform === "darwin" && s.tun_enabled) {
    // local_ip 来源是后端从系统网卡 detect，理论可控（macOS 上接口 alias、
    // hostname 等），所以走 escapeHTML，不直接拼 innerHTML（XSS 防护）。
    alert.className = "banner";
    alert.hidden = false;
    alert.innerHTML = `透明代理模式 · LAN 设备需同时配置 <strong>默认网关</strong> 与 <strong>DNS</strong> 指向 <code>${escapeHTML(s.local_ip || "本机 IP")}</code>，仅改网关流量将走 NAT 不经代理。`;
  }
}

// ─────────── Connectivity card ───────────
function renderConnectivity(c) {
  const params = $("connParams");
  params.innerHTML = "";
  const fields = [
    { key: "本机 IP",    val: c.local_ip,    mono: true,  copyable: true  },
    { key: "路由器",     val: c.router,      mono: true,  copyable: false },
    { key: "代理端口",   val: `${c.mixed_port}`, mono: true, copyable: true },
    { key: "DNS 端口",   val: `${c.dns_port}`,   mono: true, copyable: false },
  ];
  for (const f of fields) {
    const div = document.createElement("div");
    div.className = "conn-param";
    const valHtml = f.copyable
      ? `<span class="conn-param-val">${escapeHTML(f.val)}<button class="copy-btn" type="button" aria-label="复制 ${escapeHTML(f.key)}" data-copy="${escapeHTML(f.val)}">${copyIcon()}</button></span>`
      : `<span class="conn-param-val">${escapeHTML(f.val)}</span>`;
    div.innerHTML = `<span class="conn-param-key">${escapeHTML(f.key)}</span>${valHtml}`;
    params.appendChild(div);
  }

  // Table rows
  const tbody = $("connTableBody");
  tbody.innerHTML = "";
  for (const m of (c.methods || [])) {
    const tr = document.createElement("tr");
    const fieldsHtml = m.fields.map(f =>
      `<span class="conn-field">
         <span class="conn-field-label">${escapeHTML(f.label)}</span>
         <span class="conn-field-val${f.mono ? " mono" : ""}">${escapeHTML(f.value)}</span>
       </span>`).join("");
    tr.innerHTML = `
      <td>${escapeHTML(m.scenario)}</td>
      <td>${escapeHTML(m.recommended)}</td>
      <td><div class="conn-fields">${fieldsHtml}</div></td>`;
    tbody.appendChild(tr);
  }

  // Notes
  const notes = $("connNotes");
  notes.innerHTML = "";
  if (c.notes && c.notes.length) {
    notes.hidden = false;
    for (const note of c.notes) {
      const div = document.createElement("div");
      div.className = "conn-note";
      div.textContent = note;
      notes.appendChild(div);
    }
  } else {
    notes.hidden = true;
  }
}

function copyIcon() {
  return `<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <rect x="8" y="8" width="13" height="13" rx="2" stroke="currentColor" stroke-width="1.6"/>
    <path d="M16 4H5a1 1 0 00-1 1v11" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
  </svg>`;
}

function escapeHTML(s) {
  return String(s ?? "").replace(/[&<>"']/g, c => (
    {"&":"&amp;","<":"&lt;",">":"&gt;","\"":"&quot;","'":"&#39;"}[c]
  ));
}

function openInfoModal(title, bodyHTML) {
  const old = document.querySelector(".info-modal");
  if (old) old.remove();
  const overlay = document.createElement("div");
  overlay.className = "info-modal";
  overlay.innerHTML = `
    <div class="info-dialog" role="dialog" aria-modal="true" aria-label="${escapeHTML(title)}">
      <header class="info-head">
        <h2>${escapeHTML(title)}</h2>
        <button class="icon-btn" type="button" aria-label="关闭">${copyCloseIcon()}</button>
      </header>
      <div class="info-body">${bodyHTML}</div>
    </div>`;
  const close = () => overlay.remove();
  overlay.addEventListener("click", (e) => {
    if (e.target === overlay || e.target.closest(".info-head .icon-btn")) close();
  });
  document.addEventListener("keydown", function onKey(e) {
    if (e.key === "Escape") {
      close();
      document.removeEventListener("keydown", onKey);
    }
  });
  document.body.appendChild(overlay);
  overlay.querySelector(".info-head .icon-btn").focus();
}

function copyCloseIcon() {
  return `<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M6 6l12 12M18 6L6 18" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
  </svg>`;
}

function kvHTML(rows) {
  return `<dl class="info-kv">${rows.map(([k, v]) => `
    <div><dt>${escapeHTML(k)}</dt><dd>${escapeHTML(v || "—")}</dd></div>`).join("")}</dl>`;
}

function sourceTitle(p) {
  switch (p.type) {
    case "subscription": return "订阅 · " + (p.subscription?.name || "subscription");
    case "file": return "本地配置文件";
    case "external": return "本机已有代理";
    case "remote": return "单节点 · " + (p.remote?.name || "remote");
    default: return "直连";
  }
}

function sourceSubtitle(p) {
  if (p.subscription) return p.subscription.url || "";
  if (p.file) return p.file.path || "";
  if (p.external) return `${p.external.kind || "http"}://${p.external.server}:${p.external.port}`;
  if (p.remote) return `${p.remote.kind || "socks5"}://${p.remote.server}:${p.remote.port}`;
  return "";
}

function openSourceModal() {
  const profiles = cachedState?.source_profiles || [];
  const cards = profiles.map((p, idx) => `
    <button class="profile-card" type="button" data-profile="${idx}">
      <strong>${escapeHTML(sourceTitle(p))}</strong>
      <span>${escapeHTML(sourceSubtitle(p))}</span>
    </button>`).join("");
  openInfoModal("代理源", `
    ${kvHTML([
      ["当前源", cachedState?.source_runtime?.label],
      ["状态", cachedState?.source_runtime?.status_text],
      ["详情", cachedState?.source_runtime?.detail],
      ["错误", cachedState?.source_runtime?.last_error],
    ])}
    <div class="profile-list">${cards || `<p class="info-note">暂无可切换的订阅预设。可在配置库添加订阅或本地配置。</p>`}</div>`);
  document.querySelectorAll(".profile-card").forEach(btn => {
    btn.addEventListener("click", async () => {
      const p = profiles[Number(btn.dataset.profile)];
      if (!p) return;
      btn.disabled = true;
      try {
        if (await switchSourceFromButton(p, btn)) {
          document.querySelector(".info-modal")?.remove();
        }
      } catch (e) {
        toast("切换失败：" + e.message, "bad", 4000);
        btn.disabled = false;
      }
    });
  });
}

function openTunModal() {
  const enabled = !!cachedState?.tun_enabled;
  openInfoModal("透明代理", `
    ${kvHTML([
      ["当前状态", enabled ? "已启用" : "未启用"],
      ["网关策略", cachedState?.gateway_mode_label || cachedState?.gateway_mode],
      ["DNS 服务", cachedState?.dns_enabled ? "已启用" : "未启用"],
      ["接入地址", cachedState?.local_ip || "—"],
    ])}
    <div class="info-actions">
      <button class="btn ${enabled ? "btn-secondary btn-warn" : "btn-primary"}" id="btnToggleTun" type="button">${enabled ? "关闭透明代理" : "开启透明代理"}</button>
      <button class="btn btn-secondary" id="btnGotoDevices" type="button">查看设备接入</button>
    </div>`);
  $("btnToggleTun").addEventListener("click", async () => {
    const btn = $("btnToggleTun");
    btn.disabled = true;
    try {
      await api("PATCH", "/api/config/tun", { enabled: !enabled });
      toast(!enabled ? "透明代理已开启" : "透明代理已关闭");
      document.querySelector(".info-modal")?.remove();
      setTimeout(() => { refresh(); loadProxies(); }, 1200);
    } catch (e) {
      toast("切换失败：" + e.message, "bad", 4000);
      btn.disabled = false;
    }
  });
  $("btnGotoDevices").addEventListener("click", () => {
    document.querySelector(".info-modal")?.remove();
    activatePanel("devices");
  });
}

function openProxyServiceModal() {
  const svc = cachedState?.proxy_service || { enabled: true };
  openInfoModal("代理服务", `
    <div class="proxy-service-form">
      <label class="switch-row">
        <span>
          <strong>HTTP/SOCKS5 mixed-port</strong>
          <small>${escapeHTML((cachedState?.local_ip || "本机") + ":" + (cachedState?.mixed_port || "—"))}</small>
        </span>
        <label class="ios-switch" aria-label="代理服务开关">
          <input id="proxySvcEnabled" type="checkbox" ${svc.enabled ? "checked" : ""} />
          <span class="ios-track"></span>
        </label>
      </label>
      <div class="form-row-pair modal-form-grid">
        <div>
          <label class="form-label" for="proxySvcUser">Username <span class="optional">可选</span></label>
          <input class="form-input" id="proxySvcUser" type="text" autocomplete="off" value="${escapeHTML(svc.username || "")}" />
        </div>
        <div>
          <label class="form-label" for="proxySvcPass">Password <span class="optional">可选</span></label>
          <input class="form-input" id="proxySvcPass" type="password" autocomplete="off" value="${escapeHTML(svc.password || "")}" />
        </div>
      </div>
      <p class="info-note">用户名和密码都留空时，局域网设备无需认证即可使用代理端口。</p>
      <div class="info-actions">
        <button class="btn btn-primary" id="btnSaveProxySvc" type="button">保存并重启代理服务</button>
        <button class="btn btn-secondary" id="btnGotoAdvanced" type="button">修改端口</button>
      </div>
    </div>`);
  $("btnSaveProxySvc").addEventListener("click", async () => {
    const btn = $("btnSaveProxySvc");
    btn.disabled = true;
    try {
      await api("PATCH", "/api/config/proxy-service", {
        enabled: $("proxySvcEnabled").checked,
        username: $("proxySvcUser").value.trim(),
        password: $("proxySvcPass").value,
      });
      toast("代理服务已保存，正在重启...");
      document.querySelector(".info-modal")?.remove();
      setTimeout(() => { refresh(); startMihomoTraffic(); loadProxies(); }, 4000);
    } catch (e) {
      toast("保存失败：" + e.message, "bad", 4000);
      btn.disabled = false;
    }
  });
  $("btnGotoAdvanced").addEventListener("click", () => {
    document.querySelector(".info-modal")?.remove();
    activatePanel("advanced");
  });
}

async function openVersionModal() {
  try {
    const info = await api("GET", "/api/update");
    const body = `
      ${kvHTML([["当前版本", info.current], ["最新版本", info.latest || "未知"]])}
      <div class="info-actions">
        ${info.available ? `<button class="btn btn-primary" id="btnRunUpdate" type="button">升级到 ${escapeHTML(info.latest)}</button>` : `<span class="info-note">当前已是最新版本。</span>`}
        ${info.url ? `<a class="btn btn-secondary" href="${escapeHTML(info.url)}" target="_blank" rel="noopener">查看发布页</a>` : ""}
      </div>`;
    openInfoModal("版本更新", body);
    const run = $("btnRunUpdate");
    if (run) {
      run.addEventListener("click", async () => {
        run.disabled = true;
        await api("POST", "/api/update");
        toast("已启动升级进程，完成后请刷新页面");
      });
    }
  } catch (e) {
    toast("检查更新失败：" + e.message, "bad", 4000);
  }
}

async function openConnectionsModal() {
  try {
    const j = await mihomoFetch("/connections", { cache: "no-store" });
    const conns = (j && j.connections) || [];
    const devices = uniqueDeviceIPs(conns);
    const rows = conns.slice(0, 80).map(c => {
      const md = c.metadata || {};
      const target = md.host || md.destinationIP || "—";
      const port = md.destinationPort ? `:${md.destinationPort}` : "";
      const chains = (c.chains || []).join(" -> ") || "—";
      return `<tr>
        <td>${escapeHTML(md.sourceIP || "—")}</td>
        <td>${escapeHTML(target + port)}</td>
        <td>${escapeHTML(c.rule || "—")}</td>
        <td>${escapeHTML(chains)}</td>
      </tr>`;
    }).join("");
    openInfoModal("接入设备", `
      <p class="info-note">当前 ${devices.length} 台设备有活跃请求，共 ${conns.length} 条网络连接，最多显示前 80 条。</p>
      <div class="device-chip-list">${devices.map(ip => `<span class="device-chip">${escapeHTML(ip)}</span>`).join("") || `<span class="info-note">暂无设备</span>`}</div>
      <div class="info-table-wrap">
        <table class="info-table">
          <thead><tr><th>来源</th><th>目标</th><th>规则</th><th>链路</th></tr></thead>
          <tbody>${rows || `<tr><td colspan="4">暂无活跃连接</td></tr>`}</tbody>
        </table>
      </div>`);
  } catch (e) {
    toast("连接详情读取失败：" + e.message, "bad", 3600);
  }
}

function uniqueDeviceIPs(conns) {
  const local = new Set(["127.0.0.1", "::1", "localhost", cachedState?.local_ip].filter(Boolean));
  const out = new Set();
  for (const c of conns || []) {
    const ip = c?.metadata?.sourceIP || "";
    if (!ip || local.has(ip)) continue;
    out.add(ip);
  }
  return Array.from(out).sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
}

// ─────────── Source ───────────
function showSourcePanel(type) {
  for (const id of ["srcSubscription", "srcExternal", "srcFile", "srcRemote"]) {
    $(id).hidden = true;
  }
  switch (type) {
    case "subscription": $("srcSubscription").hidden = false; break;
    case "external":     $("srcExternal").hidden = false; break;
    case "file":         $("srcFile").hidden = false; break;
    case "remote":       $("srcRemote").hidden = false; break;
  }
}

function fillSourceForm(src) {
  if (!src) return;
  if (src.subscription) {
    $("subUrl").value  = src.subscription.url || "";
    $("subName").value = src.subscription.name || "";
  }
  if (src.external) {
    $("extServer").value = src.external.server || "";
    $("extPort").value   = src.external.port || "";
    $("extKind").value   = src.external.kind || "http";
  }
  if (src.file)   $("fileSrcPath").value = src.file.path || "";
  if (src.remote) {
    $("remName").value   = src.remote.name || "";
    $("remKind").value   = src.remote.kind || "socks5";
    $("remServer").value = src.remote.server || "";
    $("remPort").value   = src.remote.port || "";
    $("remUser").value   = src.remote.username || "";
    $("remPass").value   = src.remote.password || "";
  }
}

function collectSource() {
  const type = $("selSourceType").value;
  const payload = { type };
  switch (type) {
    case "subscription":
      payload.subscription = {
        url: $("subUrl").value.trim(),
        name: $("subName").value.trim() || "subscription",
      };
      break;
    case "external":
      payload.external = {
        name: "External Proxy",
        server: $("extServer").value.trim() || "127.0.0.1",
        port: parseInt($("extPort").value, 10) || 7890,
        kind: $("extKind").value,
      };
      break;
    case "file":
      payload.file = { path: $("fileSrcPath").value.trim() };
      break;
    case "remote":
      payload.remote = {
        name: $("remName").value.trim() || "remote",
        kind: $("remKind").value,
        server: $("remServer").value.trim(),
        port: parseInt($("remPort").value, 10) || 0,
        username: $("remUser").value,
        password: $("remPass").value,
      };
      break;
  }
  return payload;
}

// ─────────── Custom Rules ───────────
function renderRulesPane(s) {
  const groupMode = activeRuleVerdict === "group";
  $("ruleGroupTarget").hidden = !groupMode;
  if (groupMode) {
    syncRuleGroupSelect(s.rules || {});
    const group = findActiveRuleGroup(s.rules || {});
    $("ruleArea").value = ((group && group.rules) || []).join("\n");
    return;
  }
  const list = (s.rules && s.rules[activeRuleVerdict]) || [];
  $("ruleArea").value = list.join("\n");
}

function collectRules(targetOverride) {
  // 当前 textarea 的内容写回 activeRuleVerdict 对应的 list；其它类别保留原状。
  const lines = $("ruleArea").value.split("\n").map(s => s.trim()).filter(Boolean);
  const base = cachedState && cachedState.rules ? cachedState.rules : { direct: [], proxy: [], reject: [], groups: [] };
  const out = {
    direct: base.direct || [],
    proxy:  base.proxy  || [],
    reject: base.reject || [],
    groups: (base.groups || []).map(g => ({
      target: (g.target || "").trim(),
      rules: Array.isArray(g.rules) ? g.rules.slice() : [],
    })).filter(g => g.target && g.rules.length),
  };
  if (activeRuleVerdict === "group") {
    const target = (targetOverride || currentRuleGroupTarget()).trim();
    if (!target) throw new Error("请先选择或填写目标策略组");
    let found = false;
    out.groups = out.groups.map(g => {
      if (g.target !== target) return g;
      found = true;
      return { target, rules: lines };
    }).filter(g => g.rules.length);
    if (!found && lines.length) out.groups.push({ target, rules: lines });
    activeRuleGroup = target;
    return out;
  }
  out[activeRuleVerdict] = lines;
  return out;
}

function policyGroupNames() {
  const names = new Set(["Proxy", "🛫 AI起飞节点", "🛬 AI落地节点"]);
  if (cachedState && cachedState.rules && Array.isArray(cachedState.rules.groups)) {
    cachedState.rules.groups.forEach(g => { if (g.target) names.add(g.target); });
  }
  if (cachedProxies && cachedProxies.proxies) {
    Object.values(cachedProxies.proxies)
      .filter(p => ["Selector", "URLTest", "Fallback", "LoadBalance"].includes(p.type))
      .forEach(p => { if (p.name) names.add(p.name); });
  }
  return Array.from(names).sort((a, b) => a.localeCompare(b));
}

function syncRuleGroupSelect(rules) {
  const groups = rules.groups || [];
  if (!activeRuleGroup && groups.length) activeRuleGroup = groups[0].target || "";
  if (!activeRuleGroup) activeRuleGroup = "Proxy";
  const names = policyGroupNames();
  if (!names.includes(activeRuleGroup)) names.push(activeRuleGroup);
  $("ruleGroupSelect").innerHTML = names.map(name =>
    `<option value="${escapeHTML(name)}">${escapeHTML(name)}</option>`
  ).join("");
  $("ruleGroupSelect").value = names.includes(activeRuleGroup) ? activeRuleGroup : "Proxy";
  $("ruleGroupCustom").value = names.includes(activeRuleGroup) ? "" : activeRuleGroup;
}

function currentRuleGroupTarget() {
  return ($("ruleGroupCustom").value.trim() || $("ruleGroupSelect").value || "").trim();
}

function findActiveRuleGroup(rules) {
  const target = currentRuleGroupTarget();
  return (rules.groups || []).find(g => g.target === target) || null;
}

// ─────────── Script (preset chain / custom .js) ───────────
function renderScript(sc) {
  const mode = sc.mode || "none";
  $$("#scriptModeSeg button").forEach(b => {
    b.setAttribute("aria-checked", b.dataset.smode === mode ? "true" : "false");
  });
  $("scriptPresetForm").hidden = mode !== "preset";
  $("scriptCustomForm").hidden = mode !== "custom";

  if (sc.chain_residential) {
    const c = sc.chain_residential;
    $("chainName").value   = c.name || "";
    $("chainKind").value   = c.kind || "http";
    $("chainServer").value = c.server || "";
    $("chainPort").value   = c.port || "";
    $("chainUser").value   = c.username || "";
    $("chainPass").value   = c.password || "";
    $("chainDialer").value = c.dialer_proxy || "";
  }
  if (sc.custom_path !== undefined) {
    $("scriptCustomPath").value = sc.custom_path || "";
  }
  $("scriptHint").textContent = ({
    none:   "保存后会清空扩展脚本配置。",
    preset: "保存后会自动生成链式代理预设脚本并注入 mihomo 配置。",
    custom: "保存后 mihomo 会加载该脚本对配置做最终修改。",
  })[mode];
}

function collectScript() {
  const seg = $$("#scriptModeSeg button").find(b => b.getAttribute("aria-checked") === "true");
  const mode = seg ? seg.dataset.smode : "none";
  const payload = { mode };
  if (mode === "preset") {
    payload.chain_residential = {
      name:         $("chainName").value.trim()   || "🏠 住宅IP",
      kind:         $("chainKind").value,
      server:       $("chainServer").value.trim(),
      port:         parseInt($("chainPort").value, 10) || 0,
      username:     $("chainUser").value,
      password:     $("chainPass").value,
      dialer_proxy: $("chainDialer").value.trim() || "🛫 AI起飞节点",
    };
  } else if (mode === "custom") {
    payload.custom_path = $("scriptCustomPath").value.trim();
  }
  return payload;
}

$$("#scriptModeSeg button").forEach(b => {
  b.addEventListener("click", () => {
    $$("#scriptModeSeg button").forEach(x => x.setAttribute("aria-checked", x === b ? "true" : "false"));
    $("scriptPresetForm").hidden = b.dataset.smode !== "preset";
    $("scriptCustomForm").hidden = b.dataset.smode !== "custom";
    $("scriptHint").textContent = ({
      none:   "保存后会清空扩展脚本配置。",
      preset: "保存后会自动生成链式代理预设脚本并注入 mihomo 配置。",
      custom: "保存后 mihomo 会加载该脚本对配置做最终修改。",
    })[b.dataset.smode];
  });
});

$("btnSaveScript").addEventListener("click", async () => {
  const btn = $("btnSaveScript");
  btn.disabled = true;
  try {
    await api("PUT", "/api/config/script", collectScript());
    toast("增强脚本已保存，正在重载…");
    setTimeout(() => { refresh(); loadProxies(); }, 1500);
  } catch (e) {
    toast(e.message, "bad", 4000);
  } finally {
    btn.disabled = false;
  }
});

// ─────────── Live Dashboard (Mihomo API) ───────────
let trafficES = null;
let lastTraffic = { down: 0, up: 0 };

function startMihomoTraffic() {
  const base = mihomoBase();
  if (!base) return;

  stopMihomoTraffic();

  // mihomo /traffic 返回 chunked JSON 流（每秒一行 {up, down}）。直接用 fetch
  // 的 ReadableStream 读，比 EventSource 还稳（mihomo 没启 SSE 头）。
  const controller = new AbortController();
  trafficES = controller;
  fetch(base + "/traffic", { signal: controller.signal })
    .then(async (res) => {
      if (!res.ok || !res.body) return;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split("\n");
        buf = lines.pop() || "";
        for (const line of lines) {
          if (!line.trim()) continue;
          try {
            const j = JSON.parse(line);
            applyTrafficSample(j);
          } catch {}
        }
      }
    })
    .catch(() => {});
}

function stopMihomoTraffic() {
  if (trafficES) {
    try { trafficES.abort(); } catch {}
    trafficES = null;
  }
}

function applyTrafficSample({ up, down }) {
  lastTraffic = { up: up || 0, down: down || 0 };
  const [dv, du] = humanBytes(lastTraffic.down, true);
  const [uv, uu] = humanBytes(lastTraffic.up,   true);
  $("statDown").innerHTML = `${dv} <small>${du}</small>`;
  $("statUp").innerHTML   = `${uv} <small>${uu}</small>`;
}

// cachedProxies 保留最近一次 GET /proxies 的快照，给"出口"和测速回显共用。
let cachedProxies = null;
let quickSelectedGroup = "";

async function pollConnections() {
  try {
    const j = await mihomoFetch("/connections", { cache: "no-store" });
    const conns = (j && j.connections) || [];
    $("statConn").textContent = String(uniqueDeviceIPs(conns).length);
  } catch {
    // mihomo 没起静默
  }
  renderEgress();
}

// renderEgress 优先用「主策略组当前选中节点」显示出口，比"最热连接"
// 稳定得多：
//   - 无连接时也能显示当前会用哪个节点
//   - 不会因为 GLOBAL/Proxy 改名而错位（按优先级匹配几个常用名）
//   - 显示节点延迟，更直观
function renderEgress() {
  const el = $("statEgress");
  if (!cachedProxies || !cachedProxies.proxies) {
    el.textContent = "—";
    return;
  }
  // 优先匹配常见主策略组名：Proxy → GLOBAL → 第一个 Selector
  const proxies = cachedProxies.proxies;
  const candidates = ["Proxy", "PROXY", "🚀 节点选择", "GLOBAL"];
  let main = null;
  for (const name of candidates) {
    if (proxies[name] && proxies[name].type === "Selector") { main = proxies[name]; break; }
  }
  if (!main) {
    main = Object.values(proxies).find(p => p.type === "Selector" && Array.isArray(p.all) && p.all.length > 1);
  }
  if (!main || !main.now) {
    el.textContent = "—";
    return;
  }
  // 顺着 now 找到底层真实节点（now 可能本身又是个组）
  let cur = main.now;
  let hops = 0;
  while (proxies[cur] && Array.isArray(proxies[cur].all) && proxies[cur].now && hops < 6) {
    cur = proxies[cur].now;
    hops++;
  }
  const node = proxies[cur];
  let delayStr = "";
  if (node && Array.isArray(node.history) && node.history.length) {
    const last = node.history[node.history.length - 1].delay;
    if (Number.isFinite(last) && last > 0) delayStr = ` · ${last}ms`;
  }
  el.textContent = cur + delayStr;
  el.title = `${main.name} → ${cur}${delayStr}`;
}

function renderQuickGroups() {
  const box = $("quickGroups");
  if (!box) return;
  if (!cachedProxies || !cachedProxies.proxies) {
    box.innerHTML = `<p class="empty-state">加载策略组...</p>`;
    return;
  }
  const proxies = cachedProxies.proxies;
  const selectors = Object.values(proxies)
    .filter(p => p.type === "Selector" && Array.isArray(p.all) && p.all.length > 0)
    .sort((a, b) => groupPriority(a.name) - groupPriority(b.name) || a.name.localeCompare(b.name));
  if (!selectors.length) {
    box.innerHTML = `<p class="empty-state">无可切换策略组</p>`;
    return;
  }
  if (!quickSelectedGroup || !selectors.some(g => g.name === quickSelectedGroup)) {
    quickSelectedGroup = selectors[0].name;
  }
  const selected = selectors.find(g => g.name === quickSelectedGroup) || selectors[0];
  box.innerHTML = "";

  const shell = document.createElement("div");
  shell.className = "quick-groups-shell";

  const rail = document.createElement("div");
  rail.className = "quick-group-rail";
  for (const g of selectors) {
    const btn = document.createElement("button");
    btn.className = "quick-group-tab";
    btn.type = "button";
    btn.setAttribute("aria-selected", g.name === selected.name ? "true" : "false");
    btn.innerHTML = `<span>${escapeHTML(g.name)}</span><strong>${escapeHTML(g.now || "—")}</strong><small>${g.all.length} 节点</small>`;
    btn.addEventListener("click", () => {
      quickSelectedGroup = g.name;
      renderQuickGroups();
    });
    rail.appendChild(btn);
  }

  const detail = document.createElement("div");
  detail.className = "quick-group-detail";
  const currentNode = proxies[selected.now];
  const delay = latestDelay(currentNode);
  detail.innerHTML = `
    <div class="quick-group-detail-head">
      <div>
        <span class="source-kicker">${escapeHTML(selected.name)}</span>
        <strong>${escapeHTML(selected.now || "—")}${delay ? ` · ${escapeHTML(delay)}` : ""}</strong>
      </div>
      <div class="quick-group-actions">
        <button class="btn btn-ghost btn-tiny" type="button" data-action="speed">测速</button>
        <button class="btn btn-ghost btn-tiny" type="button" data-action="open">完整列表</button>
      </div>
    </div>
    <div class="quick-node-list"></div>`;
  const nodeList = detail.querySelector(".quick-node-list");
  for (const name of selected.all.slice(0, 12)) {
    nodeList.appendChild(renderQuickNodeButton(name, selected, proxies));
  }
  if (selected.all.length > 12) {
    const more = document.createElement("button");
    more.className = "quick-node-more";
    more.type = "button";
    more.textContent = `还有 ${selected.all.length - 12} 个节点，打开完整列表`;
    more.addEventListener("click", () => openGroupInRouting(selected.name));
    nodeList.appendChild(more);
  }
  detail.querySelector('[data-action="speed"]').addEventListener("click", async (e) => {
    await testGroupLatency(selected, e.currentTarget);
    renderQuickGroups();
  });
  detail.querySelector('[data-action="open"]').addEventListener("click", () => openGroupInRouting(selected.name));

  shell.appendChild(rail);
  shell.appendChild(detail);
  box.appendChild(shell);
}

function openGroupInRouting(name) {
  activatePanel("routing");
  setTimeout(() => {
    document.querySelector(`[data-group="${CSS.escape(name)}"]`)?.scrollIntoView({ behavior: "smooth", block: "center" });
  }, 80);
}

function renderQuickNodeButton(name, grp, allProxies) {
  const btn = document.createElement("button");
  btn.className = "quick-node-btn";
  btn.type = "button";
  if (name === grp.now) btn.classList.add("active");
  const delay = latestDelay(allProxies[name]);
  btn.innerHTML = `<span>${escapeHTML(name)}</span>${delay ? `<small>${escapeHTML(delay)}</small>` : ""}`;
  btn.addEventListener("click", async () => {
    if (name === grp.now) return;
    btn.disabled = true;
    try {
      await mihomoFetch(`/proxies/${encodeURIComponent(grp.name)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      toast(`${grp.name} → ${name}`);
      loadProxies();
    } catch (e) {
      toast("切换失败：" + e.message, "bad", 3600);
      btn.disabled = false;
    }
  });
  return btn;
}

function groupPriority(name) {
  const order = ["🛫 AI起飞节点", "🛬 AI落地节点", "Proxy", "GLOBAL"];
  const i = order.indexOf(name);
  return i >= 0 ? i : 100;
}

function latestDelay(node) {
  if (!node || !Array.isArray(node.history) || !node.history.length) return "";
  const last = node.history[node.history.length - 1].delay;
  return Number.isFinite(last) && last > 0 ? `${last}ms` : "";
}

// 折叠阈值：节点数超过这个就默认只显示前 NODE_COLLAPSE_VISIBLE 个，剩下挂"展开 N 个"。
const NODE_COLLAPSE_VISIBLE = 12;

// 已展开的组名集合（按组名记），用户展开后重渲染仍保持展开。
const expandedGroups = new Set();

async function loadProxies() {
  const groupsEl = $("proxyGroups");
  try {
    const j = await mihomoFetch("/proxies");
    cachedProxies = j;
    renderEgress();
    renderQuickGroups();
    if (activeRuleVerdict === "group" && cachedState) renderRulesPane(cachedState);
    if (!j || !j.proxies) {
      groupsEl.innerHTML = `<p class="empty-state">mihomo 没有返回代理数据。</p>`;
      return;
    }
    const selectors = Object.values(j.proxies)
      .filter(p => p.type === "Selector" && Array.isArray(p.all) && p.all.length > 0)
      .sort((a, b) => a.name.localeCompare(b.name));

    if (!selectors.length) {
      groupsEl.innerHTML = `<p class="empty-state">无 Selector 策略组（订阅可能只含 url-test/fallback 组）。</p>`;
      return;
    }

    // 换订阅源 / mihomo reload 后旧组名可能消失，把 expandedGroups 里不存在的清掉，
    // 避免内存常驻一堆 stale 字符串（虽然只是 Set<string>，但养成好习惯）。
    const presentNames = new Set(selectors.map(g => g.name));
    for (const name of Array.from(expandedGroups)) {
      if (!presentNames.has(name)) expandedGroups.delete(name);
    }

    groupsEl.innerHTML = "";
    for (const grp of selectors) {
      groupsEl.appendChild(renderProxyGroup(grp, j.proxies));
    }
  } catch (e) {
    renderQuickGroups();
    if (activeRuleVerdict === "group" && cachedState) renderRulesPane(cachedState);
    groupsEl.innerHTML = `<p class="empty-state">无法连接 mihomo API：${escapeHTML(e.message)}</p>`;
  }
}

function renderProxyGroup(grp, allProxies) {
  const g = document.createElement("div");
  g.className = "group";
  g.dataset.group = grp.name;

  // ─── head: 组名 + 节点数 + 测速按钮 ───
  const head = document.createElement("div");
  head.className = "group-head";
  const left = document.createElement("div");
  left.className = "group-head-left";
  left.innerHTML = `
    <span class="group-name">${escapeHTML(grp.name)}</span>
    <span class="group-meta">当前 <strong>${escapeHTML(grp.now || "—")}</strong> · ${grp.all.length} 节点</span>`;
  const speedBtn = document.createElement("button");
  speedBtn.type = "button";
  speedBtn.className = "btn btn-ghost btn-tiny";
  speedBtn.innerHTML = `<svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M12 21a9 9 0 100-18 9 9 0 000 18z" stroke="currentColor" stroke-width="1.6"/>
    <path d="M12 12l4-4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>
    <circle cx="12" cy="12" r="1.4" fill="currentColor"/>
  </svg><span>测速</span>`;
  speedBtn.addEventListener("click", () => testGroupLatency(grp, speedBtn));
  head.appendChild(left);
  head.appendChild(speedBtn);

  // ─── nodes：超阈值默认折叠 ───
  const nodes = document.createElement("div");
  nodes.className = "nodes";

  const isExpanded = expandedGroups.has(grp.name) || grp.all.length <= NODE_COLLAPSE_VISIBLE;
  const visibleNames = isExpanded ? grp.all : grp.all.slice(0, NODE_COLLAPSE_VISIBLE);

  for (const name of visibleNames) {
    nodes.appendChild(renderNodeChip(name, grp, allProxies));
  }

  // 折叠展开按钮
  if (grp.all.length > NODE_COLLAPSE_VISIBLE) {
    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "node-chip node-toggle";
    if (isExpanded) {
      toggle.innerHTML = `<span>收起</span><svg viewBox="0 0 24 24" fill="none"><path d="M6 15l6-6 6 6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
    } else {
      toggle.innerHTML = `<span>展开 ${grp.all.length - NODE_COLLAPSE_VISIBLE} 个</span><svg viewBox="0 0 24 24" fill="none"><path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
    }
    toggle.addEventListener("click", () => {
      if (expandedGroups.has(grp.name)) expandedGroups.delete(grp.name);
      else expandedGroups.add(grp.name);
      loadProxies();
    });
    nodes.appendChild(toggle);
  }

  g.appendChild(head);
  g.appendChild(nodes);
  return g;
}

function renderNodeChip(name, grp, allProxies) {
  const chip = document.createElement("button");
  chip.className = "node-chip";
  chip.type = "button";
  chip.dataset.node = name;
  if (name === grp.now) chip.classList.add("active");

  const label = document.createElement("span");
  label.className = "node-name";
  label.textContent = name;
  chip.appendChild(label);

  const node = allProxies[name];
  if (node && Array.isArray(node.history) && node.history.length) {
    const last = node.history[node.history.length - 1].delay;
    const delay = document.createElement("span");
    delay.className = "delay";
    if (Number.isFinite(last) && last > 0) {
      delay.textContent = `${last}ms`;
      if (last > 1500)      delay.dataset.health = "slow";
      else if (last > 500)  delay.dataset.health = "warn";
      else                  delay.dataset.health = "ok";
    } else {
      delay.textContent = "超时";
      delay.dataset.health = "bad";
    }
    chip.appendChild(delay);
  }

  chip.addEventListener("click", async () => {
    if (name === grp.now) return;
    chip.classList.add("loading");
    try {
      await mihomoFetch(`/proxies/${encodeURIComponent(grp.name)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      toast(`${grp.name} → ${name}`);
      loadProxies();
    } catch (e) {
      toast("切换失败：" + e.message, "bad", 3600);
      chip.classList.remove("loading");
    }
  });
  return chip;
}

// testGroupLatency 并发 ping 整组节点，结果实时回写 chip 上的延迟数字。
// 用 mihomo /proxies/{name}/delay 接口，url 用 mihomo 默认健康检查地址。
async function testGroupLatency(grp, btn, force = false) {
  if (!force && btn.dataset.busy === "1") return;
  btn.dataset.busy = "1";
  const originalHTML = btn.innerHTML;
  btn.innerHTML = `<svg class="spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M21 12a9 9 0 11-3-6.7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
  </svg><span>测速中</span>`;
  btn.disabled = true;
    const testURL = "http://www.gstatic.com/generate_204";
  const timeout = 5000;
  // 跑并发但限制 8 路，免得节点数大时把 mihomo 压崩
  const queue = [...grp.all];
  let active = 0;
  await new Promise(resolve => {
    let pending = queue.length;
    if (!pending) return resolve();
    const next = () => {
      while (active < 8 && queue.length) {
        const name = queue.shift();
        active++;
        const path = `/proxies/${encodeURIComponent(name)}/delay`
          + `?url=${encodeURIComponent(testURL)}&timeout=${timeout}`;
        mihomoFetch(path)
          .then(r => updateNodeChipDelay(name, r && r.delay))
          .catch(() => updateNodeChipDelay(name, 0)) // 0 = 超时
          .finally(() => {
            active--; pending--;
            if (pending === 0) resolve();
            else next();
          });
      }
    };
    next();
  });
  btn.innerHTML = originalHTML;
  btn.disabled = false;
  btn.dataset.busy = "0";
  toast(`${grp.name} 测速完成`);
  loadProxies(); // 拉一次确保延迟落到 mihomo history
}

$("btnTestQuickGroups").addEventListener("click", async () => {
  if (!cachedProxies || !cachedProxies.proxies) return;
  const btn = $("btnTestQuickGroups");
  if (btn.dataset.busy === "1") return;
  btn.dataset.busy = "1";
  btn.disabled = true;
  const old = btn.textContent;
  btn.textContent = "测速中";
  try {
    const groups = Object.values(cachedProxies.proxies)
      .filter(p => p.type === "Selector" && Array.isArray(p.all) && p.all.length > 0)
      .sort((a, b) => groupPriority(a.name) - groupPriority(b.name) || a.name.localeCompare(b.name))
      .slice(0, 6);
    for (const g of groups) {
      await testGroupLatency(g, btn, true);
    }
  } finally {
    btn.textContent = old;
    btn.disabled = false;
    btn.dataset.busy = "0";
  }
});

function updateNodeChipDelay(name, delay) {
  // 找到当前 DOM 中所有同名 chip（一个节点可能出现在多个组里），就地更新
  const chips = document.querySelectorAll(`.node-chip[data-node="${CSS.escape(name)}"]`);
  chips.forEach(chip => {
    let el = chip.querySelector(".delay");
    if (!el) {
      el = document.createElement("span");
      el.className = "delay";
      chip.appendChild(el);
    }
    if (Number.isFinite(delay) && delay > 0) {
      el.textContent = `${delay}ms`;
      el.dataset.health = delay > 1500 ? "slow" : delay > 500 ? "warn" : "ok";
    } else {
      el.textContent = "超时";
      el.dataset.health = "bad";
    }
  });
}

// ─────────── Event wiring ───────────
$$("#trafficModeSeg button").forEach(b => {
  b.addEventListener("click", async () => {
    try {
      await api("PATCH", "/api/config/traffic", { mode: b.dataset.tmode });
      toast(`分流模式：${b.textContent}`);
      refresh();
    } catch (e) { toast(e.message, "bad", 3600); }
  });
});

const togglePatch = (id, path, body) => {
  $(id).addEventListener("change", async (e) => {
    try {
      await api("PATCH", path, body(e.target.checked));
      // 改了任何会触发 mihomo reload 的配置都重拉一次 /proxies —— 比如
      // auto_groups 开启会让 mihomo 多出 Auto / Fallback 这两个新组，
      // 不重新拉的话路由矩阵那一卡看不到变化。
      refresh();
      loadProxies();
    } catch (err) {
      toast(err.message, "bad", 3600);
      refresh();
    }
  });
};
togglePatch("chkAdblock",    "/api/config/traffic", v => ({ adblock: v }));
togglePatch("chkAutoGroups", "/api/config/traffic", v => ({ auto_groups: v }));
togglePatch("chkDNS",        "/api/config/dns",     v => ({ enabled: v }));

// Rulesets · 渲染成"开关 + 折叠详情"行，开关把当前所有 ruleset 状态作为整体 PATCH。
function renderRulesets(descriptors, current) {
  const list = $("rulesetsList");
  list.innerHTML = "";
  // 保留当前界面上的状态，给"开 / 关后立刻整组提交"用。
  const state = {
    china_direct: !!current.china_direct,
    lan_direct:   !!current.lan_direct,
    apple:        !!current.apple,
    nintendo:     !!current.nintendo,
    global:       !!current.global,
  };
  for (const d of descriptors) {
    const li = document.createElement("li");
    li.className = "row row-ruleset";

    const head = document.createElement("div");
    head.className = "ruleset-head";

    const text = document.createElement("div");
    text.className = "row-text";
    const verdictColor = d.verdict === "Proxy" ? "proxy" : d.verdict === "REJECT" ? "reject" : "direct";
    text.innerHTML = `
      <span class="row-label">${escapeHTML(d.label)}
        <span class="verdict-tag verdict-${verdictColor}">${escapeHTML(d.verdict)}</span>
      </span>
      <span class="row-help">${escapeHTML(d.key)} · ${escapeHTML(d.note)} · ${d.count} 条</span>`;

    const toggle = document.createElement("label");
    toggle.className = "ios-switch";
    toggle.setAttribute("aria-label", d.label + " 开关");
    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = !!d.enabled;
    input.addEventListener("change", async () => {
      state[d.key] = input.checked;
      try {
        await api("PATCH", "/api/config/rulesets", state);
        toast(`${d.label}：${input.checked ? "开启" : "关闭"}`);
        refresh();
        loadProxies();
      } catch (e) {
        toast(e.message, "bad", 3600);
        input.checked = !input.checked;
      }
    });
    const track = document.createElement("span");
    track.className = "ios-track";
    toggle.appendChild(input);
    toggle.appendChild(track);

    head.appendChild(text);
    head.appendChild(toggle);
    li.appendChild(head);

    // 折叠详情：展开后能看到完整规则原文，并支持复制。
    const det = document.createElement("details");
    det.className = "ruleset-details";
    const summary = document.createElement("summary");
    const rules = (d.rules && d.rules.length ? d.rules : d.sample) || [];
    const rulesText = rules.join("\n");
    summary.innerHTML = `<span>展开完整规则（${d.count} 条）</span>
      <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
        <path d="M9 6l6 6-6 6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>`;
    const body = document.createElement("div");
    body.className = "ruleset-body";
    const tools = document.createElement("div");
    tools.className = "ruleset-tools";
    const copy = document.createElement("button");
    copy.className = "btn btn-ghost btn-tiny";
    copy.type = "button";
    copy.innerHTML = `${copyIcon()}<span>复制全部</span>`;
    copy.addEventListener("click", () => copyText(rulesText, copy));
    const pre = document.createElement("pre");
    pre.className = "rule-preview";
    pre.textContent = rulesText || "无规则";
    tools.appendChild(copy);
    body.appendChild(tools);
    body.appendChild(pre);
    det.appendChild(summary);
    det.appendChild(body);
    li.appendChild(det);

    list.appendChild(li);
  }
}

// Source save
$("selSourceType").addEventListener("change", (e) => showSourcePanel(e.target.value));
$("btnManageSource").addEventListener("click", () => activatePanel("profile"));
$("btnSaveSource").addEventListener("click", async () => {
  const btn = $("btnSaveSource");
  btn.disabled = true;
  try {
    await api("POST", "/api/source", collectSource());
    toast("已保存，正在重载…");
    // mihomo 换源 = 整套 proxy / group 全换，必须重拉 /proxies；
    // 同时 reload 还要在 mihomo 那边消化新节点（约 1 秒），别太早拉。
    setTimeout(() => { refresh(); loadProxies(); }, 1500);
  } catch (e) {
    toast(e.message, "bad", 4000);
  } finally {
    btn.disabled = false;
  }
});

// Rule tabs
$$("#ruleTabs button").forEach(b => {
  b.addEventListener("click", () => {
    try {
      if (cachedState) cachedState.rules = collectRules();
    } catch (e) {
      toast(e.message, "bad", 3000);
      return;
    }
    activeRuleVerdict = b.dataset.verdict;
    $$("#ruleTabs button").forEach(x => x.setAttribute("aria-selected", x === b ? "true" : "false"));
    if (cachedState) renderRulesPane(cachedState);
  });
});

$("ruleGroupSelect").addEventListener("change", () => {
  const previous = activeRuleGroup || $("ruleGroupSelect").value;
  try {
    if (cachedState) cachedState.rules = collectRules(previous);
  } catch (e) {
    toast(e.message, "bad", 3000);
    return;
  }
  activeRuleGroup = $("ruleGroupSelect").value;
  $("ruleGroupCustom").value = "";
  if (cachedState) renderRulesPane(cachedState);
});

$("ruleGroupCustom").addEventListener("change", () => {
  const previous = activeRuleGroup || $("ruleGroupSelect").value;
  try {
    if (cachedState) cachedState.rules = collectRules(previous);
  } catch (e) {
    toast(e.message, "bad", 3000);
    return;
  }
  activeRuleGroup = $("ruleGroupCustom").value.trim() || $("ruleGroupSelect").value;
  if (cachedState) renderRulesPane(cachedState);
});

$("btnSaveRules").addEventListener("click", async () => {
  const btn = $("btnSaveRules");
  btn.disabled = true;
  try {
    await api("PUT", "/api/config/rules", collectRules());
    toast("规则已保存，正在重载…");
    setTimeout(() => { refresh(); loadProxies(); }, 1500);
  } catch (e) {
    toast(e.message, "bad", 4000);
  } finally {
    btn.disabled = false;
  }
});

// Ports
$("btnSavePorts").addEventListener("click", async () => {
  if (!confirm("修改端口需要完整重启服务，代理将短暂不可用（约 3-5 秒）。")) return;
  const btn = $("btnSavePorts");
  btn.disabled = true;
  try {
    await api("PATCH", "/api/config/ports", {
      mixed: parseInt($("portMixed").value, 10) || 0,
      redir: parseInt($("portRedir").value, 10) || 0,
      api:   parseInt($("portAPI").value, 10) || 0,
      dns:   parseInt($("portDNS").value, 10) || 0,
    });
    toast("端口已保存，正在重启…");
    setTimeout(() => { refresh(); startMihomoTraffic(); loadProxies(); }, 4000);
  } catch (e) {
    toast(e.message, "bad", 4000);
  } finally {
    btn.disabled = false;
  }
});

// Control · 关键修复 v2：
//   - 失败时**立刻**解锁按钮（不走 postDelay 的 setTimeout），避免出错后用户
//     还要等几秒才能重试。
//   - 成功时按 postDelay 后解锁 + 触发 refresh，匹配 mihomo 真实重启耗时。
//   - 用 dataset.busy 守门防止 setTimeout 还没触发又点一次造成双 toast。
async function runControl(btn, originalText, action, busyText, postDelay) {
  if (btn.dataset.busy === "1") return;
  btn.dataset.busy = "1";
  btn.disabled = true;
  btn.textContent = busyText;
  const unlock = () => {
    btn.disabled = false;
    btn.textContent = originalText;
    btn.dataset.busy = "0";
  };
  try {
    await action();
    toast(busyText.replace("…", "") + "完成");
    setTimeout(() => {
      refresh();
      startMihomoTraffic();
      loadProxies();
      unlock();
    }, postDelay);
  } catch (err) {
    toast("失败：" + err.message, "bad", 4000);
    unlock(); // 出错立刻解锁，让用户能马上重试
  }
}

$("btnReload").addEventListener("click", () => {
  runControl($("btnReload"), "Reload",
    () => api("POST", "/api/control/reload"),
    "Reloading…", 1200);
});

$("btnRestart").addEventListener("click", () => {
  if (!confirm("完整重启服务？代理将暂时不可用（约 3-5 秒）。")) return;
  runControl($("btnRestart"), "Restart",
    () => api("POST", "/api/control/restart"),
    "Restarting…", 4500);
});

$("btnRefresh").addEventListener("click", () => {
  const btn = $("btnRefresh");
  // 给个旋转动画作为"有反应"的视觉反馈 —— 否则用户点完毫无变化以为没生效。
  btn.classList.add("spinning");
  Promise.all([refresh(), loadProxies(), pollConnections()])
    .finally(() => setTimeout(() => btn.classList.remove("spinning"), 400));
  toast("已刷新");
});

$("btnVersion").addEventListener("click", openVersionModal);
$("sourceDrill").addEventListener("click", openSourceModal);
$("capTun").addEventListener("click", openTunModal);
$("capProxy").addEventListener("click", openProxyServiceModal);
for (const id of ["capTun", "capProxy"]) {
  $(id).addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      id === "capTun" ? openTunModal() : openProxyServiceModal();
    }
  });
}
$("cardDashboard").addEventListener("click", openConnectionsModal);
$("cardDashboard").addEventListener("keydown", (e) => {
  if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    openConnectionsModal();
  }
});

$$("#workspaceNav .nav-item").forEach(btn => {
  btn.addEventListener("click", () => activatePanel(btn.dataset.panel));
});

window.addEventListener("hashchange", () => {
  activatePanel(location.hash.replace(/^#/, ""), false);
});

// Copy buttons (delegated)
document.body.addEventListener("click", (e) => {
  const btn = e.target.closest(".copy-btn");
  if (!btn) return;
  const text = btn.dataset.copy;
  if (text) copyText(text, btn);
});

// ─────────── Boot ───────────
// 三个轮询 interval 用 id 管起来，document.hidden 时全部 clearInterval，
// visible 时重建。避免后台标签页打几千次 mihomo API 浪费电 + 流量。
let pollers = [];

function startPollers() {
  stopPollers();
  pollers.push(setInterval(refresh, 8000));
  pollers.push(setInterval(pollConnections, 2000));
  pollers.push(setInterval(loadProxies, 30000));
}

function stopPollers() {
  for (const id of pollers) clearInterval(id);
  pollers = [];
}

(async function boot() {
  activatePanel(location.hash.replace(/^#/, ""), false);
  await refresh();
  startMihomoTraffic();
  loadProxies();
  pollConnections();
  startPollers();
})();

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    stopMihomoTraffic();
    stopPollers();
  } else {
    refresh();
    startMihomoTraffic();
    loadProxies();
    startPollers();
  }
});
