export function drawChart(canvas, data, granularity, modelColor) {
  const ctx = canvas.getContext('2d');
  const dpr = window.devicePixelRatio || 1;
  const parent = canvas.parentElement;
  const w = parent.clientWidth;
  const h = 200;
  if (w < 1) return;

  canvas.width = w * dpr; canvas.height = h * dpr;
  canvas.style.width = w + 'px'; canvas.style.height = h + 'px';
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, w, h);

  if (!data || !data.length) {
    const faint = getComputedStyle(document.body).getPropertyValue('--faint').trim() || '#6e7681';
    ctx.fillStyle = faint; ctx.font = '0.8rem system-ui'; ctx.textAlign = 'center';
    ctx.fillText('No data yet', w / 2, h / 2);
    return;
  }

  const dateKey = granularity === 'hour'
    ? d => d.date + '|' + String(d.hour).padStart(2, '0')
    : d => d.date;
  const dates = [...new Set(data.map(dateKey))].sort();
  const models = [...new Set(data.map(d => d.model))].sort();
  const buckets = new Map();
  let maxTokens = 0;

  for (const d of data) {
    const key = dateKey(d) + '|' + d.model;
    const sum = (buckets.get(key) || 0) + d.total_tokens;
    buckets.set(key, sum);
    if (sum > maxTokens) maxTokens = sum;
  }
  if (maxTokens === 0) maxTokens = 1;

  const padL = 42, padR = 12, top = 16, bottom = 30;
  const chartW = w - padL - padR, chartH = h - top - bottom;
  if (chartW <= 0) return;

  const barW = Math.max(3, Math.min(24, (chartW / dates.length) * 0.75));
  const gap = Math.max(1, (chartW / dates.length) - barW);
  const stacked = new Map(dates.map(d => [d, 0]));

  for (let i = 0; i < models.length; i++) {
    ctx.fillStyle = modelColor(models[i], i);
    for (let j = 0; j < dates.length; j++) {
      const val = buckets.get(dates[j] + '|' + models[i]) || 0;
      if (val === 0) continue;
      const barH = (val / maxTokens) * chartH;
      const x = padL + j * (barW + gap);
      const y = h - bottom - stacked.get(dates[j]) - barH;
      ctx.fillRect(x, y, barW, Math.max(1, barH));
      stacked.set(dates[j], stacked.get(dates[j]) + barH);
    }
  }

  const faint = getComputedStyle(document.body).getPropertyValue('--faint').trim() || '#6e7681';
  const border = getComputedStyle(document.body).getPropertyValue('--border').trim() || '#21262d';
  ctx.textAlign = 'center'; ctx.textBaseline = 'top';
  const step = Math.max(1, Math.floor(dates.length / 14));
  for (let j = 0; j < dates.length; j += step) {
    const x = padL + j * (barW + gap) + barW / 2;
    if (x > padL + chartW) break;
    ctx.fillStyle = faint; ctx.font = '0.6rem system-ui';
    if (granularity === 'hour') {
      const hour = dates[j].split('|')[1] || '0';
      ctx.fillText(hour + ':00', x, h - bottom + 6);
    } else {
      ctx.fillText(dates[j].slice(5), x, h - bottom + 6);
    }
  }

  ctx.textAlign = 'right'; ctx.textBaseline = 'middle';
  const ticks = 4, tickStep = Math.max(1, Math.round(maxTokens / ticks));
  for (let v = 0; v <= maxTokens; v += tickStep) {
    const y = h - bottom - (v / maxTokens) * chartH;
    if (y < top) continue;
    ctx.fillStyle = faint; ctx.font = '0.55rem system-ui';
    ctx.fillText(v >= 1000 ? Math.round(v / 1000) + 'k' : String(v), padL - 4, y);
    ctx.strokeStyle = border;
    ctx.beginPath(); ctx.moveTo(padL, y); ctx.lineTo(w - padR, y); ctx.stroke();
  }

  document.getElementById('chart-legend').innerHTML = models.map((m, i) =>
    `<span class="legend-item"><span class="legend-swatch" style="background:${modelColor(m, i)}">
</span>${m}</span>`
  ).join('');
}
