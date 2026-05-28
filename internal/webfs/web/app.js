const REFRESH_MS = 30_000;
const DEFAULT_RANGE = '6h';
let activeRange = DEFAULT_RANGE;

const html = document.documentElement;
const themeToggle = document.getElementById('themeToggle');
const overallBanner = document.getElementById('overallBanner');
const lastUpdated = document.getElementById('lastUpdated');
const serviceList = document.getElementById('serviceList');
const rangeSwitch = document.getElementById('rangeSwitch');

function getPreferredTheme() {
  const stored = localStorage.getItem('health-theme');
  if (stored === 'dark' || stored === 'light') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme) {
  html.setAttribute('data-theme', theme);
  localStorage.setItem('health-theme', theme);
  themeToggle.textContent = theme === 'dark' ? '☀️' : '🌙';
}

function toggleTheme() {
  const current = html.getAttribute('data-theme') || 'light';
  applyTheme(current === 'dark' ? 'light' : 'dark');
}

applyTheme(getPreferredTheme());
themeToggle.addEventListener('click', toggleTheme);

window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
  if (!localStorage.getItem('health-theme')) {
    applyTheme(e.matches ? 'dark' : 'light');
  }
});

function formatTime(value) {
  if (!value) return '--';
  return new Date(value).toLocaleString('zh-CN');
}

function formatStatus(status) {
  return status === 'up' ? '正常' : '异常';
}

function formatLatency(latency) {
  if (latency === undefined || latency === null) return '--';
  return `${latency} ms`;
}

function escapeHTML(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

function updateRangeSwitch(range) {
  activeRange = range || DEFAULT_RANGE;
  document.querySelectorAll('.range-btn').forEach(button => {
    button.classList.toggle('active', button.dataset.range === activeRange);
  });
}

function buildTimeline(timeline) {
  const points = Array.isArray(timeline) ? timeline : [];
  if (!points.length) {
    return '<div class="timeline empty">暂无历史数据</div>';
  }

  const bars = points.map(pt => {
    const cls = pt.status === 'up' ? 'bar-up' : 'bar-down';
    const tip = [
      `时间：${formatTime(pt.at)}`,
      `状态：${formatStatus(pt.status)}`,
      `耗时：${formatLatency(pt.latency_ms)}`,
      pt.node_name ? `节点：${pt.node_name}` : '',
      pt.message ? `信息：${pt.message}` : '',
    ].filter(Boolean).join('\n');
    return `<span class="bar ${cls}" title="${escapeHTML(tip)}"></span>`;
  }).join('');

  return `<div class="timeline">${bars}</div>`;
}

function formatAvail(pct) {
  if (pct == null || Number.isNaN(pct)) return '—';
  return `${pct.toFixed(2)}%`;
}

function renderServices(services) {
  if (!Array.isArray(services) || services.length === 0) {
    serviceList.innerHTML = '<div class="empty-state">暂无服务数据，请检查探针配置。</div>';
    return;
  }

  serviceList.innerHTML = services.map(svc => {
    const statusCls = svc.current_status === 'up' ? 'status-up' : 'status-down';
    const name = escapeHTML(svc.name || svc.probe_id);
    const msg = escapeHTML(svc.message || '--');
    return `
      <article class="service-card">
        <div class="service-header">
          <div>
            <h4>${name}</h4>
            <p class="service-meta">${escapeHTML((svc.type || '').toUpperCase())} · 最后检查 ${formatTime(svc.last_checked_at)}</p>
          </div>
          <span class="status-badge ${statusCls}">${formatStatus(svc.current_status)}</span>
        </div>
        <div class="service-metrics">
          <div>
            <span class="metric-label">可用率</span>
            <strong>${formatAvail(svc.availability_pct)}</strong>
          </div>
          <div>
            <span class="metric-label">消息</span>
            <strong>${msg}</strong>
          </div>
        </div>
        ${buildTimeline(svc.timeline)}
      </article>`;
  }).join('');
}

function updateLastUpdated(iso) {
  lastUpdated.textContent = iso ? `最近更新：${formatTime(iso)}` : '';
}

function renderBanner(overall) {
  overallBanner.textContent = overall === 'up' ? '所有服务运行正常' : '存在异常服务';
  overallBanner.className = overall === 'up' ? 'banner-up' : 'banner-down';
}

async function fetchStatus() {
  const resp = await fetch(`/api/v1/status?range=${encodeURIComponent(activeRange)}`);
  if (!resp.ok) throw new Error(`HTTP ${resp.status} ${resp.statusText}`);
  return resp.json();
}

async function refresh() {
  try {
    const data = await fetchStatus();
    renderBanner(data.overall);
    renderServices(data.services);
    updateLastUpdated(data.updated_at);
    updateRangeSwitch(data.range || activeRange);
  } catch (err) {
    console.error('status fetch failed:', err);
    overallBanner.textContent = '无法连接到服务器';
    overallBanner.className = 'banner-down';
    lastUpdated.textContent = err.message;
  }
}

rangeSwitch?.addEventListener('click', event => {
  const button = event.target.closest('.range-btn');
  if (!button) return;
  const nextRange = button.dataset.range || DEFAULT_RANGE;
  if (nextRange === activeRange) return;
  updateRangeSwitch(nextRange);
  refresh();
});

updateRangeSwitch(DEFAULT_RANGE);
refresh();
setInterval(refresh, REFRESH_MS);
