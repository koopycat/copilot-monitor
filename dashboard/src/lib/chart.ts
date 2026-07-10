// Canvas chart for stacked bars (model usage over time)

import type { Granularity, ModelId, TimelineEntry } from './types';

let _colors: { faint: string; border: string } | null = null;

function chartColors(): { faint: string; border: string } {
  if (!_colors) {
    const style = getComputedStyle(document.documentElement);
    _colors = {
      faint: (style.getPropertyValue('--faint') || '#6e7681').trim(),
      border: (style.getPropertyValue('--border') || '#21262d').trim(),
    };
  }
  return _colors;
}

export function drawChart(
  canvas: HTMLCanvasElement,
  data: TimelineEntry[],
  granularity: Granularity,
  modelColor: (model: ModelId, i?: number) => string,
  metric: 'tokens' | 'requests' = 'tokens',
): void {
  const ctx = canvas.getContext('2d');
  if (!ctx) return;
  const dpr = window.devicePixelRatio || 1;
  const parent = canvas.parentElement;
  if (!parent) return;
  const w = parent.clientWidth;
  const h = 200;
  if (w < 1) return;

  canvas.width = w * dpr;
  canvas.height = h * dpr;
  canvas.style.width = `${w}px`;
  canvas.style.height = `${h}px`;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);

  const { faint, border } = chartColors();

  if (!data.length) {
    ctx.fillStyle = faint;
    ctx.font = '0.8rem system-ui';
    ctx.textAlign = 'center';
    ctx.fillText('No data yet', w / 2, h / 2);
    return;
  }

  const value = (d: TimelineEntry): number => (metric === 'requests' ? d.requests : d.total_tokens);

  const dateKey = (d: TimelineEntry): string =>
    granularity === 'hour' ? `${d.date}|${String(d.hour ?? 0).padStart(2, '0')}` : d.date;

  const dates = [...new Set(data.map(dateKey))].sort();
  const models = [...new Set(data.map((d) => d.model))].sort();

  const buckets = new Map<string, number>();
  let maxValue = 0;
  for (const d of data) {
    const key = `${dateKey(d)}|${d.model}`;
    const sum = (buckets.get(key) ?? 0) + value(d);
    buckets.set(key, sum);
    if (sum > maxValue) maxValue = sum;
  }
  if (maxValue === 0) maxValue = 1;

  const padL = 42;
  const padR = 12;
  const top = 16;
  const bottom = 30;
  const chartW = w - padL - padR;
  const chartH = h - top - bottom;
  if (chartW <= 0) return;

  const barW = Math.max(3, Math.min(24, (chartW / dates.length) * 0.75));
  const gap = Math.max(1, chartW / dates.length - barW);
  const stacked = new Map<string, number>(dates.map((d) => [d, 0]));

  for (let i = 0; i < models.length; i++) {
    ctx.fillStyle = modelColor(models[i], i);
    for (let j = 0; j < dates.length; j++) {
      const val = buckets.get(`${dates[j]}|${models[i]}`) ?? 0;
      if (val === 0) continue;
      const barH = (val / maxValue) * chartH;
      const x = padL + j * (barW + gap);
      const y = h - bottom - (stacked.get(dates[j]) ?? 0) - barH;
      ctx.fillRect(x, y, barW, Math.max(1, barH));
      stacked.set(dates[j], (stacked.get(dates[j]) ?? 0) + barH);
    }
  }

  // X-axis labels
  ctx.fillStyle = faint;
  ctx.font = '0.6rem system-ui';
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  const step = Math.max(1, Math.floor(dates.length / 14));
  for (let j = 0; j < dates.length; j += step) {
    const x = padL + j * (barW + gap) + barW / 2;
    if (x > padL + chartW) break;
    if (granularity === 'hour') {
      const hour = dates[j].split('|')[1] ?? '0';
      ctx.fillText(`${hour}:00`, x, h - bottom + 6);
    } else {
      ctx.fillText(dates[j].slice(5), x, h - bottom + 6);
    }
  }

  // Y-axis ticks and grid lines
  ctx.textAlign = 'right';
  ctx.textBaseline = 'middle';
  const ticks = 4;
  const tickStep = Math.max(1, Math.round(maxValue / ticks));
  const formatTick = (v: number): string => (v >= 1000 ? `${Math.round(v / 1000)}k` : String(v));

  for (let v = 0; v <= maxValue; v += tickStep) {
    const y = h - bottom - (v / maxValue) * chartH;
    if (y < top) continue;
    ctx.fillStyle = faint;
    ctx.font = '0.55rem system-ui';
    ctx.fillText(formatTick(v), padL - 4, y);

    ctx.strokeStyle = border;
    ctx.beginPath();
    ctx.moveTo(padL, y);
    ctx.lineTo(w - padR, y);
    ctx.stroke();
  }

  // Legend
  const legendEl = document.getElementById('chart-legend');
  if (legendEl) {
    legendEl.innerHTML = models
      .map(
        (m, i) =>
          `<span class="legend-item"><span class="legend-swatch" style="background:${modelColor(m, i)}"></span>${m}</span>`,
      )
      .join('');
  }
}
