<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getSnapshotUrl, getAuthHeader } from '$lib/api';
  import { RefreshCw, Image, AlertCircle } from 'lucide-svelte';
  import { t } from '$lib/i18n';

  let {
    cameraId,
    cameraName = '',
    size = 'md',
    autoRefresh = true,
    refreshInterval = 30000,
    showControls = true,
  }: {
    cameraId: string;
    cameraName?: string;
    size?: 'sm' | 'md' | 'lg';
    autoRefresh?: boolean;
    refreshInterval?: number;
    showControls?: boolean;
  } = $props();

  let snapshotUrl = $state('');
  let loading = $state(true);
  let error = $state(false);
  let timestamp = $state(Date.now());
  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  function loadImage() {
    loading = true;
    error = false;
    timestamp = Date.now();
    snapshotUrl = `${getSnapshotUrl(cameraId)}?t=${timestamp}`;
  }

  function handleLoad() {
    loading = false;
    error = false;
  }

  function handleError() {
    loading = false;
    error = true;
  }

  function refresh() {
    loadImage();
  }

  // Size classes
  const sizeClasses = {
    sm: 'w-32 h-24',
    md: 'w-full aspect-video',
    lg: 'w-full aspect-video',
  };

  onMount(() => {
    loadImage();

    if (autoRefresh && refreshInterval > 0) {
      refreshTimer = setInterval(() => {
        loadImage();
      }, refreshInterval);
    }
  });

  onDestroy(() => {
    if (refreshTimer) {
      clearInterval(refreshTimer);
    }
  });
</script>

<div class="snapshot-container {sizeClasses[size]}">
  {#if loading}
    <div class="snapshot-loading">
      <RefreshCw size={20} class="animate-spin" />
    </div>
  {/if}

  {#if error}
    <div class="snapshot-error">
      <AlertCircle size={20} />
      <span class="text-xs mt-1">{t('snapshot.unavailable') || 'No snapshot'}</span>
    </div>
  {:else}
    <img
      src={snapshotUrl}
      alt={cameraName || cameraId}
      class="snapshot-image"
      class:loading
      onload={handleLoad}
      onerror={handleError}
      crossorigin="anonymous"
    />
  {/if}

  {#if showControls && !error}
    <div class="snapshot-controls">
      <button
        class="snapshot-btn"
        onclick={refresh}
        title={t('snapshot.refresh') || 'Refresh snapshot'}
      >
        <RefreshCw size={14} />
      </button>
    </div>
  {/if}
</div>

<style>
  .snapshot-container {
    position: relative;
    overflow: hidden;
    background: var(--bg-secondary);
    border-radius: var(--radius-sm);
  }

  .snapshot-image {
    width: 100%;
    height: 100%;
    object-fit: cover;
    transition: opacity 0.2s ease;
  }

  .snapshot-image.loading {
    opacity: 0;
  }

  .snapshot-loading {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-tertiary);
  }

  .snapshot-error {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: var(--text-tertiary);
    background: var(--bg-secondary);
  }

  .snapshot-controls {
    position: absolute;
    bottom: 0.5rem;
    right: 0.5rem;
    opacity: 0;
    transition: opacity 0.2s ease;
  }

  .snapshot-container:hover .snapshot-controls {
    opacity: 1;
  }

  .snapshot-btn {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.6);
    color: white;
    border: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background 0.2s ease;
  }

  .snapshot-btn:hover {
    background: rgba(0, 0, 0, 0.8);
  }
</style>
