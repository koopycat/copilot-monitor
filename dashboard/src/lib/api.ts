// API client with AbortController for race conditions

import type {
  CostResponse,
  CurrentSession,
  ModelStats,
  PeriodKey,
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
): Promise<DashboardData> {
  const pq = periodQuery(period);

  const sinceParams = buildParams({ since: pq.since, ...(pq.until ? { until: pq.until } : {}) });
  const timelineParams = buildParams({
    since: pq.since,
    ...(pq.until ? { until: pq.until } : {}),
    granularity,
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

export function exportHrefFor(period: PeriodKey): string {
  const pq = periodQuery(period);
  const params = buildParams({ since: pq.since, ...(pq.until ? { until: pq.until } : {}) });
  return `/api/export?${params}`;
}
