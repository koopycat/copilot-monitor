// Reactive dashboard store using Svelte 5 runes.
// All state lives in a single $state object. Side effects (fetch, timer, observer)
// are wired up in init() and torn down in destroy().

import { exportHrefFor, fetchConfig, fetchPolicy, fetchPolicyModels, fetchUpstreams, loadDashboard, putPolicy } from '../lib/api';
import { drawChart } from '../lib/chart';
import { modelColor } from '../lib/colors';
import { dur, intl, usd } from '../lib/format';
import { PERIODS, periodDays, periodGran } from '../lib/periods';
import type {
  CostRow,
  CurrentSession,
  Granularity,
  ModelStats,
  PeriodKey,
  Policy,
  RouteConfig,
  Session,
  TimelineEntry,
} from '../lib/types';

const REFRESH_MS = matchMedia('(prefers-reduced-data: reduce)').matches
  ? 120_000
  : 30_000;

interface ModelRow extends ModelStats {
  total_usd: number;
  fallback: boolean;
  not_billed: boolean;
  cache_hit_pct: number;
  detail: string;
}

class DashboardStore {
  period: PeriodKey = $state('30d');
  gran: Granularity = $state('day');
  metric: 'tokens' | 'requests' = $state('tokens');

  cost = $state<number | null>(null);
  lastUpdated: string | null = $state(null);

  stats: ModelStats[] = $state([]);
  costRows: CostRow[] = $state([]);
  sessions: Session[] = $state([]);
  timeline: TimelineEntry[] = $state([]);
  current: CurrentSession = $state({ session: null, models: [] });

  upstream: string = $state('');
  upstreams: string[] = $state([]);
  routes: RouteConfig[] = $state([]);
  periodIsEmpty: boolean = $state(false);

  sessionPulse = $state(false);

  policy: Policy = $state({ mode: 'allow_all', models: [] });
  policyModels: string[] = $state([]);

  private _timer: ReturnType<typeof setInterval> | null = null;
  private _pulseTimer: ReturnType<typeof setTimeout> | null = null;
  private _abort: AbortController | null = null;
  private _ro: ResizeObserver | null = null;
  private _lastSessionID: number | null = null;
  private _lastSessionCount: number | null = null;
  private _chartEl: HTMLCanvasElement | null = null;
  private _keyHandler: ((e: KeyboardEvent) => void) | null = null;

  readonly periods = PERIODS;

  // Derived values

  get periodLabel(): string {
    const p = PERIODS.find((p) => p.key === this.period);
    return p ? p.label.toLowerCase() : this.period;
  }

  get totalRequests(): number {
    return this.stats.reduce((s, r) => s + r.requests, 0);
  }

  get maxToken(): number {
    return Math.max(1, ...this.stats.map((s) => s.total_tokens));
  }

  get modelRows(): ModelRow[] {
    const costMap = new Map<string, CostRow>();
    for (const r of this.costRows) {
      costMap.set(`${r.model}|${r.endpoint}|${r.upstream_host}`, r);
    }
    return this.stats.map((s) => {
      const cost: Partial<CostRow> = costMap.get(`${s.model}|${s.endpoint}|${s.upstream_host}`) ?? {};
      const cacheHit = s.prompt_tokens
        ? Math.round((s.cached_input_tokens / s.prompt_tokens) * 100)
        : 0;
      return {
        ...s,
        total_usd: cost.total_usd ?? 0,
        fallback: cost.fallback ?? false,
        not_billed: cost.not_billed ?? false,
        cache_hit_pct: cacheHit,
        detail: `input ${intl(s.prompt_tokens)}  cached ${intl(s.cached_input_tokens)}  write ${intl(s.cache_write_tokens)}  output ${intl(s.completion_tokens)}`,
      };
    });
  }

  get projectedText(): string {
    if (this.cost === null) return '-';
    const days = periodDays(this.period);
    if (days <= 1) return usd(this.cost);
    if (days >= 365) return `~${usd(this.cost / 12)}/mo`;
    return `~${usd((this.cost / days) * 30)}`;
  }

  get liveSessionActive(): boolean {
    return !!(this.current.session && this.current.session.active);
  }

  get sessionStatusText(): string {
    const session = this.current.session;
    if (!session) return 'idle';
    if (session.active) return 'active';
    const last = new Date(session.last_request_at).getTime();
    if (!last) return 'idle';
    return `idle ${dur((Date.now() - last) / 1000)} ago`;
  }

  get sessionDurationText(): string {
    const session = this.current.session;
    if (!session) return '-';
    const start = new Date(session.started_at).getTime();
    const end = session.active
      ? Date.now()
      : new Date(session.last_request_at).getTime();
    if (!start || !end) return '-';
    return dur((end - start) / 1000);
  }

  get sessionModelsText(): string {
    if (!this.current.models.length) return '-';
    return this.current.models
      .map((m) => `${m.model} (${intl(m.requests)})`)
      .join(', ');
  }

  get exportHref(): string {
    return exportHrefFor(this.period, this.upstream || undefined);
  }

  // Actions

  switchPeriod(p: PeriodKey): void {
    if (p === this.period) return;
    this.period = p;
    this.gran = periodGran(p);
    this.runWithViewTransition(() => this.load());
  }

