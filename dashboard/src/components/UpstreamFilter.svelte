<script lang="ts">
  import { dashboard } from '../stores/dashboard.svelte';
</script>

{#if dashboard.upstreams.length > 0}
  <select
    class="upstream-filter"
    value={dashboard.upstream}
    onchange={(e) => dashboard.switchUpstream((e.target as HTMLSelectElement).value)}
  >
    <option value="">All upstreams</option>
    {#each dashboard.upstreams as u}
      <option value={u}>{u}</option>
    {/each}
  </select>
  {#if dashboard.upstream}
    <span class="filter-active" title={`Filtering by ${dashboard.upstream}`}>Filtered</span>
  {/if}
{/if}
