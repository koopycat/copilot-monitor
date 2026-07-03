// Formatters using Intl

const USD = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

export function usd(n: number | null | undefined): string {
  if (n == null) return '-';
  if (n < 0.01) return '<$0.01';
  return USD.format(n);
}

export function ms(n: number | null | undefined): string {
  if (n == null) return '-';
  if (n < 1) return '<1ms';
  if (n < 1000) return `${Math.round(n)}ms`;
  return `${(n / 1000).toFixed(1)}s`;
}

export function dur(seconds: number | null | undefined): string {
  let s = Math.max(0, Math.round(seconds ?? 0));
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, '0')}s`;
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return `${h}h ${String(m).padStart(2, '0')}m`;
}

export function intl(n: number | null | undefined): string {
  if (n == null) return '0';
  return n.toLocaleString();
}

export function midnightToday(): Date {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth(), now.getDate());
}

export function midnightYesterday(): Date {
  const t = midnightToday();
  return new Date(t.getFullYear(), t.getMonth(), t.getDate() - 1);
}

export function toISO(d: Date): string {
  return d.toISOString().slice(0, 19) + 'Z';
}
