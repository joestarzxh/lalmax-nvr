<script lang="ts">
  import { t } from '$lib/i18n';
  import { getRecordingDays } from '$lib/api';
  import { ChevronLeft, ChevronRight } from 'lucide-svelte';

  let { cameraId, selectedDate, onselect }: {
    cameraId: string;
    selectedDate: string;                 // "YYYY-MM-DD"
    onselect: (date: string) => void;
  } = $props();

  const pad = (n: number) => String(n).padStart(2, '0');
  const fmt = (y: number, m: number, d: number) => `${y}-${pad(m + 1)}-${pad(d)}`;

  const weekdays = $derived([0, 1, 2, 3, 4, 5, 6].map((d) => t(`cameras.weekday.${d}`)));

  let viewYear = $state(new Date().getFullYear());
  let viewMonth = $state(new Date().getMonth()); // 0-11
  let daysWithRecordings = $state<Set<string>>(new Set());
  let loading = $state(false);

  const todayStr = fmt(new Date().getFullYear(), new Date().getMonth(), new Date().getDate());

  // Keep the visible month in sync when selectedDate changes externally
  // (e.g. parent prev/next-day buttons), without fighting month browsing.
  let lastSelected = '';
  $effect(() => {
    if (selectedDate && selectedDate !== lastSelected) {
      lastSelected = selectedDate;
      const [y, m] = selectedDate.split('-').map(Number);
      if (y && m) { viewYear = y; viewMonth = m - 1; }
    }
  });

  async function loadDays() {
    if (!cameraId) { daysWithRecordings = new Set(); return; }
    loading = true;
    try {
      const month = `${viewYear}-${pad(viewMonth + 1)}`;
      const days = await getRecordingDays(cameraId, month);
      daysWithRecordings = new Set(days);
    } catch (e) {
      console.warn('Failed to load recording days:', e);
      daysWithRecordings = new Set();
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    // refetch on camera or month change
    const _ = [cameraId, viewYear, viewMonth];
    loadDays();
  });

  function prevMonth() {
    if (viewMonth === 0) { viewMonth = 11; viewYear -= 1; } else { viewMonth -= 1; }
  }
  function nextMonth() {
    if (viewMonth === 11) { viewMonth = 0; viewYear += 1; } else { viewMonth += 1; }
  }

  // Grid cells: leading blanks for the first weekday, then each day of the month.
  const cells = $derived.by(() => {
    const firstWeekday = new Date(viewYear, viewMonth, 1).getDay(); // 0=Sun
    const daysInMonth = new Date(viewYear, viewMonth + 1, 0).getDate();
    const out: (number | null)[] = [];
    for (let i = 0; i < firstWeekday; i++) out.push(null);
    for (let d = 1; d <= daysInMonth; d++) out.push(d);
    return out;
  });

  const monthLabel = $derived(`${viewYear}-${pad(viewMonth + 1)}`);
</script>

<div class="recording-calendar">
  <div class="flex items-center justify-between mb-2">
    <button class="btn btn-ghost btn-sm" onclick={prevMonth} title={t('recordings.calendar.prevMonth')}>
      <ChevronLeft size={16} />
    </button>
    <span class="text-sm font-semibold th-text-primary">
      {monthLabel}
      {#if loading}<span class="th-text-muted text-xs ml-1">…</span>{/if}
    </span>
    <button class="btn btn-ghost btn-sm" onclick={nextMonth} title={t('recordings.calendar.nextMonth')}>
      <ChevronRight size={16} />
    </button>
  </div>

  <div class="grid grid-cols-7 gap-1 text-center">
    {#each weekdays as wd}
      <div class="text-xs th-text-muted py-1">{wd}</div>
    {/each}
    {#each cells as day}
      {#if day === null}
        <div></div>
      {:else}
        {@const dateStr = fmt(viewYear, viewMonth, day)}
        {@const hasRec = daysWithRecordings.has(dateStr)}
        {@const isSelected = dateStr === selectedDate}
        {@const isFuture = dateStr > todayStr}
        <button
          class="day-cell {isSelected ? 'day-selected' : ''} {hasRec ? 'day-has-rec' : ''}"
          disabled={isFuture}
          title={hasRec ? t('recordings.calendar.hasRecordings') : ''}
          onclick={() => onselect(dateStr)}
        >
          {day}
          {#if hasRec}<span class="day-dot"></span>{/if}
        </button>
      {/if}
    {/each}
  </div>
</div>

<style>
  .day-cell {
    position: relative;
    aspect-ratio: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
    border-radius: 6px;
    border: 1px solid transparent;
    color: var(--th-text-secondary, inherit);
    cursor: pointer;
    transition: background 0.12s ease, border-color 0.12s ease;
  }
  .day-cell:hover:not(:disabled) {
    background: var(--th-bg-tertiary, rgba(127,127,127,0.12));
  }
  .day-cell:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
  /* Days with recordings: emphasized text + accent dot */
  .day-has-rec {
    font-weight: 600;
    color: var(--color-accent, #2563eb);
  }
  .day-dot {
    position: absolute;
    bottom: 4px;
    left: 50%;
    transform: translateX(-50%);
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--color-accent, #2563eb);
  }
  .day-selected {
    border-color: var(--color-accent, #2563eb);
    background: color-mix(in srgb, var(--color-accent, #2563eb) 15%, transparent);
  }
</style>
