import type { Granularity, Period, PeriodKey } from './types';
import { midnightToday, midnightYesterday, toISO } from './format';

export const PERIODS: Period[] = [
  { key: 'today', label: 'Today' },
  { key: 'yesterday', label: 'Yesterday' },
  { key: '7d', label: '7d' },
  { key: '30d', label: '30d' },
  { key: '90d', label: '90d' },
  { key: '365d', label: '365d' },
];

export interface PeriodQuery {
  since: string;
  until: string;
}

export function periodQuery(period: PeriodKey): PeriodQuery {
  switch (period) {
    case 'today':
      return { since: toISO(midnightToday()), until: '' };
    case 'yesterday':
      return { since: toISO(midnightYesterday()), until: toISO(midnightToday()) };
    default:
      return { since: period, until: '' };
  }
}

export function periodGran(period: PeriodKey): Granularity {
  return period === 'today' || period === 'yesterday' ? 'hour' : 'day';
}

export function periodDays(period: PeriodKey): number {
  switch (period) {
    case 'today':
    case 'yesterday':
      return 1;
    case '7d':
      return 7;
    case '30d':
      return 30;
    case '90d':
      return 90;
    case '365d':
      return 365;
  }
}

export function buildParams(obj: Record<string, string | number>): URLSearchParams {
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(obj)) {
    if (v !== '' && v != null) params.set(k, String(v));
  }
  return params;
}
