<script lang="ts">
  import { modelColor } from '../lib/colors';
  import { intl, ms, usd } from '../lib/format';
  import { dashboard } from '../stores/dashboard.svelte';

  type SortKey = 'model' | 'requests' | 'tokens' | 'latency' | 'cost';
  let sortKey: SortKey = $state('tokens');
  let sortDirection: 'asc' | 'desc' = $state('desc');

  const rows = $derived.by(() => {
    const multiplier = sortDirection === 'asc' ? 1 : -1;
    return [...dashboard.modelRows].sort((a, b) => {
      const values: Record<SortKey, string | number> = {
        model: a.model,
        requests: a.requests,
        tokens: a.total_tokens,
        latency: a.avg_latency_ms,
        cost: a.total_usd,
      };
      const other: Record<SortKey, string | number> = {
        model: b.model,
        requests: b.requests,
        tokens: b.total_tokens,
        latency: b.avg_latency_ms,
        cost: b.total_usd,
      };
      if (typeof values[sortKey] === 'string') {
        return multiplier * String(values[sortKey]).localeCompare(String(other[sortKey]));
      }
      return multiplier * (Number(values[sortKey]) - Number(other[sortKey]));
    });
  });
  const maxToken = $derived(Math.max(1, ...rows.map((row) => row.total_tokens)));
  const colorModels = $derived([...new Set(dashboard.modelRows.map((row) => row.model))].sort());

  function color(model: string): string {
    return modelColor(model, colorModels.indexOf(model), colorModels.length);
  }

  function sort(key: SortKey): void {
    if (sortKey === key) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDirection = key === 'model' ? 'asc' : 'desc';
    }
  }

  function indicator(key: SortKey): string {
    return sortKey === key ? (sortDirection === 'asc' ? '▲' : '▼') : '';
  }
</script>

<table>
  <thead>
    <tr>
      <th><button class="sort-header" onclick={() => sort('model')}>Model {indicator('model')}</button></th>
      <th>Upstream</th>
      <th><button class="sort-header" onclick={() => sort('requests')}>Requests {indicator('requests')}</button></th>
      <th class="col-optional">Cache&nbsp;%</th>
      <th><button class="sort-header" onclick={() => sort('tokens')}>Total Tokens {indicator('tokens')}</button></th>
      <th><button class="sort-header" onclick={() => sort('latency')}>Latency {indicator('latency')}</button></th>
      <th><button class="sort-header" onclick={() => sort('cost')}>Total $ {indicator('cost')}</button></th>
    </tr>
  </thead>
  <tbody>
    {#each rows as s (s.model + s.endpoint + s.upstream_host)}
      <tr title={s.detail}>
        <td>
          <span class="bar-cell">
            <span
              class="bar-inline"
              style="width: {dashboard.barW(s.total_tokens, maxToken)}px; background: {color(s.model)};"
            ></span>
            {s.model}
            <span class="tag">{s.endpoint}</span>
            {#if s.fallback}<span class="tag fb">fallback</span>{/if}
            {#if s.not_billed}<span class="tag nb">not billed</span>{/if}
          </span>
        </td>
        <td class="num"><span class="tag">{s.upstream_host || '–'}</span></td>
        <td class="num">{intl(s.requests)}</td>
        <td class="num col-optional">{s.cache_hit_pct}%</td>
        <td class="num">{intl(s.total_tokens)}</td>
        <td class="num">{ms(s.avg_latency_ms)}</td>
        <td class="num">{usd(s.total_usd)}</td>
      </tr>
    {/each}
  </tbody>
</table>
