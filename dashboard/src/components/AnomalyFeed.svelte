<script lang="ts">
  import { dashboard } from '../stores/dashboard.svelte';

  const anomalies = $derived(dashboard.anomalies.slice(0, 10));
</script>

<details class="anomaly-feed">
  <summary>
    Anomalies
    {#if dashboard.anomalies.length > 0}<span class="anomaly-count">{dashboard.anomalies.length}</span>{/if}
  </summary>
  {#if anomalies.length === 0}
    <p class="empty">No anomalies detected</p>
  {:else}
    <ul>
      {#each anomalies as anomaly (anomaly.id)}
        <li>
          <span class="severity {anomaly.severity}">{anomaly.severity}</span>
          <span>{anomaly.category.replaceAll('_', ' ')}</span>
          {#if anomaly.detail}<span class="anomaly-detail">{anomaly.detail}</span>{/if}
          <time>{new Date(anomaly.ts).toLocaleString()}</time>
        </li>
      {/each}
    </ul>
  {/if}
</details>
