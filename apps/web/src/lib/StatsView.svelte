<script lang="ts">
  import { statsRanges, statsRangeKeys, statsDuration, statsDateLabel, statsPeriodLabel, statsPathTone, statsCalendarGroups, type StatsState, type StatsSelection, type PathAppearance, type StatsCalendarUnit } from '@hourpaths/client-core';
  import type { Translator, MessageKey } from '@hourpaths/i18n';

  export let state: StatsState;
  export let i18n: Translator;
  export let onSelect: (selection: StatsSelection) => void;
  export let onRefresh: () => void;
  export let appearance: ((id: string) => PathAppearance) | undefined = undefined;
  let grouping: StatsCalendarUnit = 'day';
  let calendar: ReturnType<typeof statsCalendarGroups> = [];
  let groupedRange = state.selection.range;
  $: if (groupedRange !== state.selection.range) { groupedRange = state.selection.range; grouping = groupedRange === 'year' ? 'month' : groupedRange === 'all_time' ? 'year' : 'day'; }
  const groupings = ['day', 'week', 'month', 'year'] as const;
  const groupingKeys: Record<StatsCalendarUnit, MessageKey> = { day: 'stats.calendar.day', week: 'stats.calendar.week', month: 'stats.calendar.month', year: 'stats.calendar.year' };
  $: data = state.data;
  $: maxBucket = Math.max(1, ...(data?.buckets ?? []).map((bucket) => bucket.seconds));
  $: calendar = statsCalendarGroups(data?.calendar ?? [], grouping, data?.weekStartsOn);
  $: maxCalendar = Math.max(1, ...calendar.map((entry) => entry.seconds));
  $: slices = (data?.distribution ?? []).map((path, index, paths) => ({
    ...path,
    fraction: data?.totalSeconds ? path.seconds / data.totalSeconds : 0,
    offset: data?.totalSeconds ? paths.slice(0, index).reduce((sum, item) => sum + item.seconds, 0) / data.totalSeconds : 0,
  }));
  function togglePath(id: string) {
    const current = state.selection.pathIds;
    onSelect({ ...state.selection, pathIds: current.includes(id) ? current.filter((value) => value !== id) : [...current, id] });
  }
  const dashArray = (fraction: number) => `${fraction} ${1 - fraction}`;
  const barHeight = (seconds: number, max: number) => `${seconds / max * 100}%`;
  const valueLabel = (label: string, seconds: number) => i18n.t('stats.chartValue', { label, duration: i18n.t('duration.compactSeconds', { seconds: i18n.number(seconds) }) });
</script>

