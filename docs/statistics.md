# Statistics & Visualization Plan

## Time-based breakdowns

These show when Copilot usage happens.

| Statistic | Query | Usefulness |
|---|---|---|
| Per day | `GROUP BY date(ts)` | Daily rhythm, weekday vs weekend |
| Per hour | `GROUP BY strftime('%H', ts)` | Peak hours, "when do I code most?" |
| Per day-of-week | `GROUP BY strftime('%w', ts)` | Mon/Fri patterns |
| Rolling 7-day cost | Window over daily totals | Trend direction |

API endpoint idea: `GET /api/stats/timeline?since=14d&granularity=day|hour`

## Model breakdowns

These show which models cost what.

| Statistic | Query | Usefulness |
|---|---|---|
| Cost per model | Already have (`/api/cost`) | Allocation |
| Requests per model | Already have (`/api/stats`) | Model preference |
| Avg tokens per request | `AVG(prompt_tokens), AVG(completion_tokens) GROUP BY model` | Request size patterns |
| Cost per request | `SUM(cost) / COUNT(*) GROUP BY model` | True unit economics |
| Model switching | Within-session model changes | Do you switch mid-task? |

API endpoint idea: `GET /api/stats/by-model?since=30d` (enriched with per-request averages)

## Efficiency metrics

These show how well Copilot is performing and how much you cache.

| Statistic | Formula | Usefulness |
|---|---|---|
| Cache hit ratio | `cached_input_tokens / prompt_tokens` | Are prompts repetitive? |
| Latency P50/P95/P99 | Percentile over `latency_ms` per model | Performance degradation |
| Cost per 1K output tokens | `output_cost / output_tokens * 1000` | Model comparison |
| Empty cache-write requests | Count of requests where cache_write > 0 | How often does new context get cached? |

API endpoint idea: `GET /api/stats/efficiency?since=7d`

## Session insights

These show behavioral patterns.

| Statistic | Query | Usefulness |
|---|---|---|
| Sessions per day | Count sessions per date | Activity consistency |
| Avg session duration | `AVG(ended_at - started_at)` | Focus length |
| Avg session cost | `AVG(total_cost)` | Cost per coding session |
| Avg tokens per session | `AVG(token_count)` | Session intensity |
| Top 5 expensive sessions | Order by cost | Outlier detection |

API endpoint idea: `GET /api/sessions/summary?since=30d`

## Trend indicators

These show direction over time.

| Statistic | Formula | Usefulness |
|---|---|---|
| 7-day rolling cost | Window function over daily cost | Is cost trending up? |
| Week-over-week change | `this_week / last_week - 1` | Weekly growth |
| Projected monthly cost | `daily_avg_7d * 30` | Budget forecast |

API endpoint idea: `GET /api/stats/trends?since=30d`

## Implementation priority

Phase 1 (high value, low effort):

1. **`/api/stats/timeline?since=14d&granularity=day`** - daily token and request counts. One SQL `GROUP BY date(ts)`.
2. **Cache hit ratio** - add to existing `stats` response: `cache_hit_pct` field per row.
3. **Avg latency per model** - add to existing `stats` response: `avg_latency_ms` field per row.

Phase 2 (medium effort):

4. **`/api/stats/timeline?granularity=hour`** - hourly breakdown for a single date.
5. **`/api/sessions/summary`** - aggregated session metrics.
6. **Rolling 7-day cost** - add to `cost` response.

Phase 3 (nice to have):

7. **Projected monthly cost** - simple forecast.
8. **Model switching patterns** - within-session model changes.
9. **Latency percentiles** - P50/P95/P99 per model.

## Dashboard additions

The HTML dashboard should get:

- A line/bar chart area for daily usage timeline (first panel after cost summary)
- Cache hit percentage next to each model row
- Average latency column in the model table
- A "projected this month" line in the cost summary
