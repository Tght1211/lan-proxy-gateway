const output = document.getElementById('output')

function log(data) {
  output.textContent = typeof data === 'string' ? data : JSON.stringify(data, null, 2)
}

function cfgFromForm() {
  return {
    proxy_source: document.getElementById('proxy_source').value,
    subscription_url: document.getElementById('subscription_url').value.trim(),
    proxy_config_file: document.getElementById('proxy_config_file').value.trim(),
    subscription_name: document.getElementById('subscription_name').value.trim() || 'subscription',
    ports: {
      mixed: Number(document.getElementById('port_mixed').value || 7890),
      redir: Number(document.getElementById('port_redir').value || 7892),
      api: Number(document.getElementById('port_api').value || 9090),
      dns: Number(document.getElementById('port_dns').value || 53),
    },
    regions: {
      enabled: document.getElementById('regions_enabled').checked,
      include: document.getElementById('regions_include').value.split(',').map(s => s.trim().toUpperCase()).filter(Boolean),
      auto_switch: document.getElementById('regions_auto_switch').checked,
      strategy: 'latency',
      mapping: {
        HK: ['香港', 'HK', 'Hong Kong'],
        JP: ['日本', 'JP', 'Tokyo', 'Osaka'],
        SG: ['新加坡', 'SG', 'Singapore'],
        US: ['美国', 'US', 'United States', 'Los Angeles', 'San Jose', 'Seattle'],
        TW: ['台湾', 'TW', 'Taiwan', 'Taipei'],
      },
    },
    ui: {
      listen: document.getElementById('ui_listen').value.trim() || '127.0.0.1:9091',
    },
  }
}

function fillForm(cfg) {
  document.getElementById('proxy_source').value = cfg.proxy_source || 'url'
  document.getElementById('subscription_url').value = cfg.subscription_url || ''
  document.getElementById('proxy_config_file').value = cfg.proxy_config_file || ''
  document.getElementById('subscription_name').value = cfg.subscription_name || 'subscription'
  document.getElementById('port_mixed').value = cfg.ports?.mixed ?? 7890
  document.getElementById('port_redir').value = cfg.ports?.redir ?? 7892
  document.getElementById('port_api').value = cfg.ports?.api ?? 9090
  document.getElementById('port_dns').value = cfg.ports?.dns ?? 53
  document.getElementById('regions_enabled').checked = !!cfg.regions?.enabled
  document.getElementById('regions_auto_switch').checked = cfg.regions?.auto_switch !== false
  document.getElementById('regions_include').value = (cfg.regions?.include || []).join(',')
  document.getElementById('ui_listen').value = cfg.ui?.listen || '127.0.0.1:9091'
}

async function loadConfig() {
  const res = await fetch('/api/config')
  const data = await res.json()
  fillForm(data)
}

async function saveConfig() {
  const cfg = cfgFromForm()
  const res = await fetch('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(cfg),
  })
  const data = await res.json()
  log(data)
  if (!res.ok) throw new Error(data.error || '保存失败')
}

async function refreshStatus() {
  try {
    const res = await fetch('/api/status')
    const data = await res.json()
    document.getElementById('status-text').textContent = data.running
      ? `运行中 (PID: ${data.pid})，当前节点: ${data.current_node || '未知'}，地区: ${(data.regions || []).join(',') || '未启用'}`
      : '未运行'
  } catch {
    document.getElementById('status-text').textContent = '状态获取失败'
  }
}

document.getElementById('save-btn').onclick = async () => {
  try { await saveConfig() } catch (e) { log(String(e)) }
}

document.getElementById('apply-btn').onclick = async () => {
  try {
    await saveConfig()
    const res = await fetch('/api/apply', { method: 'POST' })
    const data = await res.json()
    log(data)
  } catch (e) {
    log(String(e))
  }
}

document.getElementById('dry-run-btn').onclick = async () => {
  const res = await fetch('/api/switch-best?dry_run=1', { method: 'POST' })
  log(await res.json())
}

document.getElementById('switch-btn').onclick = async () => {
  const res = await fetch('/api/switch-best', { method: 'POST' })
  log(await res.json())
}

loadConfig().then(refreshStatus)
