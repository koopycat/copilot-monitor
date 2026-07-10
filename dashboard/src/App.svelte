<script lang="ts">
  import LiveSessionCard from './components/LiveSessionCard.svelte';
  import MetricCard from './components/MetricCard.svelte';
  import ModelsTable from './components/ModelsTable.svelte';
  import PeriodBar from './components/PeriodBar.svelte';
  import SessionsTable from './components/SessionsTable.svelte';
  import ToggleGroup from './components/ToggleGroup.svelte';
  import UpstreamFilter from './components/UpstreamFilter.svelte';
  import UsageChart from './components/UsageChart.svelte';
  import { usd } from './lib/format';
  import { dashboard } from './stores/dashboard.svelte';

  const costText = $derived(dashboard.cost !== null ? usd(dashboard.cost) : '-');
</script>

<h1>
  Copilot Monitor
  <button class="refresh-btn" onclick={() => dashboard.load()} title="Refresh now">↻</button>
</h1>

<PeriodBar periods={dashboard.periods} active={dashboard.period} onchange={(v) => dashboard.switchPeriod(v as typeof dashboard.period)} />

<p class="subtitle">
  {#if dashboard.lastUpdated}Updated {dashboard.lastUpdated}{:else}Loading…{/if}
  <UpstreamFilter />
</p>

{#if dashboard.periodIsEmpty}
  <p class="empty-state">No data for this period</p>
{/if}

<div class="row">
  <MetricCard value={costText} label={`est. AI-credit cost, ${dashboard.periodLabel}`} />
  <MetricCard value={dashboard.projectedText} label="projected this month" />
  <MetricCard value={dashboard.totalRequests.toLocaleString()} label={`requests, ${dashboard.periodLabel}`} />
  <LiveSessionCard />
</div>

<h2>
  Usage
  <ToggleGroup
    options={[{ value: 'day', label: 'Day' }, { value: 'hour', label: 'Hour' }]}
    active={dashboard.gran}
    onchange={(v) => dashboard.switchGran(v as typeof dashboard.gran)}
  />
  <ToggleGroup
    options={[{ value: 'tokens', label: 'Tokens' }, { value: 'requests', label: 'Requests' }]}
    active={dashboard.metric}
    onchange={(v) => dashboard.switchMetric(v as typeof dashboard.metric)}
  />
</h2>

<UsageChart />

<h2>Models</h2>
<ModelsTable />

<h2>Recent Sessions</h2>
<SessionsTable />

<footer class="footer">
  <span>Estimated AI-credit list-price cost. Not your GitHub Copilot bill.</span>
  <span><a href={dashboard.exportHref}>Export CSV</a> · Auto-refreshes every 30s</span>
</footer>
