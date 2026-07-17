<script lang="ts">
  import { dur, intl, usd } from '../lib/format';
  import { PERIODS } from '../lib/periods';
  import { dashboard } from '../stores/dashboard.svelte';
  import PeriodBar from './PeriodBar.svelte';

  const sessions = $derived(dashboard.sessions);
  const sessionPeriods = [{ key: 'all', label: 'All' }, ...PERIODS];
  const showingEnd = $derived(Math.min(sessions.length, dashboard.sessionCount));
</script>

<div class="session-filters">
  <select
    class="session-filter"
    aria-label="Filter sessions by project"
    value={dashboard.sessionProject}
    onchange={(e) => dashboard.switchSessionProject((e.target as HTMLSelectElement).value)}
  >
    <option value="">All projects</option>
    {#each dashboard.sessionProjects as project}
      <option value={project}>{project}</option>
    {/each}
  </select>
  <div class="session-period-bar">
    <PeriodBar periods={sessionPeriods} active={dashboard.sessionPeriod} onchange={(v) => dashboard.switchSessionPeriod(v as typeof dashboard.sessionPeriod)} />
  </div>
</div>

<p class="session-count">
  Showing {sessions.length ? `1-${showingEnd}` : '0'} of {dashboard.sessionCount.toLocaleString()} sessions
</p>

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
    {#if sessions.length === 0 && !dashboard.sessionLoading}
      <tr><td colspan="6" class="empty">No sessions captured yet</td></tr>
    {/if}
    {#each sessions as s (s.id)}
      <tr>
        <td>{new Date(s.started_at).toLocaleString()}</td>
        <td class="num"
          >{dur((new Date(s.ended_at).getTime() - new Date(s.started_at).getTime()) / 1000)}</td
        >
        <td class="text-cell">{s.project || '-'}</td>
        <td class="num">{intl(s.request_count)}</td>
        <td class="num">{intl(s.token_count)}</td>
        <td class="num">{usd(s.cost)}</td>
      </tr>
    {/each}
  </tbody>
</table>

{#if dashboard.sessionHasMore}
  <button class="btn-sm load-more" disabled={dashboard.sessionLoading} onclick={() => dashboard.loadSessions(false)}>
    {dashboard.sessionLoading ? 'Loading…' : 'Load more'}
  </button>
{/if}
