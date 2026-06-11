<script lang="ts">
  import { getSnapshotUrl } from '$lib/api';
  import { Camera } from 'lucide-svelte';
  import { t } from '$lib/i18n';
  let { cameraId }: { cameraId: string } = $props();

  function handleSnapshot() {
    const url = getSnapshotUrl(cameraId);
    const separator = url.includes('?') ? '&' : '?';
    window.open(`${url}${separator}t=${Date.now()}`, '_blank');
  }
</script>

<button
  class="btn btn-ghost snapshot-btn"
  onclick={handleSnapshot}
  title={t('onvif.snapshot.takeSnapshot')}
>
  <Camera size={14} />
  <span class="snapshot-btn-label">{t('onvif.snapshot.title')}</span>
</button>

<style>
  .snapshot-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.375rem 0.625rem;
    font-size: 0.75rem;
  }

  .snapshot-btn-label {
    font-weight: 500;
  }
</style>
