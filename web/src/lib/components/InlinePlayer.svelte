<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { getRecordingPlaybackUrl } from '$lib/api';
  import type { Recording } from '$lib/api';
  import { formatDuration, formatFileSize } from '$lib/format';
  import { SkipForward, SkipBack, Loader2, X } from 'lucide-svelte';
  import MjpegPlayer from '$lib/components/MjpegPlayer.svelte';

  interface Props {
    recording: Recording;
    allRecordings: Recording[];
    onClose: () => void;
    onNavigate: (recording: Recording) => void;
  }

  let { recording, allRecordings, onClose, onNavigate }: Props = $props();

  let videoSrc = $state('');
  let videoLoading = $state(false);
  let error = $state('');
  let mjpegPlayer: MjpegPlayer | undefined = $state();
  let videoEl: HTMLVideoElement | undefined = $state();
  let lastLoadedId = $state('');

  let currentIndex = $derived(allRecordings.findIndex(r => r.id === recording.id));
  let hasPrevious = $derived(currentIndex > 0);
  let hasNext = $derived(currentIndex < allRecordings.length - 1);
  let isVideoFormat = $derived(
    recording.format === 'h264' || recording.format === 'h265' || recording.format === 'timelapse'
  );

  async function loadVideo() {
    const recordingId = recording.id;
    const recordingFormat = recording.format;

    if (lastLoadedId === recordingId) return;
    lastLoadedId = recordingId;

    videoLoading = true;
    error = '';
    videoSrc = '';

    try {
      if (recordingFormat === 'mjpeg') {
        await tick();
        if (mjpegPlayer) await mjpegPlayer.initPlayer();
      } else if (isVideoFormat) {
        videoSrc = getRecordingPlaybackUrl(recordingId);
      } else {
        error = 'Unsupported format';
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

  function handleVideoError() {
    error = 'Failed to play video';
    videoSrc = '';
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
    const _ = recording.id;
    loadVideo();
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
        {#if recording.merged}
          · merged
        {/if}
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
    {:else if videoSrc}
      <video
        bind:this={videoEl}
        controls
        preload="metadata"
        class="w-full max-h-[60vh]"
        src={videoSrc}
        onended={handleVideoEnded}
        onerror={handleVideoError}
      >
        <track kind="captions" />
        Your browser does not support video.
      </video>
    {/if}
  </div>
</div>
