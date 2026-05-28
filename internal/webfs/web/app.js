/* ============================================================
   Theme
   ============================================================ */
const html = document.documentElement;
const themeToggle = document.getElementById('themeToggle');

function getPreferredTheme() {
  const stored = localStorage.getItem('health-theme');
  if (stored === 'dark' || stored === 'light') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme) {
  html.setAttribute('data-theme', theme);
  localStorage.setItem('health-theme', theme);
}

function toggleTheme() {
  const current = html.getAttribute('data-theme') || 'light';
  applyTheme(current === 'dark' ? 'light' : 'dark');
}

// Apply on load, then wire button
applyTheme(getPreferredTheme());
themeToggle.addEventListener('click', toggleTheme);

// Follow system preference changes when no manual override exists
window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', e => {
  if (!localStorage.getItem('health-theme')) {
    applyTheme(e.matches ? 'dark' : 'light');
  }
});

/* ============================================================
   Data Fetching
   ============================================================ */
const REFRESH_MS = 30_000;

async function fetchStatus() {
  const resp = await fetch('/api/v1/status');
  if (!resp.ok) throw new Error(`HTTP ${resp.status} ${resp.statusText}`);
  return resp.json();
}

/* ============================================================
   Rendering
   ============================================================ */
function renderBanner(overall) {
  const banner = document.getElementById('overallBanner');
  const icon   = document.getElementById('bannerIcon');
  const title  = document.getElementById('bannerTitle');
  const sub    = document.getElementById('bannerSub');

  banner.className = 'banner ' + (overall === 'up' ? 'banner-ok' : 'banner-down');

  if (overall === 'up') {
    icon.textContent  = '✅';
    title.textContent = '一切运行正常';
    sub.textContent   = '所有系统均按预期运行。';
  } else {
    icon.textContent  = '⚠️';
    title.textContent = '部分服务异常';
    sub.textContent   = '存在一个或多个服务不可用，请检查下方详情。';
  }
}

/**
 * Build a timeline bar strip from an array of StatusPoint objects.
 * Shows the last MAX_BARS checks; pads left with empty bars if fewer exist.
 */
function buildTimeline(timeline) {
  const MAX_BARS = 45;
  const points = Array.isArray(timeline) ? timeline.slice(-MAX_BARS) : [];
  const pad = MAX_BARS - points.length;

  let html = '';
  for (let i = 0; i < pad; i++) {
    html += '<span class="bar"></span>';
  }
  for (const pt of points) {
    const cls  = pt.status === 'up' ? 'bar-up' : 'bar-down';
    const time = new Date(pt.at).toLocaleString('zh-CN');
    const tip  = `${time} — ${pt.status === 'up' ? '正常' : '异常'}`;
    html += `<span class="bar ${cls}" title="${tip}"></span>`;
  }
  return `<div class="timeline">${html}</div>`;
}

function formatAvail(pct) {
  if (pct == null || isNaN(pct)) return '—';
  return pct.toFixed(2) + '%';
}

function renderServices(services) {
  const list = document.getElementById('serviceList');

  if (!Array.isArray(services) || services.length === 0) {
    list.innerHTML = '<div class="loading-placeholder">暂无服务数据，请检查探针配置。</div>';
    return;
  }

  list.innerHTML = services.map(svc => {
    const dotCls = svc.current_status === 'up' ? 'dot-up' : 'dot-down';
    const avail  = formatAvail(svc.availability_pct);
    const name   = escapeHTML(svc.name || svc.probe_id);
    return `
      <div class="service-row">
        <span class="service-dot ${dotCls}"></span>
        <span class="service-name" title="${name}">${name}</span>
        ${buildTimeline(svc.timeline)}
        <span class="service-avail">${avail}</span>
      </div>`;
  }).join('');
}

function updateLastUpdated(iso) {
  const el = document.getElementById('lastUpdated');
  if (!iso) { el.textContent = ''; return; }
  const d = new Date(iso);
  el.textContent = '最近更新: ' + d.toLocaleTimeString('zh-CN');
}

function escapeHTML(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

/* ============================================================
   Main Refresh Loop
   ============================================================ */
async function refresh() {
  try {
    const data = await fetchStatus();
    renderBanner(data.overall);
    renderServices(data.services);
    updateLastUpdated(data.updated_at);
  } catch (err) {
    console.error('status fetch failed:', err);
    const banner = document.getElementById('overallBanner');
    banner.className = 'banner banner-down';
    document.getElementById('bannerIcon').textContent  = '❌';
    document.getElementById('bannerTitle').textContent = '无法连接到服务器';
    document.getElementById('bannerSub').textContent   = err.message;
  }
}

refresh();
setInterval(refresh, REFRESH_MS);
