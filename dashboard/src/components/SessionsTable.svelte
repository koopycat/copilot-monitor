<script lang="ts">
  import { dur, intl, usd } from '../lib/format';
  import { dashboard } from '../stores/dashboard.svelte';

  const sessions = $derived(dashboard.sessions);
</script>

<table>
  <thead>
    <tr>
      <th>Start</th>
      <th>Duration</th>
      <th>Project</th>
      <th>Requests</th>
      <th>Tokens</th>
      <th>Total&nbsp;$</th>
    </tr>
  </thead>
  <tbody>
    {#if sessions.length === 0}
      <tr><td colspan="6" class="empty">No sessions captured yet</td></tr>
    {/if}
    {#each sessions as s (s.id)}
      <tr>
        <td>{new Date(s.started_at).toLocaleString()}</td>
        <td class="num">{dur((new Date(s.ended_at).getTime() - new Date(s.started_at).getTime()) / 1000)}</td>
        <td>{s.project || '-'}</td>
        <td class="num">{intl(s.request_count)}</td>
        <td class="num">{intl(s.token_count)}</td>
        <td class="num">{usd(s.cost)}</td>
      </tr>
    {/each}
  </tbody>
</table>
