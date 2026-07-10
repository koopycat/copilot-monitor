<script lang="ts">
  import { dashboard } from '../stores/dashboard.svelte';

  const routes = $derived(dashboard.routes);
</script>

{#if routes.length > 0}
  <details class="routes-panel">
    <summary>Active Routes ({routes.length})</summary>
    <table>
      <thead>
        <tr><th>Path</th><th>Upstream</th><th>Capture</th></tr>
      </thead>
      <tbody>
        {#each routes as r}
          <tr>
            <td><code>{r.path}</code>{#if r.label} <span class="tag">{r.label}</span>{/if}</td>
            <td>{r.upstream_host}{#if r.upstream_path_prefix}<span class="tag">{r.upstream_path_prefix}</span>{/if}</td>
            <td><span class="tag">{r.capture}</span></td>
          </tr>
        {/each}
      </tbody>
    </table>
  </details>
{/if}
