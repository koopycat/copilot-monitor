<script lang="ts">
  import { modelColor } from '../lib/colors';
  import { intl, ms, usd } from '../lib/format';
  import { dashboard } from '../stores/dashboard.svelte';

  const rows = $derived(dashboard.modelRows);
  const maxToken = $derived(dashboard.maxToken);
</script>

<table>
  <thead>
    <tr>
      <th>Model</th>
      <th>Requests</th>
      <th>Cache&nbsp;%</th>
      <th>Total Tokens</th>
      <th>Latency</th>
      <th>Total&nbsp;$</th>
    </tr>
  </thead>
  <tbody>
    {#each rows as s, i (s.model + s.endpoint)}
      <tr title={s.detail}>
        <td>
          <span class="bar-cell">
            <span
              class="bar-inline"
              style="width: {dashboard.barW(s.total_tokens, maxToken)}px; background: {modelColor(s.model, i)};"
            ></span>
            {s.model}
            <span class="tag">{s.endpoint}</span>
            {#if s.fallback}<span class="tag fb">fallback</span>{/if}
            {#if s.not_billed}<span class="tag nb">not billed</span>{/if}
          </span>
        </td>
        <td class="num">{intl(s.requests)}</td>
        <td class="num">{s.cache_hit_pct}%</td>
        <td class="num">{intl(s.total_tokens)}</td>
        <td class="num">{ms(s.avg_latency_ms)}</td>
        <td class="num">{usd(s.total_usd)}</td>
      </tr>
    {/each}
  </tbody>
</table>
