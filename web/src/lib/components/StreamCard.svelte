<script lang="ts">
  import { t } from '$lib/i18n';
  import type { StreamInfo } from '$lib/api';
  import { Eye, Users } from 'lucide-svelte';

  interface Props {
    stream: StreamInfo;
    managed?: boolean;
  }

  let { stream, managed = false }: Props = $props();

  let title = $derived(managed ? (stream.camera_name || stream.stream_id) : stream.stream_id);

  let sourceLabel = $derived.by(() => {
    switch (stream.source_type) {
      case 'camera':
        return t('streams.sourceCamera');
      case 'rtmp_push':
        return t('streams.sourceRTMPPush');
      case 'srt_push':
        return t('streams.sourceSRTPush');
      case 'relay_pull':
        return t('streams.sourceRelayPull');
      default:
        return t('streams.sourceStream');
    }
  });

  let codecLabel = $derived(
    stream.video_codec ? stream.video_codec.toUpperCase() : ''
  );

  let fpsLabel = $derived(
    stream.in_fps ? `${stream.in_fps.toFixed(1)} fps` : ''
  );

  let viewerCount = $derived(stream.subscribers?.length || 0);
</script>

<a
  href={`#/streams/${encodeURIComponent(stream.stream_id)}`}
  class="card stream-card border th-border p-4 transition-all hover:border-[var(--color-primary)]"
>
  <div class="flex items-start justify-between gap-2 mb-3">
    <div class="min-w-0 flex-1">
      <span class="font-medium th-text-primary truncate block">{title}</span>
    </div>
    <div class="shrink-0">
      {#if stream.active}
        <span class="badge badge-success">{t('streams.active')}</span>
      {:else}
        <span class="badge badge-neutral">{t('streams.idle')}</span>
      {/if}
    </div>
  </div>

  <div class="space-y-1.5 mb-3 flex-1">
    <div class="flex items-center gap-2 flex-wrap">
      <span class="text-xs font-medium th-text-secondary px-2 py-0.5 rounded th-bg-tertiary">
        {sourceLabel}
      </span>
      {#if codecLabel}
        <span class="text-xs th-text-tertiary px-2 py-0.5 rounded th-bg-tertiary">{codecLabel}</span>
      {/if}
      {#if fpsLabel}
        <span class="text-xs th-text-tertiary px-2 py-0.5 rounded th-bg-tertiary">{fpsLabel}</span>
      {/if}
    </div>
    <p class="text-xs th-text-tertiary truncate font-mono" title={stream.stream_id}>
      {stream.stream_id}
    </p>
  </div>

  <div class="flex items-center justify-between pt-3 border-t th-border">
    <span class="inline-flex items-center gap-1.5 text-xs th-text-secondary">
      <Users size={14} class="th-text-tertiary" />
      {viewerCount}
    </span>
    <span class="btn btn-ghost px-2 py-1 text-sm pointer-events-none">
      <Eye size={14} />
      <span class="hidden sm:inline">{t('streams.viewDetails')}</span>
    </span>
  </div>
</a>

<style>
  .stream-card {
    display: flex;
    flex-direction: column;
    text-decoration: none;
    color: inherit;
  }
</style>
