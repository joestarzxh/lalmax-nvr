<script lang="ts">
  import { onMount, onDestroy, tick } from 'svelte';
  import { loadRecordingVideoBlob } from '$lib/api';
  import type { Recording } from '$lib/api';
  import { formatDate, formatDuration, formatFileSize } from '$lib/format';
  import { SkipForward, SkipBack, Loader2, X } from 'lucide-svelte';
  import MjpegPlayer from '$lib/components/MjpegPlayer.svelte';

  interface Props {
    recording: Recording;
    allRecordings: Recording[];
    onClose: () => void;
    onNavigate: (recording: Recording) => void;
  }

  let { recording, allRecordings, onClose, onNavigate }: Props = $props();

  let videoBlobUrl = $state('');
  let videoLoading = $state(false);
  let error = $state('');
  let mjpegPlayer: MjpegPlayer | undefined = $state();
  let nextBlobUrl = $state<string | null>(null);
  let nextRecordingId = $state<string | null>(null);
  let videoEl: HTMLVideoElement | undefined = $state();
  let lastLoadedId = $state<string>('');

  let currentIndex = $derived(allRecordings.findIndex(r => r.id === recording.id));
  let hasPrevious = $derived(currentIndex > 0);
  let hasNext = $derived(currentIndex < allRecordings.length - 1);

  async function loadVideo() {
    const recordingId = recording.id;
    const recordingFormat = recording.format;
    
    // Skip if already loaded this recording
    if (lastLoadedId === recordingId) return;
    lastLoadedId = recordingId;
    
    videoLoading = true;
    error = '';
    if (videoBlobUrl) URL.revokeObjectURL(videoBlobUrl);
    videoBlobUrl = '';

    try {
      if (recordingFormat === 'mjpeg') {
        await tick();
        if (mjpegPlayer) await mjpegPlayer.initPlayer();
      } else if (nextRecordingId === recordingId && nextBlobUrl) {
        videoBlobUrl = nextBlobUrl;
        nextBlobUrl = null;
        nextRecordingId = null;
      } else {
        videoBlobUrl = await loadRecordingVideoBlob(recordingId);
      }
    } catch (e) {
      error = 'Failed to load video';
    } finally {
      videoLoading = false;
    }
  }

  function handleVideoEnded() {
    if (hasNext) {
      onNavigate(allRecordings[currentIndex + 1]);
    }
  }

  function handleTimeUpdate(e: Event) {
    const video = e.target as HTMLVideoElement;
    if (video.duration && video.currentTime / video.duration > 0.8 && hasNext && !nextRecordingId) {
      prefetchNext();
    }
  }

  async function prefetchNext() {
    if (!hasNext) return;
    const next = allRecordings[currentIndex + 1];
    nextRecordingId = next.id;
    try {
      nextBlobUrl = await loadRecordingVideoBlob(next.id);
    } catch {
      nextRecordingId = null;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    const tag = (e.target as HTMLElement).tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;

    switch (e.key) {
      case ' ':
        e.preventDefault();
        if (videoEl) videoEl.paused ? videoEl.play() : videoEl.pause();
        break;
      case 'ArrowLeft':
        e.preventDefault();
        if (videoEl) videoEl.currentTime = Math.max(0, videoEl.currentTime - 5);
        break;
      case 'ArrowRight':
        e.preventDefault();
        if (videoEl) videoEl.currentTime = Math.min(videoEl.duration, videoEl.currentTime + 5);
        break;
      case 'Escape':
        onClose();
        break;
    }
  }

  function formatTime(dateStr: string): string {
    return new Date(dateStr).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
  }

  $effect(() => {
    // Track recording.id as dependency, call loadVideo when it changes
    const _ = recording.id;
    loadVideo();
  });

  $effect(() => {
    return () => {
      if (videoBlobUrl) URL.revokeObjectURL(videoBlobUrl);
      if (nextBlobUrl) URL.revokeObjectURL(nextBlobUrl);
    };
  });

  onMount(() => {
    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  });
</script>

<div id="inline-player" class="card border th-border overflow-hidden">
  <div class="flex items-center justify-between px-4 py-3 th-bg-secondary border-b th-border">
    <div class="flex items-center gap-3">
      <span class="text-sm font-medium th-text-primary">
        ▶ Now Playing: {formatTime(recording.started_at)} - {formatTime(recording.ended_at)}
      </span>
      <span class="text-xs th-text-tertiary">
        {formatDuration(recording.duration)} · {formatFileSize(recording.file_size)}
      </span>
    </div>
    <div class="flex items-center gap-2">
      <button
        onclick={() => hasPrevious && onNavigate(allRecordings[currentIndex - 1])}
        disabled={!hasPrevious}
        class="btn btn-ghost btn-sm disabled:opacity-30"
        title="Previous segment"
      >
        <SkipBack size={16} />
      </button>
      <button
        onclick={() => hasNext && onNavigate(allRecordings[currentIndex + 1])}
        disabled={!hasNext}
        class="btn btn-ghost btn-sm disabled:opacity-30"
        title="Next segment"
      >
        <SkipForward size={16} />
      </button>
      <button onclick={onClose} class="btn btn-ghost btn-sm" title="Close player">
        <X size={16} />
      </button>
    </div>
  </div>

  <div class="relative bg-black">
    {#if videoLoading}
      <div class="flex items-center justify-center h-64">
        <Loader2 size={32} class="animate-spin th-text-secondary" />
      </div>
    {:else if error}
      <div class="flex items-center justify-center h-64">
        <span class="th-color-danger">{error}</span>
      </div>
    {:else if recording.format === 'mjpeg'}
      <MjpegPlayer bind:this={mjpegPlayer} recordingId={recording.id} oninitdone={() => {}} />
    {:else if videoBlobUrl}
      <video
        bind:this={videoEl}
        controls
        preload="auto"
        class="w-full max-h-[60vh]"
        src={videoBlobUrl}
        onended={handleVideoEnded}
        ontimeupdate={handleTimeUpdate}
        autoplay
      >
        <track kind="captions" />
        Your browser does not support video.
      </video>
    {/if}
  </div>
</div>
