<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { listStreams } from '$lib/api';
  import type { StreamInfo } from '$lib/api';
  import { t } from '$lib/i18n';
  import StreamCard from '$lib/components/StreamCard.svelte';
  import Pagination from '../components/Pagination.svelte';
  import {
    AlertCircle,
    Camera,
    RefreshCw,
    SatelliteDish,
    Search,
    X,
  } from 'lucide-svelte';

  const PAGE_SIZE = 12;

  let loading = $state(true);
  let error = $state('');
  let searchQuery = $state('');
  let searchApplied = $state('');
  let searchTimer: number | undefined;

  let managedStreams = $state<StreamInfo[]>([]);
  let externalStreams = $state<StreamInfo[]>([]);
  let managedTotal = $state(0);
  let externalTotal = $state(0);
  let managedPage = $state(1);
  let externalPage = $state(1);
  let refreshTimer: number | undefined;

  let managedTotalPages = $derived(Math.max(1, Math.ceil(managedTotal / PAGE_SIZE)));
  let externalTotalPages = $derived(Math.max(1, Math.ceil(externalTotal / PAGE_SIZE)));

  async function loadStreams(options?: { silent?: boolean }) {
    if (!options?.silent) {
      loading = managedStreams.length === 0 && externalStreams.length === 0;
    }
    error = '';
    try {
      const q = searchApplied || undefined;
      const [managedRes, externalRes] = await Promise.all([
        listStreams({
          q,
          managed: true,
          limit: PAGE_SIZE,
          offset: (managedPage - 1) * PAGE_SIZE,
        }),
        listStreams({
          q,
          managed: false,
          limit: PAGE_SIZE,
          offset: (externalPage - 1) * PAGE_SIZE,
        }),
      ]);
      managedTotal = managedRes.total;
      externalTotal = externalRes.total;

      const maxManagedPage = Math.max(1, Math.ceil(managedTotal / PAGE_SIZE));
      const maxExternalPage = Math.max(1, Math.ceil(externalTotal / PAGE_SIZE));
      let reload = false;
      if (managedPage > maxManagedPage) {
        managedPage = maxManagedPage;
        reload = true;
      }
      if (externalPage > maxExternalPage) {
        externalPage = maxExternalPage;
        reload = true;
      }
      if (reload) {
        return loadStreams(options);
      }

      managedStreams = managedRes.streams;
      externalStreams = externalRes.streams;
    } catch (e) {
      error = e instanceof Error ? e.message : t('streams.loadFailed');
    } finally {
      loading = false;
    }
  }

  function scheduleSearch(value: string) {
    searchQuery = value;
    if (searchTimer) {
      window.clearTimeout(searchTimer);
    }
    searchTimer = window.setTimeout(() => {
      searchApplied = searchQuery.trim();
      managedPage = 1;
      externalPage = 1;
      void loadStreams();
    }, 300);
  }

  function clearSearch() {
    searchQuery = '';
    searchApplied = '';
    managedPage = 1;
    externalPage = 1;
    if (searchTimer) {
      window.clearTimeout(searchTimer);
    }
    void loadStreams();
  }

  function handleManagedPageChange(page: number) {
    managedPage = page;
    void loadStreams({ silent: true });
  }

  function handleExternalPageChange(page: number) {
    externalPage = page;
    void loadStreams({ silent: true });
  }

  function emptyMessage(managed: boolean): string {
    if (searchApplied) return t('streams.noSearchResults');
    return managed ? t('streams.managedEmpty') : t('streams.externalEmpty');
  }

  onMount(() => {
    void loadStreams();
    refreshTimer = window.setInterval(() => {
      void loadStreams({ silent: true });
    }, 5000);
  });

  onDestroy(() => {
    if (refreshTimer) {
      window.clearInterval(refreshTimer);
    }
    if (searchTimer) {
      window.clearTimeout(searchTimer);
    }
  });
</script>

