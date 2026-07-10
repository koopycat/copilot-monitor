// API client with AbortController for race conditions

import type {
  CostResponse,
  CurrentSession,
  ModelStats,
  PeriodKey,
  Policy,
  RouteConfig,
  Session,
  TimelineEntry,
  Granularity,
} from './types';
import { buildParams, periodQuery } from './periods';

async function safeFetch<T>(url: string, signal: AbortSignal): Promise<T | null> {
  try {
    const r = await fetch(url, { signal });
    if (!r.ok) return null;
    return (await r.json()) as T;
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') throw e;
    return null;
  }
}

export interface DashboardData {
  stats: ModelStats[];
  cost: CostResponse;
  sessions: Session[];
  timeline: TimelineEntry[];
  current: CurrentSession;
}

export async function loadDashboard(
  period: PeriodKey,
  granularity: Granularity,
  signal: AbortSignal,
  upstream?: string,
): Promise<DashboardData> {
  const pq = periodQuery(period);

  const extra: Record<string, string> = {};
  if (upstream) extra.upstream = upstream;

  const sinceParams = buildParams({
    since: pq.since,
    ...(pq.until ? { until: pq.until } : {}),
    ...extra,
  });
  const timelineParams = buildParams({
    since: pq.since,
    ...(pq.until ? { until: pq.until } : {}),
    granularity,
    ...extra,
  });

  const [stats, cost, sessions, timeline, current] = await Promise.all([
    safeFetch<ModelStats[]>(`/api/stats?${sinceParams}`, signal),
    safeFetch<CostResponse>(`/api/cost?${sinceParams}`, signal),
    safeFetch<Session[]>(`/api/sessions?${sinceParams}&limit=20`, signal),
    safeFetch<TimelineEntry[]>(`/api/stats/timeline?${timelineParams}`, signal),
    safeFetch<CurrentSession>(`/api/session/current`, signal),
  ]);

  return {
    stats: stats ?? [],
    cost: cost ?? { total_usd: 0, rows: [] },
    sessions: sessions ?? [],
    timeline: timeline ?? [],
    current: current ?? { session: null, models: [] },
  };
}

export function exportHrefFor(period: PeriodKey, upstream?: string): string {
  const pq = periodQuery(period);
  const params = buildParams({
    since: pq.since,
    ...(pq.until ? { until: pq.until } : {}),
    ...(upstream ? { upstream } : {}),
  });
  return `/api/export?${params}`;
}

export async function fetchUpstreams(signal: AbortSignal): Promise<string[]> {
  try {
    const r = await fetch('/api/upstreams', { signal });
    if (!r.ok) return [];
    return (await r.json()) as string[];
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') throw e;
    return [];
  }
}

export async function fetchConfig(signal: AbortSignal): Promise<{ routes: RouteConfig[] }> {
  try {
    const r = await fetch('/api/config', { signal });
    if (!r.ok) return { routes: [] };
    return (await r.json()) as { routes: RouteConfig[] };
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') throw e;
    return { routes: [] };
  }
}

export async function fetchPolicy(signal: AbortSignal): Promise<Policy | null> {
  return safeFetch<Policy>('/api/policy', signal);
}

export async function putPolicy(policy: Policy, signal: AbortSignal): Promise<Policy | null> {
  try {
    const r = await fetch('/api/policy', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(policy),
      signal,
    });
    if (!r.ok) return null;
    return (await r.json()) as Policy;
  } catch (e) {
    if (e instanceof Error && e.name === 'AbortError') throw e;
    return null;
  }
}

export async function fetchPolicyModels(signal: AbortSignal): Promise<string[] | null> {
  return safeFetch<string[]>('/api/policy/models', signal);
}
