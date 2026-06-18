<script lang="ts">
  import { getDeviceCapabilities } from '$lib/api';
  import type { DeviceCapabilitiesInfo } from '$lib/api';
  import { Move, Image, Bell, Camera, Radio, ZoomIn, Home, Info } from 'lucide-svelte';
  import { t } from '$lib/i18n';
  let { cameraId }: { cameraId: string } = $props();

  let caps = $state<DeviceCapabilitiesInfo | null>(null);
  let loading = $state(true);
  let error = $state('');

  $effect(() => {
    cameraId;
    loadCapabilities();
  });

  async function loadCapabilities() {
    loading = true;
    error = '';
    try {
      caps = await getDeviceCapabilities(cameraId);
    } catch (e: any) {
      error = e.message || t('onvif.capabilities.loadError');
    } finally {
      loading = false;
    }
  }

  interface CapabilityBadge {
    key: string;
    label: string;
    icon: any;
    enabled: boolean;
  }

  let badges = $derived<CapabilityBadge[]>(caps ? [
    { key: 'ptz', label: t('onvif.capabilities.ptz'), icon: Move, enabled: caps.ptz },
    { key: 'imaging', label: t('onvif.capabilities.imaging'), icon: Image, enabled: caps.imaging },
    { key: 'events', label: t('onvif.capabilities.events'), icon: Bell, enabled: caps.events },
    { key: 'snapshot', label: t('onvif.capabilities.snapshot'), icon: Camera, enabled: caps.snapshot },
    { key: 'streaming', label: t('onvif.capabilities.streaming'), icon: Radio, enabled: caps.streaming },
  ] : []);

  let ptzBadges = $derived<CapabilityBadge[]>(caps?.ptz_detail ? [
    { key: 'pan_tilt', label: '云台', icon: Move, enabled: caps.ptz_detail.pan_tilt },
    { key: 'zoom', label: '变焦', icon: ZoomIn, enabled: caps.ptz_detail.zoom },
    { key: 'presets', label: '预设点', icon: Camera, enabled: caps.ptz_detail.presets },
    { key: 'home', label: '归位', icon: Home, enabled: caps.ptz_detail.home },
  ] : []);
</script>

<div class="caps-panel">
  {#if loading}
    <div class="caps-loading">
      <span class="spinner"></span>
    </div>
  {:else if error}
    <div class="caps-error">{error}</div>
  {:else if caps}
    <!-- Device info -->
    {#if caps.device_info}
      <div class="caps-device-info">
        <div class="caps-info-header">
          <Info size={14} />
          <span>设备信息</span>
        </div>
        <div class="caps-info-grid">
          {#if caps.device_info.manufacturer}
            <div class="caps-info-row">
              <span class="caps-info-label">制造商</span>
              <span class="caps-info-value">{caps.device_info.manufacturer}</span>
            </div>
          {/if}
          {#if caps.device_info.model}
            <div class="caps-info-row">
              <span class="caps-info-label">型号</span>
              <span class="caps-info-value">{caps.device_info.model}</span>
            </div>
          {/if}
          {#if caps.device_info.firmware}
            <div class="caps-info-row">
              <span class="caps-info-label">固件</span>
              <span class="caps-info-value">{caps.device_info.firmware}</span>
            </div>
          {/if}
          {#if caps.device_info.serial_number}
            <div class="caps-info-row">
              <span class="caps-info-label">序列号</span>
              <span class="caps-info-value font-mono">{caps.device_info.serial_number}</span>
            </div>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Capability badges -->
    <div class="caps-section">
      <div class="caps-section-title">设备能力</div>
      <div class="caps-badges">
        {#each badges as badge (badge.key)}
          <span class="caps-badge" class:caps-badge-on={badge.enabled} class:caps-badge-off={!badge.enabled}>
            <badge.icon size={12} />
            {badge.label}
          </span>
        {/each}
      </div>
    </div>

    <!-- PTZ detail badges -->
    {#if caps.ptz && ptzBadges.length > 0}
      <div class="caps-section">
        <div class="caps-section-title">PTZ功能</div>
        <div class="caps-badges">
          {#each ptzBadges as badge (badge.key)}
            <span class="caps-badge" class:caps-badge-on={badge.enabled} class:caps-badge-off={!badge.enabled}>
              <badge.icon size={12} />
              {badge.label}
            </span>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .caps-panel {
    padding: 0.75rem;
    background-color: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }

  .caps-loading {
    display: flex;
    justify-content: center;
    padding: 0.75rem 0;
  }

  .caps-error {
    font-size: 0.75rem;
    color: var(--color-danger);
    text-align: center;
    padding: 0.25rem 0.5rem;
    background-color: rgba(239, 68, 68, 0.1);
    border-radius: var(--radius-sm);
  }

  .caps-device-info {
    margin-bottom: 0.75rem;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--border);
  }

  .caps-info-header {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-primary);
    margin-bottom: 0.5rem;
  }

  .caps-info-grid {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .caps-info-row {
    display: flex;
    align-items: center;
    font-size: 0.6875rem;
  }

  .caps-info-label {
    width: 3.5rem;
    color: var(--text-tertiary);
    flex-shrink: 0;
  }

  .caps-info-value {
    color: var(--text-secondary);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .caps-section {
    margin-bottom: 0.5rem;
  }

  .caps-section:last-child {
    margin-bottom: 0;
  }

  .caps-section-title {
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--text-tertiary);
    margin-bottom: 0.375rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .caps-badges {
    display: flex;
    flex-wrap: wrap;
    gap: 0.375rem;
  }

  .caps-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.25rem 0.5rem;
    border-radius: 9999px;
    font-size: 0.6875rem;
    font-weight: 500;
    border: 1px solid transparent;
    transition: all var(--duration-fast) var(--ease-out);
  }

  .caps-badge-on {
    background-color: rgba(16, 185, 129, 0.1);
    color: var(--color-success-light);
    border-color: rgba(16, 185, 129, 0.2);
  }

  .caps-badge-off {
    background-color: var(--bg-tertiary);
    color: var(--text-tertiary);
    border-color: var(--border);
    opacity: 0.6;
  }
</style>