  switchGran(g: Granularity): void {
    if (g === this.gran) return;
    this.gran = g;
    this.runWithViewTransition(() => this.load());
  }

  switchMetric(m: 'tokens' | 'requests'): void {
    if (m === this.metric) return;
    this.metric = m;
    this.runWithViewTransition(() => this.redrawChart());
  }

  switchUpstream(value: string): void {
    if (value === this.upstream) return;
    this.upstream = value;
    this.runWithViewTransition(() => this.load());
  }

  barW(val: number, max: number): number {
    const pct = max ? val / max : 0;
    return Math.max(1, Math.round(pct * 60));
  }

  init(canvas: HTMLCanvasElement | null): void {
    this._chartEl = canvas;
    this.gran = periodGran(this.period);
    this.load();
    this.startTimer();

    const chartWrap = document.querySelector('.chart-wrap');
    if (chartWrap && 'ResizeObserver' in window) {
      this._ro = new ResizeObserver(() => this.redrawChart());
      this._ro.observe(chartWrap);
    }

    this._keyHandler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'SELECT' || tag === 'TEXTAREA') return;
      if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
        const idx = PERIODS.findIndex((p) => p.key === this.period);
        if (e.key === 'ArrowLeft' && idx > 0) {
          this.switchPeriod(PERIODS[idx - 1].key);
        } else if (e.key === 'ArrowRight' && idx < PERIODS.length - 1) {
          this.switchPeriod(PERIODS[idx + 1].key);
        }
      }
    };
    window.addEventListener('keydown', this._keyHandler);
  }

  destroy(): void {
    this.stopTimer();
    this._ro?.disconnect();
    this._ro = null;
    this._abort?.abort();
    this._abort = null;
    if (this._keyHandler) {
      window.removeEventListener('keydown', this._keyHandler);
      this._keyHandler = null;
    }
  }

  async load(): Promise<void> {
    this._abort?.abort();
    this._abort = new AbortController();
    const { signal } = this._abort;
    try {
      const data = await loadDashboard(this.period, this.gran, signal, this.upstream || undefined);
      if (signal.aborted) return;
      this.stats = data.stats;
      this.cost = data.cost.total_usd;
      this.costRows = data.cost.rows;
      this.sessions = data.sessions;
      this.timeline = data.timeline;
      this.periodIsEmpty = data.stats.length === 0;
      this.updateCurrentSession(data.current);
      this.redrawChart();
      this.lastUpdated = new Date().toLocaleTimeString();

      if (this.upstreams.length === 0) {
        fetchUpstreams(signal).then((hosts) => {
          if (!signal.aborted) this.upstreams = hosts;
        }).catch(() => {});
      }
      if (this.routes.length === 0) {
        fetchConfig(signal).then((cfg) => {
          if (!signal.aborted) this.routes = cfg.routes;
        }).catch(() => {});
      }

      this.refreshPolicy().catch(() => {});
      if (this.policyModels.length === 0) {
        fetchPolicyModels(signal).then((models) => {
          if (!signal.aborted && models) this.policyModels = models;
        }).catch(() => {});
      }
    } catch (e) {
      if (e instanceof Error && e.name === 'AbortError') return;
      this.lastUpdated = null;
      console.error(e);
    }
  }

  private updateCurrentSession(current: CurrentSession): void {
    const session = current.session;
    if (
      session &&
      this._lastSessionID === session.id &&
      this._lastSessionCount !== null &&
      session.request_count > this._lastSessionCount
    ) {
      this.triggerSessionPulse();
    }
    this.current = current;
    this._lastSessionID = session ? session.id : null;
    this._lastSessionCount = session ? session.request_count : null;
  }

  private triggerSessionPulse(): void {
    this.sessionPulse = false;
    if (this._pulseTimer) clearTimeout(this._pulseTimer);
    requestAnimationFrame(() => {
      this.sessionPulse = true;
      this._pulseTimer = setTimeout(() => {
        this.sessionPulse = false;
        this._pulseTimer = null;
      }, 2000);
    });
  }

  private startTimer(): void {
    this.stopTimer();
    this._timer = setInterval(() => this.load(), REFRESH_MS);
  }

  private stopTimer(): void {
    if (this._timer) {
      clearInterval(this._timer);
      this._timer = null;
    }
    if (this._pulseTimer) {
      clearTimeout(this._pulseTimer);
      this._pulseTimer = null;
    }
  }

  private redrawChart(): void {
    if (!this._chartEl) return;
    drawChart(this._chartEl, this.timeline, this.gran, modelColor, this.metric);
  }

  async refreshPolicy(): Promise<void> {
    const ctrl = new AbortController();
    const p = await fetchPolicy(ctrl.signal);
    if (p) {
      if (!p.models) p.models = [];
      this.policy = p;
    }
  }

  async savePolicy(policy: Policy): Promise<boolean> {
    const ctrl = new AbortController();
    const result = await putPolicy(policy, ctrl.signal);
    if (result) { this.policy = result; return true; }
    return false;
  }

  private runWithViewTransition(fn: () => void): void {
    type DocWithVT = Document & {
      startViewTransition?: (cb: () => void) => unknown;
    };
    const doc = document as DocWithVT;
    if (doc.startViewTransition) {
      doc.startViewTransition(() => fn());
    } else {
      fn();
    }
  }
}

export const dashboard = new DashboardStore();
