import Alpine from 'alpinejs';
import { drawChart } from './chart.js';

const COLORS = ['#58a6ff','#3fb950','#d29922','#f85149','#bc8cff','#79c0ff','#56d364'];
const colorMap = new Map();
function modelColor(model, i) {
  if (!colorMap.has(model)) colorMap.set(model, COLORS[i % COLORS.length]);
  return colorMap.get(model);
}

const USD  = new Intl.NumberFormat('en-US', { style:'currency', currency:'USD', minimumFractionDigits:2, maximumFractionDigits:2 });
const usd  = n => n == null ? '-' : n < 0.01 ? '<$0.01' : USD.format(n);
const ms   = n => n == null ? '-' : n < 1 ? '<1ms' : n < 1000 ? Math.round(n)+'ms' : (n/1000).toFixed(1)+'s';
const dur  = s => {
  s = Math.max(0, Math.round(s || 0));
  if (s < 60) return s+'s';
  if (s < 3600) return Math.floor(s/60)+'m '+String(s % 60).padStart(2, '0')+'s';
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return h+'h '+String(m).padStart(2, '0')+'m';
};
const intl = n => n == null ? '0' : n.toLocaleString();

async function safeFetch(url, signal) {
  try {
    const r = await fetch(url, { signal });
    if (!r.ok) return null;
    return await r.json();
  } catch (e) {
    if (e.name === 'AbortError') throw e;
    return null;
  }
}

function midnightToday() {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate());
}
function midnightYesterday() {
  const t = midnightToday();
  return new Date(t.getFullYear(), t.getMonth(), t.getDate() - 1);
}
function toISO(d) {
  return d.toISOString().slice(0, 19) + 'Z';
}

const PERIODS = [
  { key: 'today', label: 'Today' },
  { key: 'yesterday', label: 'Yesterday' },
  { key: '7d', label: '7d' },
  { key: '30d', label: '30d' },
  { key: '90d', label: '90d' },
  { key: '365d', label: '365d' },
];

function periodQuery(period) {
  switch (period) {
    case 'today':
      return { since: toISO(midnightToday()), until: '' };
    case 'yesterday':
      return { since: toISO(midnightYesterday()), until: toISO(midnightToday()) };
    default:
      return { since: period, until: '' };
  }
}

function periodGran(period) {
  return (period === 'today' || period === 'yesterday') ? 'hour' : 'day';
}

function periodDays(period) {
  switch (period) {
    case 'today': return 1;
    case 'yesterday': return 1;
    case '7d': return 7;
    case '30d': return 30;
    case '90d': return 90;
    case '365d': return 365;
    default: return 30;
  }
}

const REFRESH_MS = matchMedia('(prefers-reduced-data: reduce)').matches ? 120_000 : 30_000;

function vt(fn) {
  if (document.startViewTransition) {
    document.startViewTransition(() => { fn(); });
  } else {
    fn();
  }
}