<section class="stats" aria-label={i18n.t('stats.title')}>
  <header class="heading"><div><h2>{i18n.t('stats.title')}</h2><p>{i18n.t('stats.personalOnly')}</p></div><button class="icon-button" onclick={onRefresh} disabled={state.refreshing || state.status === 'loading'} aria-label={i18n.t('stats.refresh')}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 7v5h-5M4 17v-5h5M6.2 7a7 7 0 0 1 11.6-1L20 9M4 15l2.2 3A7 7 0 0 0 18 17" /></svg></button></header>
  <div class="segmented" role="group" aria-label={i18n.t('stats.rangeLabel')}>
    {#each statsRanges as range}<button aria-pressed={state.selection.range === range} onclick={() => onSelect({ ...state.selection, range, anchor: undefined })}>{i18n.t(statsRangeKeys[range])}</button>{/each}
  </div>
  <div class="period">
    <button class="icon-button" disabled={!data?.previousAnchor || state.status === 'loading' || state.selection.range === 'all_time'} aria-label={i18n.t('stats.previous')} onclick={() => onSelect({ ...state.selection, anchor: data?.previousAnchor })}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m14 6-6 6 6 6" /></svg></button>
    <div><strong>{data ? statsPeriodLabel(data, i18n) : i18n.t(statsRangeKeys[state.selection.range])}</strong>{#if state.selection.anchor && state.selection.range !== 'all_time'}<button class="text-button" onclick={() => onSelect({ ...state.selection, anchor: undefined })}>{i18n.t('stats.current')}</button>{/if}</div>
    <button class="icon-button" disabled={!data?.nextAnchor || state.status === 'loading' || state.selection.range === 'all_time'} aria-label={i18n.t('stats.next')} onclick={() => onSelect({ ...state.selection, anchor: data?.nextAnchor })}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m10 6 6 6-6 6" /></svg></button>
  </div>
  <details class="path-filter"><summary>{i18n.t('stats.paths')}<span>{state.selection.pathIds.length ? i18n.t('stats.selectedPaths', { count: state.selection.pathIds.length }) : i18n.t('stats.allPaths')}</span></summary>
    <div class="filter-options" role="group" aria-label={i18n.t('stats.paths')}>
      <button class="filter-chip" aria-pressed={!state.selection.pathIds.length} onclick={() => onSelect({ ...state.selection, pathIds: [] })}>{i18n.t('stats.allPaths')}</button>
      {#each state.paths ?? data?.availablePaths ?? [] as path}<button class="filter-chip" aria-pressed={state.selection.pathIds.includes(path.id)} onclick={() => togglePath(path.id)}><span class="dot" style:background={statsPathTone(path.id, appearance).accent}></span>{path.name}{#if path.archived}<small>{i18n.t('stats.archived')}</small>{/if}</button>{/each}
    </div>
  </details>
  {#if state.status === 'error'}<div class="notice" role="alert"><p>{i18n.t('stats.unavailable')}</p><button onclick={onRefresh}>{i18n.t('common.retry')}</button></div>{/if}
  {#if state.status === 'loading' || state.refreshing}<p class="loading" role="status">{i18n.t('common.loading')}</p>{/if}
  {#if data}
    <div class="total"><span>{i18n.t('stats.total')}</span><strong>{statsDuration(data.totalSeconds, i18n)}</strong></div>
    {#if data.totalSeconds === 0}<div class="empty"><h3>{i18n.t('stats.emptyTitle')}</h3><p>{i18n.t('stats.emptyDescription')}</p></div>
    {:else}
      <section class="panel"><h3>{i18n.t('stats.distribution')}</h3><div class="distribution">
        <svg class="donut" viewBox="0 0 120 120" aria-hidden="true"><circle cx="60" cy="60" r="44" fill="none" stroke="var(--surface)" stroke-width="19" />{#each slices as slice}<circle cx="60" cy="60" r="44" fill="none" stroke={statsPathTone(slice.pathId, appearance).accent} stroke-width="19" pathLength="1" stroke-dasharray={dashArray(slice.fraction)} stroke-dashoffset={-slice.offset} transform="rotate(-90 60 60)" />{/each}</svg>
        <ul class="legend">{#each data.distribution as path}<li><span class="dot" style:background={statsPathTone(path.pathId, appearance).accent}></span><span>{path.name}</span><strong>{statsDuration(path.seconds, i18n)}</strong></li>{/each}</ul>
      </div></section>
      <section class="panel"><h3>{i18n.t('stats.activity')}</h3><div class="chart-scroll"><ol class="bars">{#each data.buckets as bucket}<li aria-label={valueLabel(statsDateLabel(bucket.key, data.bucketUnit, i18n), bucket.seconds)}><div class="bar-track" aria-hidden="true"><span style:height={barHeight(bucket.seconds, maxBucket)}></span></div><strong>{statsDuration(bucket.seconds, i18n)}</strong><span>{statsDateLabel(bucket.key, data.bucketUnit, i18n)}</span></li>{/each}</ol></div></section>
      <section class="panel"><div class="calendar-heading"><h3>{i18n.t('stats.calendar')}</h3><label><span class="sr-only">{i18n.t('stats.calendarGrouping')}</span><select bind:value={grouping}>{#each groupings as value}<option value={value}>{i18n.t(groupingKeys[value])}</option>{/each}</select></label></div>
        <ol class="calendar">{#each calendar as entry}<li class:has-time={entry.seconds > 0} style:--intensity={String(.08 + entry.seconds / maxCalendar * .2)} aria-label={valueLabel(statsDateLabel(entry.key, grouping === 'week' ? 'day' : grouping, i18n), entry.seconds)}><time datetime={entry.key}>{statsDateLabel(entry.key, grouping === 'week' ? 'day' : grouping, i18n)}</time><strong>{statsDuration(entry.seconds, i18n)}</strong></li>{/each}</ol>
      </section>
    {/if}
  {/if}
</section>

<style>
  .stats { --surface:#f2f2f7; --panel:#fff; --ink:#1c1c1e; --muted:#636366; --line:#d8d8dc; color:var(--ink); display:flex; flex-direction:column; gap:1rem; }
  h2,h3,p { margin:0; } h2 { font-size:1.8rem; letter-spacing:-.05rem; } h3 { font-size:1rem; font-weight:650; } button,select { font:inherit; } button { border:0; border-radius:.7rem; min-height:2.75rem; padding:.55rem .85rem; color:var(--ink); background:var(--surface); cursor:pointer; } button:disabled { opacity:.35; cursor:default; } button:focus-visible,summary:focus-visible,select:focus-visible { outline:3px solid #235f9e; outline-offset:3px; }
  .heading { display:flex; align-items:center; justify-content:space-between; } .heading p,.total>span { color:var(--muted); font-size:.875rem; margin-top:.2rem; }
  .icon-button { width:2.75rem; display:grid; place-items:center; padding:.6rem; background:transparent; flex:none; } .icon-button svg { width:1.4rem; fill:none; stroke:currentColor; stroke-width:1.8; stroke-linecap:round; stroke-linejoin:round; }
  .segmented { display:flex; padding:.2rem; border-radius:.85rem; background:var(--surface); } .segmented button { flex:1; min-width:0; padding:.45rem .3rem; background:transparent; font-size:.85rem; min-height:2.3rem; } .segmented button[aria-pressed=true] { background:var(--panel); box-shadow:0 1px 3px #0002; font-weight:600; }
  .period { display:flex; justify-content:space-between; align-items:center; gap:.5rem; } .period>div { display:flex; flex-direction:column; text-align:center; font-size:.95rem; } .text-button { background:transparent; color:#235f9e; min-height:2rem; font-size:.8rem; padding:.3rem; }
  .path-filter { border:1px solid var(--line); border-radius:1rem; padding:.75rem 1rem; } summary { cursor:pointer; font-weight:600; } summary>span { float:right; font-weight:400; color:var(--muted); font-size:.875rem; } .filter-options { display:flex; flex-wrap:wrap; gap:.5rem; padding-top:1rem; } .filter-chip { display:flex; gap:.4rem; align-items:center; font-size:.85rem; border:1px solid transparent; } .filter-chip[aria-pressed=true] { border-color:var(--ink); } .filter-chip small { color:var(--muted); }
  .total { padding:.5rem 0; display:flex; flex-direction:column; gap:.25rem; } .total strong { font-size:clamp(2rem,7vw,3rem); letter-spacing:-.08rem; font-variant-numeric:tabular-nums; }
  .panel { border:1px solid var(--line); border-radius:1.25rem; padding:1.15rem; min-width:0; } .distribution { display:flex; align-items:center; gap:1rem; padding-top:.75rem; } .donut { width:9rem; flex:none; } .legend { flex:1; min-width:0; padding:0; margin:0; list-style:none; } .legend li { display:flex; align-items:center; gap:.5rem; padding:.45rem 0; font-size:.85rem; } .legend li>span:nth-child(2) { overflow-wrap:anywhere; flex:1; } .legend strong { font-size:.8rem; white-space:nowrap; font-variant-numeric:tabular-nums; } .dot { height:.6rem; width:.6rem; border-radius:50%; flex:none; display:inline-block; }
  .chart-scroll { overflow-x:auto; padding-top:1rem; } .bars { display:flex; gap:.5rem; list-style:none; padding:0; margin:0; min-width:100%; } .bars li { flex:1 0 2.6rem; text-align:center; display:flex; flex-direction:column; gap:.3rem; font-size:.65rem; } .bars strong { font-size:.65rem; font-weight:500; } .bars li>span { color:var(--muted); } .bar-track { height:8rem; display:flex; align-items:flex-end; justify-content:center; border-bottom:1px solid var(--line); } .bar-track>span { display:block; width:70%; min-width:4px; border-radius:.35rem .35rem 0 0; background:#755400; }
  .calendar-heading { display:flex; justify-content:space-between; align-items:center; gap:1rem; margin-bottom:1rem; } select { border:0; border-radius:.6rem; padding:.5rem; color:var(--ink); background:var(--surface); } .calendar { display:grid; grid-template-columns:repeat(auto-fill,minmax(5rem,1fr)); gap:.4rem; padding:0; margin:0; list-style:none; max-height:24rem; overflow-y:auto; } .calendar li { padding:.65rem .45rem; background:var(--surface); border-radius:.6rem; display:flex; flex-direction:column; gap:.4rem; text-align:center; font-size:.7rem; } .calendar .has-time { background:rgb(117 84 0 / var(--intensity)); } .calendar time { color:var(--muted); } .calendar strong { font-variant-numeric:tabular-nums; }
  .empty,.notice { border-radius:1rem; background:var(--surface); padding:1.5rem; } .empty p,.notice p { line-height:1.5; color:var(--muted); margin-top:.5rem; } .notice button { margin-top:.75rem; background:var(--ink); color:var(--panel); } .loading { font-size:.875rem; color:var(--muted); } .sr-only { position:absolute; width:1px; height:1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; }
  @media(max-width:430px) { .distribution { gap:.65rem; } .donut { width:6.5rem; } .panel { padding:.9rem; } }
  @media(prefers-color-scheme:dark) { .stats { --surface:#2c2c2e; --panel:#1c1c1e; --ink:#f5f5f7; --muted:#aaa; --line:#3a3a3c; } .text-button { color:#b2d1f0; } .bar-track>span { background:#e4cf8e; } .calendar .has-time { background:rgb(228 207 142 / var(--intensity)); } }
</style>