{#snippet streamGrid(items: StreamInfo[], managed: boolean)}
  {#if !loading && items.length === 0}
    <div class="card border th-border p-12 text-center col-span-full">
      <div class="flex justify-center mb-4 th-text-muted">
        {#if managed}
          <Camera size={48} />
        {:else}
          <SatelliteDish size={48} />
        {/if}
      </div>
      <p class="text-sm th-text-muted">{emptyMessage(managed)}</p>
    </div>
  {:else}
    {#each items as stream (stream.stream_id)}
      <StreamCard {stream} {managed} />
    {/each}
  {/if}
{/snippet}

<div class="min-h-screen th-bg-primary ">
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-6 gap-3">
      <div>
        <h2 class="text-2xl font-bold th-text-primary">{t('streams.title')}</h2>
        <p class="text-sm th-text-muted mt-1">{t('streams.subtitle')}</p>
      </div>
      <button class="btn btn-secondary btn-sm" onclick={() => void loadStreams()} disabled={loading}>
        <span class={loading ? 'spin' : ''}>
          <RefreshCw size={16} />
        </span>
        <span>{t('common.refresh')}</span>
      </button>
    </div>

    <div class="card border th-border p-4 mb-6">
      <label for="stream-search" class="input-label">{t('streams.searchLabel')}</label>
      <div class="relative mt-1">
        <Search size={16} class="absolute left-3 top-1/2 -translate-y-1/2 th-text-tertiary pointer-events-none" />
        <input
          id="stream-search"
          type="search"
          class="input pl-9 pr-9"
          placeholder={t('streams.searchPlaceholder')}
          value={searchQuery}
          oninput={(e) => scheduleSearch(e.currentTarget.value)}
        />
        {#if searchQuery}
          <button
            type="button"
            class="absolute right-2 top-1/2 -translate-y-1/2 btn btn-ghost btn-xs px-1"
            onclick={clearSearch}
            aria-label={t('streams.clearSearch')}
          >
            <X size={14} />
          </button>
        {/if}
      </div>
      {#if searchApplied}
        <p class="text-xs th-text-tertiary mt-2">
          {t('streams.searchSummary', { query: searchApplied, count: String(managedTotal + externalTotal) })}
        </p>
      {/if}
    </div>

    {#if error}
      <div class="card border th-border-danger p-8 text-center mb-6">
        <div class="flex justify-center mb-4 th-color-danger">
          <AlertCircle size={48} />
        </div>
        <h3 class="text-lg font-medium th-text-primary mb-2">{t('common.error')}</h3>
        <p class="th-text-secondary mb-4">{error}</p>
        <button onclick={() => void loadStreams()} class="btn btn-primary btn-sm">{t('common.retry')}</button>
      </div>
    {/if}

    {#if loading && managedStreams.length === 0 && externalStreams.length === 0}
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {#each Array(6) as _}
          <div class="card border th-border p-4 space-y-3 animate-pulse">
            <div class="flex items-center justify-between">
              <div class="h-4 w-28 th-bg-tertiary rounded"></div>
              <div class="h-5 w-16 th-bg-tertiary rounded-full"></div>
            </div>
            <div class="flex gap-2">
              <div class="h-5 w-16 th-bg-tertiary rounded"></div>
              <div class="h-5 w-12 th-bg-tertiary rounded"></div>
            </div>
            <div class="h-3 w-full th-bg-tertiary rounded"></div>
            <div class="border-t th-border pt-3 flex justify-between">
              <div class="h-4 w-10 th-bg-tertiary rounded"></div>
              <div class="h-6 w-16 th-bg-tertiary rounded"></div>
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <section class="mb-8">
        <div class="flex items-center justify-between gap-3 mb-4">
          <div>
            <h3 class="text-lg font-semibold th-text-primary">{t('streams.managedTitle')}</h3>
            <p class="text-xs th-text-tertiary mt-0.5">{t('streams.managedHint')}</p>
          </div>
          <span class="badge badge-neutral shrink-0">{managedTotal}</span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {@render streamGrid(managedStreams, true)}
        </div>

        {#if managedTotal > PAGE_SIZE}
          <div class="card border th-border mt-4 overflow-hidden">
            <div class="px-4 py-2 border-b th-border text-sm th-text-muted">
              {t('streams.pageSummary', {
                start: String((managedPage - 1) * PAGE_SIZE + 1),
                end: String(Math.min(managedPage * PAGE_SIZE, managedTotal)),
                total: String(managedTotal),
              })}
            </div>
            <Pagination
              currentPage={managedPage}
              totalPages={managedTotalPages}
              onPageChange={handleManagedPageChange}
            />
          </div>
        {/if}
      </section>

      <section>
        <div class="flex items-center justify-between gap-3 mb-4">
          <div>
            <h3 class="text-lg font-semibold th-text-primary">{t('streams.externalTitle')}</h3>
            <p class="text-xs th-text-tertiary mt-0.5">{t('streams.externalHint')}</p>
          </div>
          <span class="badge badge-neutral shrink-0">{externalTotal}</span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {@render streamGrid(externalStreams, false)}
        </div>

        {#if externalTotal > PAGE_SIZE}
          <div class="card border th-border mt-4 overflow-hidden">
            <div class="px-4 py-2 border-b th-border text-sm th-text-muted">
              {t('streams.pageSummary', {
                start: String((externalPage - 1) * PAGE_SIZE + 1),
                end: String(Math.min(externalPage * PAGE_SIZE, externalTotal)),
                total: String(externalTotal),
              })}
            </div>
            <Pagination
              currentPage={externalPage}
              totalPages={externalTotalPages}
              onPageChange={handleExternalPageChange}
            />
          </div>
        {/if}
      </section>
    {/if}
  </main>
</div>

<style>
  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
