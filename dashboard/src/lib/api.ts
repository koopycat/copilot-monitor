// API client with AbortController for race conditions

import type {
  Anomaly,
  CostResponse,
  CurrentSession,
  ModelStats,
  PeriodKey,
  Policy,
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
  timeline: TimelineEntry[];
  current: CurrentSession;
  anomalies: Anomaly[];
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

  const [stats, cost, timeline, current, anomalies] = await Promise.all([
    safeFetch<ModelStats[]>(`/api/stats?${sinceParams}`, signal),
    safeFetch<CostResponse>(`/api/cost?${sinceParams}`, signal),
    safeFetch<TimelineEntry[]>(`/api/stats/timeline?${timelineParams}`, signal),
    safeFetch<CurrentSession>(`/api/session/current`, signal),
    fetchAnomalies(signal),
  ]);

  return {
    stats: stats ?? [],
    cost: cost ?? { total_usd: 0, rows: [] },
    timeline: timeline ?? [],
    current: current ?? { session: null, models: [] },
    anomalies: anomalies ?? [],
  };
}

export interface SessionFilters {
  since?: string;
  until?: string;
  project?: string;
}

const SESSION_PAGE_SIZE = 20;

function sessionQuery(filters: SessionFilters, cursor?: Session): string {
  return buildParams({
    ...(filters.since ? { since: filters.since } : {}),
    ...(filters.until ? { until: filters.until } : {}),
    ...(filters.project ? { project: filters.project } : {}),
    ...(cursor ? { cursor: cursor.started_at, cursor_id: cursor.id } : {}),
    limit: SESSION_PAGE_SIZE,
  }).toString();
}

// fetchSessions returns exactly one cursor-based page. Callers append the page
// and pass its final session as the cursor to retrieve older rows.
export async function fetchSessions(
  filters: SessionFilters,
  signal: AbortSignal,
  cursor?: Session,
): Promise<Session[]> {
  return (
    (await safeFetch<Session[]>(`/api/sessions?${sessionQuery(filters, cursor)}`, signal)) ?? []
  );
}

export async function fetchSessionCount(
  filters: SessionFilters,
  signal: AbortSignal,
): Promise<number> {
  const params = buildParams({
    ...(filters.since ? { since: filters.since } : {}),
    ...(filters.until ? { until: filters.until } : {}),
    ...(filters.project ? { project: filters.project } : {}),
  });
  const result = await safeFetch<{ count: number }>(`/api/sessions/count?${params}`, signal);
  return result?.count ?? 0;
}

export async function fetchSessionProjects(signal: AbortSignal): Promise<string[]> {
  return (await safeFetch<string[]>('/api/sessions/distinct-projects', signal)) ?? [];
}

export async function fetchAnomalies(signal: AbortSignal): Promise<Anomaly[]> {
  return (await safeFetch<Anomaly[]>('/api/anomalies', signal)) ?? [];
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