function App() {
  const S = {
    period: '30d',
    periods: PERIODS,
    gran: 'day',
    metric: 'tokens',
    cost: null,
    lastUpdated: null,
    stats: [],
    costRows: [],
    sessions: [],
    timeline: [],
    currentSession: null,
    currentSessionModels: [],
    sessionPulse: false,
    exportHref: '/api/export?since=30d',
    _lastSessionID: null,
    _lastSessionCount: null,
    _pulseTimer: null,
    _timer: null,
    _ro: null,
    _abort: null,
    _initDone: false,

    init() {
      if (this._initDone) return;
      this._initDone = true;
      this.gran = periodGran(this.period);
      this._syncPeriodDerived();
      this.load();
      this.startTimer();
      const chartWrap = document.querySelector('.chart-wrap');
      if (chartWrap) {
        this._ro = new ResizeObserver(() => {
          drawChart(document.getElementById('chart'), this.timeline, this.gran, modelColor, this.metric);
        });
        this._ro.observe(chartWrap);
      }
    },
    get maxToken() {
      return Math.max(1, ...this.stats.map(s => s.total_tokens));
    },
    get totalRequests() {
      return this.stats.reduce((s, r) => s + r.requests, 0);
    },
    get modelRows() {
      const costMap = new Map();
      for (const r of this.costRows) {
        costMap.set(r.model + '|' + r.endpoint, r);
      }
      return this.stats.map(s => {
        const cost = costMap.get(s.model + '|' + s.endpoint) || {};
        const cacheHit = s.prompt_tokens ? Math.round((s.cached_input_tokens / s.prompt_tokens) * 100) : 0;
        return {
          model: s.model,
          endpoint: s.endpoint,
          requests: s.requests,
          total_tokens: s.total_tokens,
          total_usd: cost.total_usd || 0,
          fallback: cost.fallback || false,
          not_billed: cost.not_billed || false,
          cache_hit_pct: cacheHit,
          avg_latency_ms: s.avg_latency_ms,
          detail: 'input ' + intl(s.prompt_tokens) + '  cached ' + intl(s.cached_input_tokens) + '  write ' + intl(s.cache_write_tokens) + '  output ' + intl(s.completion_tokens),
        };
      });
    },
    get periodLabel() {
      const p = PERIODS.find(p => p.key === this.period);
      return p ? p.label.toLowerCase() : this.period;
    },
    get projectedText() {
      if (this.cost === null) return '-';
      const days = periodDays(this.period);
      if (days <= 1) return usd(this.cost);
      if (days >= 365) return '~' + usd(this.cost / 12) + '/mo';
      return '~' + usd(this.cost / days * 30);
    },
    get latencyText() {
      const total = this.stats.reduce((s, r) => s + r.requests, 0);
      if (!total) return '-';
      const sum = this.stats.reduce((s, r) => s + r.avg_latency_ms * r.requests, 0);
      return ms(sum / total);
    },
    _syncPeriodDerived() {
      const pq = periodQuery(this.period);
      const params = new URLSearchParams({ since: pq.since });
      if (pq.until) params.set('until', pq.until);
      this.exportHref = '/api/export?' + params;
    },
    get liveSessionActive() {
      return !!(this.currentSession && this.currentSession.active);
    },
    get sessionStatusText() {
      if (!this.currentSession) return 'idle';
      if (this.currentSession.active) return 'active';
      const last = new Date(this.currentSession.last_request_at).getTime();
      if (!last) return 'idle';
      return 'idle '+dur((Date.now() - last) / 1000)+' ago';
    },
    get sessionDurationText() {
      if (!this.currentSession) return '-';
      const start = new Date(this.currentSession.started_at).getTime();
      const end = this.currentSession.active ? Date.now() : new Date(this.currentSession.last_request_at).getTime();
      if (!start || !end) return '-';
      return dur((end - start) / 1000);
    },
    get sessionModelsText() {
      if (!this.currentSessionModels.length) return '-';
      return this.currentSessionModels.map(m => m.model+' ('+intl(m.requests)+')').join(', ');
    },

    switchPeriod(p) {
      if (p === this.period) return;
      this.period = p;
      this.gran = periodGran(p);
      this._syncPeriodDerived();
      vt(() => this.load());
    },
    switchGran(g) {
      if (g === this.gran) return;
      this.gran = g;
      vt(() => this.load());
    },
    switchMetric(m) {
      if (m === this.metric) return;
      this.metric = m;
      vt(() => {
        drawChart(document.getElementById('chart'), this.timeline, this.gran, modelColor, this.metric);
      });
    },
    barW(val, max) {
      const pct = max ? val / max : 0;
      return Math.max(1, Math.round(pct * 60));
    },

    async load() {
      this._abort?.abort();
      this._abort = new AbortController();
      const { signal } = this._abort;
      try {
        const pq = periodQuery(this.period);
        const since = pq.since;

        const sinceParams = new URLSearchParams({ since });
        if (pq.until) sinceParams.set('until', pq.until);

        const timelineParams = new URLSearchParams({ since });
        if (pq.until) timelineParams.set('until', pq.until);
        timelineParams.set('granularity', this.gran);

        const fetches = [
          safeFetch('/api/stats?' + sinceParams, signal),
          safeFetch('/api/cost?' + sinceParams, signal),
          safeFetch('/api/sessions?' + sinceParams + '&limit=20', signal),
          safeFetch('/api/stats/timeline?' + timelineParams, signal),
          safeFetch('/api/session/current', signal),
        ];

        const results = await Promise.all(fetches);
        if (signal.aborted) return;

        let idx = 0;
        this.stats = results[idx++] || [];
        const cost = results[idx++] || {};
        this.cost = cost.total_usd || 0;
        this.costRows = cost.rows || [];
        this.sessions = results[idx++] || [];
        this.timeline = results[idx++] || [];
        const current = results[idx++] || {};

        this.updateCurrentSession(current);
        drawChart(document.getElementById('chart'), this.timeline, this.gran, modelColor, this.metric);
        this.lastUpdated = new Date().toLocaleTimeString();
      } catch(e) {
        if (e.name === 'AbortError') return;
        this.lastUpdated = null;
        console.error(e);
      }
    },
    updateCurrentSession(current) {
      const session = current && current.session ? current.session : null;
      const models = current && current.models ? current.models : [];
      if (session && this._lastSessionID === session.id && this._lastSessionCount !== null && session.request_count > this._lastSessionCount) {
        this.triggerSessionPulse();
      }
      this.currentSession = session;
      this.currentSessionModels = models;
      this._lastSessionID = session ? session.id : null;
      this._lastSessionCount = session ? session.request_count : null;
    },
    triggerSessionPulse() {
      this.sessionPulse = false;
      if (this._pulseTimer) clearTimeout(this._pulseTimer);
      requestAnimationFrame(() => {
        this.sessionPulse = true;
        this._pulseTimer = setTimeout(() => { this.sessionPulse = false; this._pulseTimer = null; }, 2000);
      });
    },
    stopTimer() {
      if (this._timer) { clearInterval(this._timer); this._timer = null; }
      if (this._pulseTimer) { clearTimeout(this._pulseTimer); this._pulseTimer = null; }
      if (this._ro) { this._ro.disconnect(); this._ro = null; }
      if (this._abort) { this._abort.abort(); this._abort = null; }
    },
    startTimer() {
      if (this._timer) { clearInterval(this._timer); this._timer = null; }
      this._timer = setInterval(() => this.load(), REFRESH_MS);
    },
  };
  return S;
}

document.addEventListener('alpine:init', () => {
  Alpine.data('dashboard', App);
  Alpine.magic('usd', () => usd);
  Alpine.magic('ms', () => ms);
  Alpine.magic('dur', () => dur);
  Alpine.magic('intl', () => intl);
  Alpine.magic('modelColor', () => modelColor);
});
Alpine.start();

// Initialize the dashboard component after Alpine has set up reactivity.
// Avoid x-init here because Alpine 3.14.9 evaluates x-init expressions twice,
// and a returned cleanup function is invoked immediately which aborts the
// initial in-flight data fetch.
document.addEventListener('alpine:initialized', () => {
  const root = document.getElementById('app');
  if (root && root._x_dataStack) root._x_dataStack[0].init();
});
window.addEventListener('beforeunload', () => {
  const root = document.getElementById('app');
  if (root && root._x_dataStack) root._x_dataStack[0].stopTimer();
});
