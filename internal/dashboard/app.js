import { createApp } from 'petite-vue';
import { drawChart } from './chart.js';

const COLORS = ['#58a6ff','#3fb950','#d29922','#f85149','#bc8cff','#79c0ff','#56d364'];
const colorMap = new Map();
export function modelColor(model, i) {
  if (!colorMap.has(model)) colorMap.set(model, COLORS[i % COLORS.length]);
  return colorMap.get(model);
}

const USD  = new Intl.NumberFormat('en-US', { style:'currency', currency:'USD', minimumFractionDigits:2, maximumFractionDigits:2 });
const usd  = n => n == null ? '—' : n < 0.01 ? '<$0.01' : USD.format(n);
const ms   = n => n == null ? '—' : n < 1 ? '<1ms' : n < 1000 ? Math.round(n)+'ms' : (n/1000).toFixed(1)+'s';
const dur  = s => s < 60 ? s+'s' : Math.round(s/60)+'m';
const intl = n => n == null ? '0' : n.toLocaleString();

function App() {
  const S = {
    gran: 'day',
    cost: null,
    lastUpdated: null,
    stats: [],
    costRows: [],
    sessions: [],
    timeline: [],
    _timer: null,

    get maxToken() {
      return Math.max(1, ...this.stats.map(s => s.total_tokens));
    },
    get projectedText() {
      if (this.cost === null) return '—';
      const days = new Set(this.timeline.map(t => t.date)).size || 1;
      return '~' + usd(this.cost / days * 30);
    },
    get latencyText() {
      const total = this.stats.reduce((s, r) => s + r.requests, 0);
      if (!total) return '—';
      const sum = this.stats.reduce((s, r) => s + r.avg_latency_ms * r.requests, 0);
      return ms(sum / total);
    },

    switchGran(g) {
      if (g === this.gran) return;
      this.gran = g;
      this.load();
    },
    barW(val, max) {
      const pct = max ? val / max : 0;
      return Math.max(1, Math.round(pct * 60));
    },

    async load() {
      try {
        const params = this.gran === 'hour' ? 'since=24h&granularity=hour' : 'since=30d&granularity=day';
        const [stats, cost, sessions, timeline] = await Promise.all([
          fetch('/api/stats?since=30d').then(r=>r.json()),
          fetch('/api/cost?since=30d').then(r=>r.json()),
          fetch('/api/sessions?since=30d&limit=20').then(r=>r.json()),
          fetch('/api/stats/timeline?'+params).then(r=>r.json()),
        ]);
        this.stats = stats || [];
        this.cost = cost.total_usd || 0;
        this.costRows = cost.rows || [];
        this.sessions = sessions || [];
        this.timeline = timeline || [];
        drawChart(document.getElementById('chart'), this.timeline, this.gran, colorMap, COLORS, modelColor);
        this.lastUpdated = new Date().toLocaleTimeString();
      } catch(e) {
        this.lastUpdated = null;
        console.error(e);
      }
    },
    stopTimer() {
      if (this._timer) { clearInterval(this._timer); this._timer = null; }
    },
    startTimer() {
      this.stopTimer();
      this._timer = setInterval(() => this.load(), 30000);
    },
  };
  return S;
}

createApp({
  App,
  $usd: usd,
  $ms: ms,
  $dur: dur,
  $intl: intl,
  modelColor,
}).mount('#app');
