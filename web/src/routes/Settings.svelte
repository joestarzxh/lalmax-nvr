<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { getSettings, updateSettings, getMergeSettings, updateMergeSettings, getFeatures, updateFeatures, getStats, listCameras, getStreamingSettings, updateStreamingSettings, getAiSettings, saveAiSettings, detectAiBackend, getAiBackendConfig, updateAiBackendConfig, getGB28181Settings, updateGB28181Settings, reloadConfig, checkConfigChange, regenerateLalmaxConfig, getHLSSettings, updateHLSSettings, getLocalNetworkInterfaces } from '$lib/api';
  import type { AiBackendConfig } from '$lib/api';
  import { getTranscodingCheck, getTranscodingStatus, getFFmpegStatus, downloadFFmpeg, retryDownload, getTranscodingSettings, updateTranscodingSettings } from '$lib/api/transcoding';
  import type { SelfCheckResult, DownloadStatus, HardwareCapabilities, ManagerStatus, TranscodeTask } from '$lib/api/transcoding';
  import type { SettingsConfig, FeatureFlags, StorageStats, Camera, StreamingConfig, GB28181Config, HLSConfig, NetworkInterface } from '$lib/api';
  import { getItemsPerPage, setItemsPerPage, getAutoRefresh, setAutoRefresh } from '../lib/preferences';
  import { t } from '$lib/i18n';
  import { AlertCircle, AlertTriangle, Settings as SettingsIcon, RefreshCw, CircleDot, Download, Cpu, ChevronDown, ChevronUp, RotateCw } from 'lucide-svelte';
  import { showToast } from '$lib/toast';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import Tab from '$lib/components/Tab.svelte';
  let settings = $state<SettingsConfig | null>(null);
  let loading = $state(true);
  let error = $state('');
  let saving = $state(false);

// Form state
let retentionDays = $state(30);
let diskThresholdPercent = $state(90);
let checkInterval = $state('1h');
let itemsPerPage = $state(getItemsPerPage());
  let autoRefresh = $state(getAutoRefresh());
let webdavEnabled = $state(false);
let webdavPathPrefix = $state('/dav');
let webdavReadWrite = $state(false);

// Merge settings state
let mergeEnabled = $state(true);
let mergeCheckInterval = $state('1h');
let mergeWindowSize = $state('1h');
let mergeMinSegments = $state(3);
let mergeMinSegmentAge = $state('10m');
let mergeBatchLimit = $state(100);

// Streaming settings state
let streamingDefaultProtocol = $state('webrtc');
let streamingAutoStopNoViewSec = $state(300);
let streamingWebrtcEnabled = $state(true);
let streamingWebrtcMaxViewers = $state(4);
let streamingWebrtcIdleTimeout = $state('5m');
let streamingFlvEnabled = $state(true);
let streamingFlvMaxViewers = $state(10);
let streamingRtmpEnabled = $state(false);
let streamingRtmpPort = $state(1935);
let streamingSrtEnabled = $state(false);
let streamingSrtPort = $state(9000);
let streamingSaving = $state(false);
let expandedProtocolDoc = $state<string | null>(null);

// SRT stream configurations
let srtStreams = $state<{streamId: string, cameraId: string, mode: string, address: string, passphrase: string}[]>([]);

// AI Detection state
let aiEnabled = $state(false);
let aiBackend = $state<'http' | 'webhook' | 'disabled'>('disabled');
let aiConfidenceThreshold = $state(0.5);
let aiFrameSkip = $state(3);
let aiInferenceTimeout = $state(30000);
let aiHttpEndpoint = $state('');
let aiHttpApiKey = $state('');
let aiHttpTimeout = $state(10000);
let aiWebhookEnabled = $state(false);
let aiDetectedBackend = $state('');
let aiSaving = $state(false);

// GB28181 state
let gb28181Enabled = $state(false);
let gb28181Host = $state('');
let gb28181Port = $state(5060);
let gb28181Id = $state('');
let gb28181Password = $state('');
let gb28181MediaIp = $state('');

// HLS config state
let hlsEnabled = $state(false);
let hlsOnDemand = $state(true);
let hlsIdleTimeout = $state('60s');
let hlsSegmentCount = $state(7);
let hlsLalFragmentDurationMs = $state(3000);
let hlsLalFragmentNum = $state(6);
let hlsLalCleanupMode = $state(1);
let hlsLalUseMemory = $state(false);
let hlsLalmaxSegmentDuration = $state(1);
let hlsLalmaxPartDuration = $state(200);

// Feature toggles state
let featureFlags = $state<Record<string, boolean>>({});
let featuresLoading = $state(true);
let featuresSaving = $state(false);

// Transcoding state
let transcodingEnabled = $state(false);
let transcodingMaxWorkers = $state(1);
let transcodingReplaceOriginal = $state(false);
let transcodingCheck = $state<SelfCheckResult | null>(null);
let ffmpegStatus = $state<DownloadStatus>({ status: 'not_installed', progress: 0, version: '', error: '', total_bytes: 0, downloaded_bytes: 0 });
let ffmpegDownloading = $state(false);
let hardwareInfo = $state<HardwareCapabilities | null>(null);
let checkingTranscoding = $state(false);
let showHardwareInfo = $state(false);
let ffmpegPollInterval = $state<ReturnType<typeof setInterval> | null>(null);
let transcodingCheckError = $state('');
let downloadStartTime = $state<number | null>(null);

// Derived download speed (bytes/s) and ETA (seconds)
let downloadInfo = $derived.by(() => {
  if (ffmpegStatus.status !== 'downloading' || !downloadStartTime || ffmpegStatus.downloaded_bytes <= 0) {
    return { speed: 0, eta: 0 };
  }
  const elapsed = (Date.now() - downloadStartTime) / 1000;
  if (elapsed <= 0) return { speed: 0, eta: 0 };
  const speed = ffmpegStatus.downloaded_bytes / elapsed;
  const remaining = ffmpegStatus.total_bytes - ffmpegStatus.downloaded_bytes;
  const eta = speed > 0 ? remaining / speed : 0;
  return { speed, eta };
});

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec >= 1_048_576) {
    return (bytesPerSec / 1_048_576).toFixed(1) + ' MB/s';
  }
  return Math.round(bytesPerSec / 1024) + ' KB/s';
}

function formatEta(seconds: number): string {
  if (seconds >= 60) {
    const m = Math.floor(seconds / 60);
    const s = Math.round(seconds % 60);
    return m + 'm ' + s + 's';
  }
  return Math.round(seconds) + 's';
}

// Transcoding queue status state
let managerStatus = $state<ManagerStatus | null>(null);
let queuePollInterval = $state<ReturnType<typeof setInterval> | null>(null);

// Camera list for feature toggle affected count
let allCameras = $state<Camera[]>([]);

// Disk info from stats API
let diskInfo = $state<StorageStats | null>(null);

// Original values snapshot for dirty tracking (cleanup + webdav + merge + streaming + features)
let originalSnapshot = $state('');
let originalRetentionDays = $state(0);
let originalFeatureFlags = $state<Record<string, boolean>>({});

// Settings tab state
let activeSettingsTab = $state('general');
let settingsTabs = $derived([
  { id: 'general', label: t('settings.tabs.general') },
  { id: 'advanced', label: t('settings.tabs.advanced') },
]);

// Derived: is any setting dirty?
let isDirty = $derived(() => {
    if (loading) return false;
    const current = JSON.stringify({
      retentionDays, diskThresholdPercent, checkInterval,
      webdavEnabled, webdavPathPrefix, webdavReadWrite,
      mergeEnabled, mergeCheckInterval, mergeWindowSize,
      mergeMinSegments, mergeMinSegmentAge, mergeBatchLimit,
      streamingDefaultProtocol, streamingWebrtcEnabled, streamingWebrtcMaxViewers,
      streamingWebrtcIdleTimeout, streamingFlvEnabled, streamingFlvMaxViewers,
      streamingRtmpEnabled,
      streamingRtmpPort, streamingSrtEnabled, streamingSrtPort,
      srtStreams,
      gb28181Enabled, gb28181Host, gb28181Port, gb28181Id, gb28181Password, gb28181MediaIp,
      hlsEnabled, hlsOnDemand, hlsIdleTimeout, hlsSegmentCount, hlsLalFragmentDurationMs, hlsLalFragmentNum, hlsLalCleanupMode, hlsLalUseMemory, hlsLalmaxSegmentDuration, hlsLalmaxPartDuration,
    });
    if (current !== originalSnapshot) return true;
    if (JSON.stringify(featureFlags) !== JSON.stringify(originalFeatureFlags)) return true;
    return false;
  });

// Unsaved changes navigation guard
let showNavGuard = $state(false);
let pendingHash = $state('');

function handleHashChange(e: HashChangeEvent) {
    const dirty = isDirty();
    if (dirty && !showNavGuard) {
      e.preventDefault();
      pendingHash = window.location.hash;
      showNavGuard = true;
    }
  }

function confirmNavigation() {
    showNavGuard = false;
    // Allow navigation
    window.removeEventListener('hashchange', handleHashChange);
    if (pendingHash) window.location.hash = pendingHash;
    window.addEventListener('hashchange', handleHashChange);
  }

function cancelNavigation() {
    showNavGuard = false;
    pendingHash = '';
  }

// Disk GB estimation
let diskGbEstimate = $derived(() => {
    if (!diskInfo || diskInfo.total_bytes === 0) return '';
    const remainingPct = (100 - diskThresholdPercent) / 100;
    const remainingBytes = diskInfo.total_bytes * remainingPct;
    const gb = remainingBytes / (1024 * 1024 * 1024);
    if (gb >= 1) return `≈ ${gb.toFixed(0)} GB`;
    const mb = remainingBytes / (1024 * 1024);
    return `≈ ${mb.toFixed(0)} MB`;
  });

// Affected camera count for a protocol
function getAffectedCameraCount(protocol: string): number {
    return allCameras.filter(c => c.protocol === protocol || c.protocol.startsWith(protocol)).length;
  }

  // Validation
  let validationErrors = $state<Record<string, string>>({});


  // Confirmation dialog
  let showConfirmDialog = $state(false);
  function validateField(field: string, value: string) {
    const val = parseInt(value);
    if (field === 'retention_days') {
      if (isNaN(val) || val < 0) {
        validationErrors['retention_days'] = t('settings.invalidRetentionDays');
      } else {
        delete validationErrors['retention_days'];
      }
    } else if (field === 'disk_threshold') {
      if (isNaN(val) || val < 0 || val > 100) {
        validationErrors['disk_threshold'] = t('settings.invalidDiskThreshold');
      } else {
        delete validationErrors['disk_threshold'];
      }
    }
  }

  function validate(): boolean {
    validationErrors = {};

    if (retentionDays < 1) {
      validationErrors['retention_days'] = t('settings.validationRetention');
    }

    if (diskThresholdPercent < 0 || diskThresholdPercent > 100) {
      validationErrors['disk_threshold'] = t('settings.validationThreshold');
    }

    return Object.keys(validationErrors).length === 0;
  }

  function captureSnapshot() {
    originalSnapshot = JSON.stringify({
      retentionDays, diskThresholdPercent, checkInterval,
      webdavEnabled, webdavPathPrefix, webdavReadWrite,
      mergeEnabled, mergeCheckInterval, mergeWindowSize,
      mergeMinSegments, mergeMinSegmentAge, mergeBatchLimit,
      streamingDefaultProtocol, streamingWebrtcEnabled, streamingWebrtcMaxViewers,
      streamingWebrtcIdleTimeout, streamingFlvEnabled, streamingFlvMaxViewers,
      streamingRtmpEnabled,
      streamingRtmpPort, streamingSrtEnabled, streamingSrtPort,
      srtStreams,
      gb28181Enabled, gb28181Host, gb28181Port, gb28181Id, gb28181Password, gb28181MediaIp,
      hlsEnabled, hlsOnDemand, hlsIdleTimeout, hlsSegmentCount, hlsLalFragmentDurationMs, hlsLalFragmentNum, hlsLalCleanupMode, hlsLalUseMemory, hlsLalmaxSegmentDuration, hlsLalmaxPartDuration,
    });
    originalRetentionDays = retentionDays;
    originalFeatureFlags = { ...featureFlags };
  }

  function extractIPv4Address(address: string): string | null {
    const ip = address.split('/')[0]?.trim();
    if (!ip || ip === '0.0.0.0' || ip.startsWith('127.') || ip.startsWith('169.254.')) {
      return null;
    }
    if (!/^(?:\d{1,3}\.){3}\d{1,3}$/.test(ip)) {
      return null;
    }
    const parts = ip.split('.').map(Number);
    if (parts.some(part => part < 0 || part > 255)) {
      return null;
    }
    return ip;
  }

  function getPreferredInterfaceIP(interfaces: NetworkInterface[]): string {
    const connected = interfaces.filter(iface => iface.is_up && !iface.is_loopback);
    const candidates = connected.length > 0 ? connected : interfaces;
    for (const iface of candidates) {
      for (const address of iface.addresses || []) {
        const ip = extractIPv4Address(address);
        if (ip) return ip;
      }
    }
    return '';
  }

  async function getDefaultGB28181IP(): Promise<string> {
    try {
      const data = await getLocalNetworkInterfaces();
      return getPreferredInterfaceIP(data.interfaces || []);
    } catch (e) {
      console.warn('Failed to load local network interfaces:', e);
      return '';
    }
  }

  async function loadSettings() {
    loading = true;
    error = '';

    try {
      settings = await getSettings();
      retentionDays = settings.cleanup.retention_days;
      diskThresholdPercent = settings.cleanup.disk_threshold_percent;
      checkInterval = settings.cleanup.check_interval;
      webdavEnabled = settings.webdav?.enabled ?? false;
      webdavPathPrefix = settings.webdav?.path_prefix ?? '/dav';
      webdavReadWrite = settings.webdav?.read_write ?? false;

      // Load merge settings
      const mergeSettings = await getMergeSettings();
      mergeEnabled = mergeSettings.enabled ?? true;
      mergeCheckInterval = mergeSettings.check_interval ?? '1h';
      mergeWindowSize = mergeSettings.window_size ?? '1h';
      mergeMinSegments = mergeSettings.min_segments_to_merge ?? 3;
      mergeMinSegmentAge = mergeSettings.min_segment_age ?? '10m';
      mergeBatchLimit = mergeSettings.batch_limit ?? 100;

      // Load transcoding settings
      try {
        const transcodingCfg = await getTranscodingSettings();
        transcodingEnabled = transcodingCfg.enabled;
        transcodingMaxWorkers = transcodingCfg.max_workers || 1;
        transcodingReplaceOriginal = transcodingCfg.replace_original || false;
        if (transcodingEnabled) {
          // Load hardware info + FFmpeg status + queue polling
          try {
            const checkResult = await getTranscodingCheck();
            transcodingCheck = checkResult;
            hardwareInfo = {
              h264_encoder: checkResult.encoders.h264 || '',
              h265_encoder: checkResult.encoders.h265 || '',
              total_cores: checkResult.total_cores,
              total_memory_mb: checkResult.total_memory_mb,
              estimated_fps: checkResult.estimated_fps,
              max_concurrent_streams: checkResult.max_concurrent,
              h264_encoder_type: checkResult.h264_encoder_type,
              h265_encoder_type: checkResult.h265_encoder_type,
              devices: checkResult.devices,
              arch: '',
              ffmpeg_available: checkResult.supported,
            };
          } catch (e) {
            console.warn('Failed to load transcoding hardware info:', e);
          }
          refreshFfmpegStatus();
          startQueuePolling();
        }
      } catch (e) {
        console.warn('Failed to load transcoding settings:', e);
      }

      // Load GB28181 settings
      try {
        const gb28181Cfg = await getGB28181Settings();
        gb28181Enabled = gb28181Cfg.enabled ?? false;
        gb28181Host = gb28181Cfg.host ?? '';
        gb28181Port = gb28181Cfg.port ?? 5060;
        gb28181Id = gb28181Cfg.id ?? '';
        gb28181Password = gb28181Cfg.password ?? '';
        gb28181MediaIp = gb28181Cfg.media_ip ?? '';
        if (!gb28181Host || !gb28181MediaIp) {
          const defaultIP = await getDefaultGB28181IP();
          if (defaultIP) {
            if (!gb28181Host) gb28181Host = defaultIP;
            if (!gb28181MediaIp) gb28181MediaIp = defaultIP;
          }
        }
      } catch (e) {
        console.warn('Failed to load GB28181 settings:', e);
      }

      // Load HLS settings
      try {
        const hlsCfg = await getHLSSettings();
        hlsEnabled = hlsCfg.enabled ?? false;
        hlsOnDemand = hlsCfg.on_demand ?? true;
        hlsIdleTimeout = hlsCfg.idle_timeout || '60s';
        hlsSegmentCount = hlsCfg.segment_count ?? 7;
        hlsLalFragmentDurationMs = hlsCfg.lal_fragment_duration_ms ?? 3000;
        hlsLalFragmentNum = hlsCfg.lal_fragment_num ?? 6;
        hlsLalCleanupMode = hlsCfg.lal_cleanup_mode ?? 1;
        hlsLalUseMemory = hlsCfg.lal_use_memory ?? false;
        hlsLalmaxSegmentDuration = hlsCfg.lalmax_segment_duration ?? 1;
        hlsLalmaxPartDuration = hlsCfg.lalmax_part_duration ?? 200;
      } catch (e) {
        console.warn('Failed to load HLS settings:', e);
      }

      captureSnapshot();
    } catch (e) {
      error = e instanceof Error ? e.message : t('common.failedLoadSettings');
    } finally {
      loading = false;
    }
  }

  async function save() {
    if (!validate()) return;

    // Check if we're reducing retention (destructive change)
    if (retentionDays < originalRetentionDays && originalRetentionDays > 0) {
      showConfirmDialog = true;
      return;
    }

    await performSave();
  }

  async function performSave() {
    saving = true;
    try {
      const payload: SettingsConfig = {
        cleanup: {
          retention_days: retentionDays,
          disk_threshold_percent: diskThresholdPercent,
          check_interval: checkInterval,
        },
        webdav: {
          enabled: webdavEnabled,
          path_prefix: webdavPathPrefix,
          read_write: webdavReadWrite,
        },
      };

      await updateSettings(payload);

      // Save merge settings
      await updateMergeSettings({
        enabled: mergeEnabled,
        check_interval: mergeCheckInterval,
        window_size: mergeWindowSize,
        min_segments_to_merge: mergeMinSegments,
        min_segment_age: mergeMinSegmentAge,
        batch_limit: mergeBatchLimit,
      });

      // Save streaming settings
      await updateStreamingSettings({
        default_protocol: streamingDefaultProtocol,
        webrtc: {
          enabled: streamingWebrtcEnabled,
          max_viewers: streamingWebrtcMaxViewers,
          idle_timeout: streamingWebrtcIdleTimeout,
        },
        flv: {
          enabled: streamingFlvEnabled,
          max_viewers: streamingFlvMaxViewers,
          idle_timeout: '5m',
        },

        rtmp: {
          enabled: streamingRtmpEnabled,
          port: streamingRtmpPort,
        },
        srt: {
          enabled: streamingSrtEnabled,
          port: streamingSrtPort,
          streams: srtStreams.map(s => ({
            stream_id: s.streamId,
            camera_id: s.cameraId,
            mode: s.mode,
            address: s.address,
            passphrase: s.passphrase,
          })),
        },
      });

      // Save feature toggles
      await updateFeatures({ protocols: featureFlags });

      // Save GB28181 settings
      await updateGB28181Settings({
        enabled: gb28181Enabled,
        host: gb28181Host,
        port: gb28181Port,
        id: gb28181Id,
        password: gb28181Password,
        media_ip: gb28181MediaIp,
      });

      // Save HLS settings
      if (!hlsEnabled && (streamingDefaultProtocol === 'hls' || streamingDefaultProtocol === 'll-hls')) {
        streamingDefaultProtocol = 'webrtc';
      }
      await updateHLSSettings({
        enabled: hlsEnabled,
        on_demand: hlsOnDemand,
        idle_timeout: hlsIdleTimeout,
        segment_count: hlsSegmentCount,
        lal_fragment_duration_ms: hlsLalFragmentDurationMs,
        lal_fragment_num: hlsLalFragmentNum,
        lal_cleanup_mode: hlsLalCleanupMode,
        lal_use_memory: hlsLalUseMemory,
        lalmax_segment_duration: hlsLalmaxSegmentDuration,
        lalmax_part_duration: hlsLalmaxPartDuration,
      });

      // Refresh state
      settings = await getSettings();
      captureSnapshot();
      showToast(t('settings.saved'), 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : t('common.failedSaveSettings'), 'error');
    } finally {
      saving = false;
    }
  }

  function confirmSave() {
    showConfirmDialog = false;
    performSave();
  }

  function cancelSave() {
    showConfirmDialog = false;
  }

  function handleItemsPerPageChange() {
    setItemsPerPage(itemsPerPage);
  }

  function handleAutoRefreshChange(event: Event) {
    const select = event.target as HTMLSelectElement;
    setAutoRefresh(select.value);
  }

  // Config conflict detection
  let configChanged = $state(false);
  let configCheckInterval = $state<ReturnType<typeof setInterval> | null>(null);

  async function checkForConfigChanges() {
    try {
      const result = await checkConfigChange();
      configChanged = result.changed;
    } catch { /* ignore */ }
  }

  async function handleReloadConfig() {
    try {
      const result = await reloadConfig();
      if (result.status === 'reloaded') {
        showToast(t('settings.config.reloaded'), 'success');
        configChanged = false;
        // Reload all settings from fresh config
        await loadSettings();
        await loadStreamingConfig();
      } else {
        showToast(t('settings.config.noChanges'), 'info');
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Reload failed', 'error');
    }
  }

  async function handleRegenerateLalmax() {
    try {
      await regenerateLalmaxConfig();
      showToast(t('settings.lalmax.regenerate') + ' ✓', 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Regenerate failed', 'error');
    }
  }

  onMount(() => {
    loadSettings();
    loadFeatures();
    loadDiskInfo();
    loadCameraList();
    loadStreamingConfig();
    loadAiSettings();
    loadAiBackendSettings();
    window.addEventListener('hashchange', handleHashChange);
    // Check for external config changes every 30s
    configCheckInterval = setInterval(checkForConfigChanges, 30000);
    checkForConfigChanges();
  });

  onDestroy(() => {
    window.removeEventListener('hashchange', handleHashChange);
    stopFfmpegPolling();
    stopQueuePolling();
    if (configCheckInterval) clearInterval(configCheckInterval);
  });

  async function loadFeatures() {
    featuresLoading = true;
    try {
      const data = await getFeatures();
      featureFlags = data.protocols;
      originalFeatureFlags = { ...data.protocols };
    } catch (e) { console.warn('Failed to load feature flags:', e); featureFlags = {}; } finally {
      featuresLoading = false;
    }
  }

  async function saveFeatures() {
    featuresSaving = true;
    try {
      await updateFeatures({ protocols: featureFlags });
      originalFeatureFlags = { ...featureFlags };
      showToast(t('settings.featureToggles.saved'), 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : t('settings.featureToggles.error'), 'error');
    } finally {
      featuresSaving = false;
    }
  }

  async function loadDiskInfo() {
    try {
      diskInfo = await getStats();
    } catch (e) { /* non-critical */ }
  }

  async function loadCameraList() {
    try {
      allCameras = await listCameras();
    } catch (e) { /* non-critical */ }
  }

  async function loadStreamingConfig() {
    try {
      const config = await getStreamingSettings();
      streamingDefaultProtocol = config.default_protocol || 'webrtc';
      streamingAutoStopNoViewSec = config.auto_stop_no_view_sec ?? 300;
      streamingWebrtcEnabled = config.webrtc?.enabled ?? true;
      streamingWebrtcMaxViewers = config.webrtc?.max_viewers ?? 4;
      streamingWebrtcIdleTimeout = config.webrtc?.idle_timeout || '5m';
      streamingFlvEnabled = config.flv?.enabled ?? true;
      streamingFlvMaxViewers = config.flv?.max_viewers ?? 10;
      streamingRtmpEnabled = config.rtmp?.enabled ?? false;
      streamingRtmpPort = config.rtmp?.port ?? 1935;
      streamingSrtEnabled = config.srt?.enabled ?? false;
      streamingSrtPort = config.srt?.port ?? 9000;
      // Load SRT streams
      const srtStreamList = config.srt?.streams;
      srtStreams = srtStreamList
        ? srtStreamList.map((s) => ({
            streamId: s.stream_id || '',
            cameraId: s.camera_id || '',
            mode: s.mode || 'listener',
            address: s.address || '',
            passphrase: s.passphrase || '',
          }))
        : [];
    } catch (e) { console.warn('Failed to load streaming settings:', e); }
    captureSnapshot();
  }

  async function saveStreamingSettings() {
    streamingSaving = true;
    try {
      await updateStreamingSettings({
        default_protocol: streamingDefaultProtocol,
        auto_stop_no_view_sec: streamingAutoStopNoViewSec,
        webrtc: {
          enabled: streamingWebrtcEnabled,
          max_viewers: streamingWebrtcMaxViewers,
          idle_timeout: streamingWebrtcIdleTimeout,
        },
        flv: {
          enabled: streamingFlvEnabled,
          max_viewers: streamingFlvMaxViewers,
          idle_timeout: '5m',
        },

        rtmp: {
          enabled: streamingRtmpEnabled,
          port: streamingRtmpPort,
        },
        srt: {
          enabled: streamingSrtEnabled,
          port: streamingSrtPort,
          streams: srtStreams.map(s => ({
            stream_id: s.streamId,
            camera_id: s.cameraId,
            mode: s.mode,
            address: s.address,
            passphrase: s.passphrase,
          })),
        },
      });
      showToast(t('settings.streaming.saved'), 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : t('settings.streaming.error'), 'error');
    } finally {
      streamingSaving = false;
    }
  }

  // --- AI Detection ---

  async function loadAiBackendSettings() {
    try {
      const config = await getAiBackendConfig();
      aiEnabled = config.enabled;
      aiBackend = config.backend;
      aiConfidenceThreshold = config.confidence_threshold;
      aiFrameSkip = config.frame_skip_rate;
      aiInferenceTimeout = config.inference_timeout_ms;
      if (config.http) {
        aiHttpEndpoint = config.http.endpoint;
        aiHttpApiKey = config.http.api_key;
        aiHttpTimeout = config.http.timeout;
      }
      if (config.webhook) {
        aiWebhookEnabled = config.webhook.enabled;
      }
    } catch (e) {
      console.warn('Failed to load AI backend config:', e);
    }
  }

  function loadAiSettings() {
    const settings = getAiSettings();
    aiDetectedBackend = detectAiBackend();
  }

  async function saveAiBackendSettings() {
    aiSaving = true;
    try {
      const config: Partial<AiBackendConfig> = {
        enabled: aiEnabled,
        backend: aiEnabled ? aiBackend : 'disabled',
        frame_skip_rate: aiFrameSkip,
        confidence_threshold: aiConfidenceThreshold,
        inference_timeout_ms: aiInferenceTimeout,
      };

      if (aiBackend === 'http') {
        config.http = {
          endpoint: aiHttpEndpoint,
          api_key: aiHttpApiKey,
          headers: {},
          timeout: aiHttpTimeout,
        };
      }

      if (aiBackend === 'webhook') {
        config.webhook = {
          enabled: aiWebhookEnabled,
        };
      }

      await updateAiBackendConfig(config);
      showToast('AI 配置已保存', 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : '保存 AI 配置失败', 'error');
    } finally {
      aiSaving = false;
    }
  }

  function saveAiSettingsLocal() {
    saveAiSettings({
      enabled: aiEnabled,
      confidenceThreshold: aiConfidenceThreshold,
      frameSkip: aiFrameSkip,
    });
    showToast(t('settings.ai.saved'), 'success');
  }

  // --- Transcoding ---

  async function handleTranscodingToggle() {
    if (transcodingEnabled) {
      // Disabling — persist to backend, no self-check needed
      try {
        await updateTranscodingSettings({ enabled: false });
        transcodingEnabled = false;
        stopFfmpegPolling();
        stopQueuePolling();
        managerStatus = null;
      } catch (e) {
        showToast(e instanceof Error ? e.message : 'Failed to disable transcoding', 'error');
      }
      return;
    }

    // Enabling — run self-check first
    checkingTranscoding = true;
    transcodingCheckError = '';
    try {
      // Step 1: Run hardware self-check
      let result: SelfCheckResult;
      try {
        result = await getTranscodingCheck();
      } catch (e) {
        throw new Error(e instanceof Error ? e.message : 'Self-check request failed');
      }

      transcodingCheck = result;

      // Build HardwareCapabilities from API response
      const hw: HardwareCapabilities = {
        h264_encoder: result.encoders?.h264 || '',
        h265_encoder: result.encoders?.h265 || '',
        total_cores: result.total_cores || 0,
        total_memory_mb: result.total_memory_mb || 0,
        estimated_fps: result.estimated_fps || 0,
        max_concurrent_streams: result.max_concurrent || 0,
        h264_encoder_type: result.h264_encoder_type || '',
        h265_encoder_type: result.h265_encoder_type || '',
        devices: result.devices || [],
        arch: '',
        ffmpeg_available: result.supported || false,
      };
      hardwareInfo = hw;

      if (!result.supported) {
        transcodingEnabled = false;
        const warnings = result.warnings?.length ? result.warnings.join('; ') : t('transcoding.self_check_failed');
        transcodingCheckError = warnings;
        showToast(t('transcoding.self_check_failed'), 'error');
        return;
      }

      // Step 2: Persist enabled=true to backend
      try {
        await updateTranscodingSettings({
          enabled: true,
          max_workers: transcodingMaxWorkers,
          replace_original: transcodingReplaceOriginal,
        });
      } catch (e) {
        throw new Error(e instanceof Error ? e.message : 'Failed to save transcoding settings');
      }

      // Step 3: Update local state
      transcodingEnabled = true;
      showToast(t('transcoding.self_check_passed') + ' — ' + t('transcoding.restart_required'), 'success');

      // Step 4: Load FFmpeg status (non-critical, don't fail if this errors)
      try {
        await refreshFfmpegStatus();
      } catch (e) {
        console.warn('Failed to refresh FFmpeg status after enabling transcoding:', e);
      }

      // Step 5: Start queue polling (non-critical)
      try {
        startQueuePolling();
      } catch (e) {
        console.warn('Failed to start queue polling:', e);
      }

    } catch (e) {
      // Handle any error from steps 1-2
      transcodingEnabled = false;
      transcodingCheckError = e instanceof Error ? e.message : t('transcoding.self_check_failed');
      showToast(transcodingCheckError, 'error');
    } finally {
      // Always reset checking state, regardless of success or failure
      checkingTranscoding = false;
    }
  }

  async function refreshFfmpegStatus() {
    try {
      const status = await getFFmpegStatus();
      ffmpegStatus = status;
      if (status.status === 'downloading') {
        ffmpegDownloading = true;
        if (downloadStartTime === null) {
          downloadStartTime = Date.now();
        }
        startFfmpegPolling();
      } else {
        ffmpegDownloading = false;
        downloadStartTime = null;
        stopFfmpegPolling();
      }
    } catch (e) {
      console.warn('Failed to get FFmpeg status:', e);
    }
  }

  function startFfmpegPolling() {
    stopFfmpegPolling();
    ffmpegPollInterval = setInterval(async () => {
      try {
        const status = await getFFmpegStatus();
        ffmpegStatus = status;
        if (status.status !== 'downloading') {
          ffmpegDownloading = false;
          downloadStartTime = null;
          stopFfmpegPolling();
          if (status.status === 'available') {
            showToast(t('transcoding.ffmpeg_available'), 'success');
          } else if (status.status === 'failed') {
            showToast(t('transcoding.ffmpeg_failed'), 'error');
          }
        }
      } catch (e) {
        stopFfmpegPolling();
        ffmpegDownloading = false;
        downloadStartTime = null;
      }
    }, 1000);
  }

  function stopFfmpegPolling() {
    if (ffmpegPollInterval) {
      clearInterval(ffmpegPollInterval);
      ffmpegPollInterval = null;
    }
  }

  async function handleDownloadFFmpeg() {
    ffmpegDownloading = true;
    ffmpegStatus = { ...ffmpegStatus, status: 'downloading', progress: 0, error: '' };
    downloadStartTime = Date.now();
    try {
      await downloadFFmpeg();
      startFfmpegPolling();
    } catch (e) {
      ffmpegDownloading = false;
      ffmpegStatus = { ...ffmpegStatus, status: 'failed', error: e instanceof Error ? e.message : 'Download failed' };
      showToast(t('transcoding.ffmpeg_failed'), 'error');
    }
  }

  async function handleRetryDownload() {
    ffmpegDownloading = true;
    ffmpegStatus = { ...ffmpegStatus, status: 'downloading', progress: 0, error: '' };
    downloadStartTime = Date.now();
    try {
      await retryDownload();
      startFfmpegPolling();
    } catch (e) {
      ffmpegDownloading = false;
      ffmpegStatus = { ...ffmpegStatus, status: 'failed', error: e instanceof Error ? e.message : 'Download failed' };
      showToast(t('transcoding.ffmpeg_failed'), 'error');
    }
  }

  // --- Transcoding Queue Status Polling ---

  function startQueuePolling() {
    stopQueuePolling();
    loadQueueStatus();
    queuePollInterval = setInterval(loadQueueStatus, 5000);
  }

  function stopQueuePolling() {
    if (queuePollInterval) {
      clearInterval(queuePollInterval);
      queuePollInterval = null;
    }
  }

  async function loadQueueStatus() {
    try {
      managerStatus = await getTranscodingStatus();
    } catch (e) {
      console.warn('Failed to load transcoding status:', e);
    }
  }
</script>

<div class="min-h-screen th-bg-primary ">
  <!-- Main content -->
  <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <div class="mb-6">
      <h2 class="text-2xl font-bold th-text-primary">{t('settings.title')}
        {#if isDirty()}
          <span class="text-xs font-normal th-color-warning ml-2 inline-flex items-center gap-1"><CircleDot size={12} />{t('settings.unsavedChanges')}</span>
        {/if}
      </h2>
    </div>

    <!-- Error message -->
    {#if error}
      <div class="card border th-border-danger p-8 text-center">
        <div class="flex justify-center mb-4 th-color-danger">
          <AlertCircle size={48} />
        </div>
        <h3 class="text-lg font-medium th-text-primary mb-2">{t('common.error')}</h3>
        <p class="th-text-secondary mb-4">{error}</p>
        <button onclick={loadSettings} class="btn btn-primary btn-sm">{t('common.retry')}</button>
      </div>
    {/if}

    <!-- Loading state -->
    {#if loading}
      <div class="card border th-border">
        <div class="p-6 space-y-4">
          <div class="h-6 w-40 th-bg-tertiary rounded animate-pulse"></div>
          <div class="h-4 w-64 th-bg-tertiary rounded animate-pulse"></div>
          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div class="space-y-2">
              <div class="h-4 w-24 th-bg-tertiary rounded animate-pulse"></div>
              <div class="h-10 th-bg-tertiary rounded animate-pulse"></div>
            </div>
            <div class="space-y-2">
              <div class="h-4 w-32 th-bg-tertiary rounded animate-pulse"></div>
              <div class="h-3 w-full th-bg-tertiary rounded animate-pulse"></div>
              <div class="h-10 th-bg-tertiary rounded animate-pulse"></div>
            </div>
            <div class="space-y-2">
              <div class="h-4 w-28 th-bg-tertiary rounded animate-pulse"></div>
              <div class="h-10 th-bg-tertiary rounded animate-pulse"></div>
            </div>
          </div>
          <div class="flex items-center gap-4 pt-2">
            <div class="h-10 w-28 th-bg-tertiary rounded animate-pulse"></div>
          </div>
        </div>
      </div>
    {:else}
      <Tab tabs={settingsTabs} activeTab={activeSettingsTab} onchange={(id) => activeSettingsTab = id} />
      <div class="space-y-6 mt-6">
      {#if activeSettingsTab === 'general'}
        <!-- Cleanup Policy -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.cleanup')}</h3>
          <p class="text-sm th-text-tertiary mb-8">{t('settings.cleanupDesc')}</p>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <!-- Retention Days -->
            <div>
              <label for="retention" class="input-label">{t('settings.retentionDays')}</label>
              <input
                id="retention"
                type="number"
                class="input {validationErrors['retention_days'] ? 'border-red-500' : ''}"
                bind:value={retentionDays}
                min="1"
                onblur={() => validateField('retention_days', String(retentionDays))}
                oninput={() => { if (validationErrors['retention_days']) delete validationErrors['retention_days']; }}
              />
              {#if validationErrors['retention_days']}
                <p class="th-color-danger text-xs mt-1" aria-live="polite">{validationErrors['retention_days']}</p>
              {/if}
            </div>

            <!-- Disk Threshold -->
            <div>
              <label for="threshold" class="input-label">{t('settings.diskThreshold', { percent: String(diskThresholdPercent) })}</label>
              <input
                id="threshold"
                type="number"
                class="input {validationErrors['disk_threshold'] ? 'border-red-500' : ''}"
                bind:value={diskThresholdPercent}
                min="0"
                max="100"
                onblur={() => validateField('disk_threshold', String(diskThresholdPercent))}
                oninput={() => { if (validationErrors['disk_threshold']) delete validationErrors['disk_threshold']; }}
              />
              {#if validationErrors['disk_threshold']}
                <p class="th-color-danger text-xs mt-1" aria-live="polite">{validationErrors['disk_threshold']}</p>
              {/if}
              {#if diskGbEstimate()}
                <p class="text-xs th-text-muted mt-1">{diskThresholdPercent}% {t('settings.diskRemaining')} {diskGbEstimate()}</p>
              {/if}
            </div>

            <!-- Check Interval -->
            <div>
              <label for="interval" class="input-label">{t('settings.checkInterval')}</label>
              <select id="interval" class="input" bind:value={checkInterval}>
                <option value="30m">{t('settings.every30m')}</option>
                <option value="1h">{t('settings.every1h')}</option>
                <option value="6h">{t('settings.every6h')}</option>
                <option value="24h">{t('settings.every24h')}</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Frontend Preferences -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.frontendPrefs')}</h3>
          <p class="text-sm th-text-tertiary mb-8">{t('settings.frontendPrefsDesc')}</p>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <!-- Items Per Page -->
            <div>
              <label for="itemsPerPage" class="input-label">{t('settings.itemsPerPage')}</label>
              <select id="itemsPerPage" class="input" bind:value={itemsPerPage} onchange={handleItemsPerPageChange}>
                <option value={20}>20</option>
                <option value={50}>50</option>
                <option value={100}>100</option>
              </select>
            </div>

            <!-- Auto Refresh -->
            <div>
              <label for="autoRefresh" class="input-label">{t('settings.autoRefresh')}</label>
              <select id="autoRefresh" class="input" bind:value={autoRefresh} onchange={handleAutoRefreshChange}>
                <option value="30s">{t('settings.every30s')}</option>
                <option value="60s">{t('settings.every60s')}</option>
                <option value="120s">{t('settings.every2m')}</option>
                <option value="off">{t('settings.off')}</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Default Protocol Selector -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.streaming.defaultProtocol')}</h3>
          <p class="text-sm th-text-tertiary mb-8">{t('settings.streaming.defaultProtocolHint')}</p>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div>
              <label for="defaultProtocol" class="input-label">{t('settings.streaming.defaultProtocol')}</label>
              <select id="defaultProtocol" class="input" bind:value={streamingDefaultProtocol}>
                <option value="webrtc">WebRTC</option>
                <option value="flv">HTTP-FLV</option>
                <option value="ws-flv">WS-FLV</option>
                {#if hlsEnabled}
                  <option value="hls">HLS</option>
                  <option value="ll-hls">LL-HLS</option>
                {/if}
              </select>
              <p class="text-xs th-text-tertiary mt-1">{t('settings.streaming.defaultProtocolHint')}</p>
            </div>
            <div>
              <label for="autoStopNoView" class="input-label">{t('settings.streaming.autoStopNoView')}</label>
              <select id="autoStopNoView" class="input" bind:value={streamingAutoStopNoViewSec}>
                <option value={0}>{t('settings.streaming.autoStopDisabled')}</option>
                <option value={60}>{t('settings.streaming.autoStop1min')}</option>
                <option value={300}>{t('settings.streaming.autoStop5min')}</option>
                <option value={600}>{t('settings.streaming.autoStop10min')}</option>
                <option value={1800}>{t('settings.streaming.autoStop30min')}</option>
              </select>
              <p class="text-xs th-text-tertiary mt-1">{t('settings.streaming.autoStopNoViewHint')}</p>
            </div>
          </div>
        </div>

        <!-- Protocol Guide -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.protocolDocs')}</h3>
          <p class="text-sm th-text-tertiary mb-6">{t('settings.protocolDocsDesc')}</p>

          <div class="space-y-3">
            {#each ['webrtc', 'fmp4', 'flv', 'hls', 'llHls'] as docKey (docKey)}
              {@const isExpanded = expandedProtocolDoc === docKey}
              <div class="border th-border rounded-lg overflow-hidden">
                <button
                  onclick={() => { expandedProtocolDoc = isExpanded ? null : docKey; }}
                  class="w-full px-4 py-3 text-left flex items-center justify-between hover:th-bg-hover transition-colors"
                >
                  <span class="font-medium th-text-primary">{t(`settings.protocolDocs.${docKey}.title`)}</span>
                  <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="transition-transform {isExpanded ? 'rotate-180' : ''} th-text-tertiary"><polyline points="6 9 12 15 18 9"></polyline></svg>
                </button>
                {#if isExpanded}
                  <div class="px-4 pb-4 pt-0 space-y-3">
                    <p class="text-sm th-text-secondary">{t(`settings.protocolDocs.${docKey}.desc`)}</p>
                    <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                      <div class="p-3 rounded-md bg-[var(--color-success)]/5 border border-[var(--color-success)]/20">
                        <div class="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-success)] mb-1">Pros</div>
                        <p class="text-xs th-text-secondary">{t(`settings.protocolDocs.${docKey}.pros`)}</p>
                      </div>
                      <div class="p-3 rounded-md bg-[var(--color-danger)]/5 border border-[var(--color-danger)]/20">
                        <div class="text-[10px] font-semibold uppercase tracking-wider text-[var(--color-danger)] mb-1">Cons</div>
                        <p class="text-xs th-text-secondary">{t(`settings.protocolDocs.${docKey}.cons`)}</p>
                      </div>
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {:else}
        <!-- Merge Strategy -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('merge.title')}</h3>
          <p class="text-sm th-text-secondary mt-1 mb-3">{t('settings.advanced.merge.description')}</p>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <!-- Enable Merge -->
            <div>
              <label class="input-label" for="merge-toggle">{t('merge.enableMerge')}</label>
              <div class="flex items-center gap-3 mt-2">
                <button
                  id="merge-toggle"
                  type="button"
                  aria-label={t('merge.enableMerge')}
                  class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {mergeEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                  onclick={() => { mergeEnabled = !mergeEnabled; }}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); mergeEnabled = !mergeEnabled; } }}
                  role="switch"
                  aria-checked={mergeEnabled}
                >
                  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {mergeEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                </button>
                <span class="text-sm th-text-secondary">{mergeEnabled ? t('merge.enabledState') : t('merge.disabledState')}</span>
              </div>
            </div>

            <!-- Check Interval -->
            <div>
              <label for="mergeInterval" class="input-label">{t('merge.checkInterval')}</label>
              <select id="mergeInterval" class="input" bind:value={mergeCheckInterval}>
                <option value="30m">{t('merge.30m')}</option>
                <option value="1h">{t('merge.1h')}</option>
                <option value="2h">{t('merge.2h')}</option>
                <option value="6h">{t('merge.6h')}</option>
              </select>
            </div>

            <!-- Window Size -->
            <div>
              <label for="mergeWindow" class="input-label">{t('merge.windowSize')}</label>
              <select id="mergeWindow" class="input" bind:value={mergeWindowSize}>
                <option value="30m">{t('merge.30m')}</option>
                <option value="1h">{t('merge.1h')}</option>
                <option value="2h">{t('merge.2h')}</option>
              </select>
            </div>

            <!-- Min Segments -->
            <div>
              <label for="mergeMinSegs" class="input-label">{t('merge.minSegments')}</label>
              <input
                id="mergeMinSegs"
                type="number"
                class="input"
                bind:value={mergeMinSegments}
                min="2"
                max="50"
              />
            </div>

            <!-- Min Segment Age -->
            <div>
              <label for="mergeMinAge" class="input-label">{t('merge.minAge')}</label>
              <select id="mergeMinAge" class="input" bind:value={mergeMinSegmentAge}>
                <option value="5m">{t('merge.5m')}</option>
                <option value="10m">{t('merge.10m')}</option>
                <option value="30m">{t('merge.30m')}</option>
                <option value="1h">{t('merge.1h')}</option>
              </select>
            </div>

            <!-- Batch Limit -->
            <div>
              <label for="mergeBatch" class="input-label">{t('merge.batchLimitLabel')}</label>
              <input
                id="mergeBatch"
                type="number"
                class="input"
                bind:value={mergeBatchLimit}
                min="10"
                max="1000"
              />
            </div>
          </div>
        </div>

        <!-- WebDAV Settings -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.webdav')}</h3>
          <p class="text-sm th-text-secondary mt-1 mb-3">{t('settings.advanced.webdav.description')}</p>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <!-- Enable WebDAV -->
            <div>
              <label class="input-label" for="webdav-toggle">{t('settings.webdavEnabled')}</label>
              <div class="flex items-center gap-3 mt-2">
                <button
                  id="webdav-toggle" aria-label={t('settings.webdavEnabled')}
                  type="button"

                  class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {webdavEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                  onclick={() => { webdavEnabled = !webdavEnabled; }}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); webdavEnabled = !webdavEnabled; } }}
                  role="switch"
                  aria-checked={webdavEnabled}
                >
                  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {webdavEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                </button>
                <span class="text-sm th-text-secondary">{webdavEnabled ? t('settings.webdavEnabledOn') : t('settings.webdavEnabledOff')}</span>
              </div>
            </div>

            <!-- Path Prefix -->
            <div>
              <label for="webdavPrefix" class="input-label">{t('settings.webdavPathPrefix')}</label>
              <input
                id="webdavPrefix"
                type="text"
                class="input"
                bind:value={webdavPathPrefix}
                placeholder="/dav"
              />
            </div>

            <!-- Read-Write Mode -->
            <div>
              <label class="input-label" for="webdav-rw-toggle">{t('settings.webdavReadWrite')}</label>
              <div class="flex items-center gap-3 mt-2">
                <button
                  id="webdav-rw-toggle" aria-label={t('settings.webdavReadWrite')}
                  type="button"

                  class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {webdavReadWrite ? 'bg-blue-600' : 'th-bg-tertiary'}"
                  onclick={() => { webdavReadWrite = !webdavReadWrite; }}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); webdavReadWrite = !webdavReadWrite; } }}
                  role="switch"
                  aria-checked={webdavReadWrite}
                >
                  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {webdavReadWrite ? 'translate-x-6' : 'translate-x-1'}"></span>
                </button>
                <span class="text-sm th-text-secondary">{webdavReadWrite ? t('settings.webdavReadWriteOn') : t('settings.webdavReadWriteOff')}</span>
              </div>
              <p class="text-xs th-text-tertiary mt-2">{t('settings.webdavReadWriteHint')}</p>
            </div>
          </div>
        </div>

        <!-- Streaming Sub-protocol Details -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.streaming')}</h3>
          <p class="text-sm th-text-secondary mt-1 mb-3">{t('settings.advanced.streaming.description')}</p>

          <!-- WebRTC Settings -->
          <div class="mt-2 pt-2">
            <h4 class="text-sm font-semibold th-text-primary mb-1">{t('settings.streaming.webrtc')}</h4>
            <p class="text-xs th-text-tertiary mb-4">{t('settings.streaming.webrtcDesc')}</p>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div>
                <label class="input-label" for="webrtc-toggle">{t('settings.streaming.webrtc')}</label>
                <div class="flex items-center gap-3 mt-2">
                  <button
                    id="webrtc-toggle" aria-label={t('settings.streaming.webrtc')}
                    type="button"

                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {streamingWebrtcEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                    onclick={() => { streamingWebrtcEnabled = !streamingWebrtcEnabled; }}
                    onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); streamingWebrtcEnabled = !streamingWebrtcEnabled; } }}
                    role="switch"
                    aria-checked={streamingWebrtcEnabled}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {streamingWebrtcEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                </div>
              </div>
              <div>
                <label for="webrtcMaxViewers" class="input-label">{t('settings.streaming.webrtc.maxViewers')}</label>
                <input id="webrtcMaxViewers" type="number" class="input" bind:value={streamingWebrtcMaxViewers} min="1" max="20" />
              </div>
              <div>
                <label for="webrtcIdleTimeout" class="input-label">{t('settings.streaming.webrtc.idleTimeout')}</label>
                <select id="webrtcIdleTimeout" class="input" bind:value={streamingWebrtcIdleTimeout}>
                  <option value="1m">1 min</option>
                  <option value="5m">5 min</option>
                  <option value="10m">10 min</option>
                  <option value="30m">30 min</option>
                </select>
                <p class="text-xs th-text-tertiary mt-1">{t('settings.streaming.webrtc.idleTimeoutHint')}</p>
              </div>
            </div>
          </div>

          <!-- FLV Settings -->
          <div class="mt-6 pt-6 border-t th-border">
            <h4 class="text-sm font-semibold th-text-primary mb-1">{t('settings.streaming.flv')}</h4>
            <p class="text-xs th-text-tertiary mb-4">{t('settings.streaming.flvDesc')}</p>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div>
                <label class="input-label" for="flv-toggle">{t('settings.streaming.flv')}</label>
                <div class="flex items-center gap-3 mt-2">
                  <button
                    id="flv-toggle" aria-label={t('settings.streaming.flv')}
                    type="button"

                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {streamingFlvEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                    onclick={() => { streamingFlvEnabled = !streamingFlvEnabled; }}
                    onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); streamingFlvEnabled = !streamingFlvEnabled; } }}
                    role="switch"
                    aria-checked={streamingFlvEnabled}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {streamingFlvEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                </div>
              </div>
              <div>
                <label for="flvMaxViewers" class="input-label">{t('settings.streaming.flv.maxViewers')}</label>
                <input id="flvMaxViewers" type="number" class="input" bind:value={streamingFlvMaxViewers} min="1" max="50" />
              </div>
            </div>
          </div>

          <!-- RTMP Ingest -->
          <div class="mt-6 pt-6 border-t th-border">
            <h4 class="text-sm font-semibold th-text-primary mb-1">{t('settings.streaming.rtmp')}</h4>
            <p class="text-xs th-text-tertiary mb-4">{t('settings.streaming.rtmpDesc')}</p>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div>
                <label class="input-label" for="rtmp-toggle">{t('settings.streaming.rtmp')}</label>
                <div class="flex items-center gap-3 mt-2">
                  <button
                    id="rtmp-toggle" aria-label={t('settings.streaming.rtmp')}
                    type="button"

                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {streamingRtmpEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                    onclick={() => { streamingRtmpEnabled = !streamingRtmpEnabled; }}
                    onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); streamingRtmpEnabled = !streamingRtmpEnabled; } }}
                    role="switch"
                    aria-checked={streamingRtmpEnabled}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {streamingRtmpEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                </div>
              </div>
              <div>
                <label for="rtmpPort" class="input-label">{t('settings.streaming.rtmp.port')}</label>
                <input id="rtmpPort" type="number" class="input" bind:value={streamingRtmpPort} min="1" max="65535" />
              </div>
              <div>
                <p class="text-xs th-text-tertiary mt-6">{t('settings.streaming.rtmp.pushHint')}</p>
              </div>
            </div>
          </div>

          <!-- SRT Receiver -->
          <div class="mt-6 pt-6 border-t th-border">
            <h4 class="text-sm font-semibold th-text-primary mb-1">{t('settings.streaming.srt')}</h4>
            <p class="text-xs th-text-tertiary mb-4">{t('settings.streaming.srtDesc')}</p>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div>
                <label class="input-label" for="srt-toggle">{t('settings.streaming.srt')}</label>
                <div class="flex items-center gap-3 mt-2">
                  <button
                    id="srt-toggle" aria-label={t('settings.streaming.srt')}
                    type="button"

                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {streamingSrtEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                    onclick={() => { streamingSrtEnabled = !streamingSrtEnabled; }}
                    onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); streamingSrtEnabled = !streamingSrtEnabled; } }}
                    role="switch"
                    aria-checked={streamingSrtEnabled}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {streamingSrtEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                </div>
              </div>
              <div>
                <label for="srtPort" class="input-label">{t('settings.streaming.srt.port')}</label>
                <input id="srtPort" type="number" class="input" bind:value={streamingSrtPort} min="1" max="65535" />
              </div>
              <div>
                <p class="text-xs th-text-tertiary mt-6">{t('settings.streaming.srt.hint')}</p>
              </div>
            </div>

            <!-- SRT Stream Configurations (visible when enabled) -->
            {#if streamingSrtEnabled}
              <div class="mt-4 pt-4 border-t th-border">
                <h5 class="text-sm font-medium th-text-primary mb-1">{t('settings.streaming.srt.streams')}</h5>
                <p class="text-xs th-text-tertiary mb-3">{t('settings.streaming.srt.streamsHint')}</p>
                {#if srtStreams.length > 0}
                  <div class="space-y-3">
                    {#each srtStreams as stream, i}
                      <div class="p-3 rounded-lg th-bg-secondary border th-border">
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                          <div>
                            <label class="text-xs th-text-tertiary" for="srt-streamId-{i}">{t('settings.streaming.srt.streamId')}</label>
                            <input id="srt-streamId-{i}" type="text" class="input text-sm mt-1" placeholder="live/my-stream" bind:value={stream.streamId} />
                          </div>

                          <div>
                            <label class="text-xs th-text-tertiary" for="srt-cameraId-{i}">{t('settings.streaming.srt.cameraId')}</label>
                            <input id="srt-cameraId-{i}" type="text" class="input text-sm mt-1" placeholder="front-door" bind:value={stream.cameraId} />
                          </div>

                          <div>
                            <label class="text-xs th-text-tertiary" for="srt-mode-{i}">{t('settings.streaming.srt.mode')}</label>
                            <select id="srt-mode-{i}" class="input text-sm mt-1" bind:value={stream.mode}>

                              <option value="listener">{t('settings.streaming.srt.modeListener')}</option>
                              <option value="caller">{t('settings.streaming.srt.modeCaller')}</option>
                            </select>
                          </div>
                          {#if stream.mode === 'caller'}
                            <div>
                              <label class="text-xs th-text-tertiary" for="srt-address-{i}">{t('settings.streaming.srt.address')}</label>
                              <input id="srt-address-{i}" type="text" class="input text-sm mt-1" placeholder="192.168.1.100:9000" bind:value={stream.address} />
                            </div>

                          {/if}
                          <div>
                            <label class="text-xs th-text-tertiary" for="srt-passphrase-{i}">{t('settings.streaming.srt.passphrase')}</label>
                            <input id="srt-passphrase-{i}" type="password" class="input text-sm mt-1" placeholder="......" bind:value={stream.passphrase} />
                          </div>

                        </div>
                        <div class="flex justify-end mt-2">
                          <button type="button" class="text-xs th-text-tertiary hover:text-red-400 transition-colors" onclick={() => { srtStreams.splice(i, 1); srtStreams = [...srtStreams]; }}>{t('common.dismiss')}</button>
                        </div>
                      </div>
                    {/each}
                  </div>
                {:else}
                  <p class="text-xs th-text-tertiary italic">{t('settings.streaming.srt.noStreams')}</p>
                {/if}
                <button type="button" class="mt-3 text-xs font-medium text-blue-500 hover:text-blue-400 transition-colors" onclick={() => { srtStreams = [...srtStreams, { streamId: '', cameraId: '', mode: 'listener', address: '', passphrase: '' }]; }}>+ {t('settings.streaming.srt.addStream')}</button>
              </div>
            {/if}
          </div>

          <!-- HLS Config -->
          <div class="mt-6 pt-6 border-t th-border">
            <h4 class="text-sm font-semibold th-text-primary mb-1">{t('settings.hls.title')}</h4>
            <p class="text-xs th-text-tertiary mb-4">{t('settings.hls.description')}</p>

            <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-4">
              <div>
                <label class="input-label" for="hls-toggle">{t('settings.streaming.hls')}</label>
                <div class="flex items-center gap-3 mt-2">
                  <button
                    id="hls-toggle" aria-label={t('settings.streaming.hls')}
                    type="button"
                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {hlsEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                    onclick={() => { hlsEnabled = !hlsEnabled; }}
                    onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); hlsEnabled = !hlsEnabled; } }}
                    role="switch"
                    aria-checked={hlsEnabled}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {hlsEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                </div>
                <p class="text-xs th-text-tertiary mt-1">{t('settings.streaming.hlsDesc')}</p>
              </div>
              <div>
                <label id="hlsOnDemandLabel" class="input-label" for="hls-on-demand-toggle">{t('settings.hls.onDemand')}</label>
                <div class="flex items-center gap-3 mt-2">
                  <button
                    id="hls-on-demand-toggle" type="button"
                    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {hlsOnDemand ? 'bg-blue-600' : 'th-bg-tertiary'}"
                    onclick={() => { hlsOnDemand = !hlsOnDemand; }}
                    onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); hlsOnDemand = !hlsOnDemand; } }}
                    role="switch" aria-checked={hlsOnDemand} aria-labelledby="hlsOnDemandLabel"
                    disabled={!hlsEnabled}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {hlsOnDemand ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                  <span class="text-sm th-text-secondary">{hlsOnDemand ? t('settings.hls.onDemandOn') : t('settings.hls.onDemandOff')}</span>
                </div>
                <p class="text-xs th-text-tertiary mt-1">{t('settings.hls.onDemandHint')}</p>
              </div>
              <div>
                <label for="hlsIdleTimeout" class="input-label">{t('settings.hls.idleTimeout')}</label>
                <select id="hlsIdleTimeout" class="input" bind:value={hlsIdleTimeout} disabled={!hlsEnabled || !hlsOnDemand}>
                  <option value="30s">30s</option>
                  <option value="60s">60s</option>
                  <option value="2m">2 min</option>
                  <option value="5m">5 min</option>
                </select>
                <p class="text-xs th-text-tertiary mt-1">{t('settings.hls.idleTimeoutHint')}</p>
              </div>
            </div>

            <div class:opacity-50={!hlsEnabled} class:pointer-events-none={!hlsEnabled}>
            <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
              <!-- Segment Count (shared) -->
              <div>
                <label for="hlsSegmentCount" class="input-label">{t('settings.hls.segmentCount')}</label>
                <input id="hlsSegmentCount" type="number" class="input" bind:value={hlsSegmentCount} min={3} max={10} />
                <p class="text-xs th-text-tertiary mt-1">{t('settings.hls.segmentCountHint')}</p>
              </div>
            </div>

            <!-- lal TS HLS -->
            <div class="mt-4 pt-4 border-t th-border">
              <h5 class="text-xs font-semibold th-text-primary mb-3">{t('settings.hls.lalTitle')}</h5>
              <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
                <div>
                  <label for="hlsLalFragmentDuration" class="input-label">{t('settings.hls.lalFragmentDuration')}</label>
                  <input id="hlsLalFragmentDuration" type="number" class="input" bind:value={hlsLalFragmentDurationMs} min={500} max={10000} step={500} />
                  <p class="text-xs th-text-tertiary mt-1">{t('settings.hls.lalFragmentDurationHint')}</p>
                </div>
                <div>
                  <label for="hlsLalFragmentNum" class="input-label">{t('settings.hls.lalFragmentNum')}</label>
                  <input id="hlsLalFragmentNum" type="number" class="input" bind:value={hlsLalFragmentNum} min={3} max={20} />
                </div>
                <div>
                  <label for="hlsLalCleanupMode" class="input-label">{t('settings.hls.lalCleanupMode')}</label>
                  <select id="hlsLalCleanupMode" class="input" bind:value={hlsLalCleanupMode}>
                    <option value={0}>{t('settings.hls.cleanupNever')}</option>
                    <option value={1}>{t('settings.hls.cleanupEnd')}</option>
                    <option value={2}>{t('settings.hls.cleanupAsap')}</option>
                  </select>
                </div>
                <div>
                  <label id="hlsLalUseMemoryLabel" class="input-label" for="hlsLalUseMemory">{t('settings.hls.lalUseMemory')}</label>
                  <div class="flex items-center gap-3 mt-2">
                    <button
                      id="hlsLalUseMemory" type="button"
                      class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {hlsLalUseMemory ? 'bg-blue-600' : 'th-bg-tertiary'}"
                      onclick={() => { hlsLalUseMemory = !hlsLalUseMemory; }}
                      role="switch" aria-checked={hlsLalUseMemory} aria-labelledby="hlsLalUseMemoryLabel"
                    >
                      <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {hlsLalUseMemory ? 'translate-x-6' : 'translate-x-1'}"></span>
                    </button>
                    <span class="text-sm th-text-secondary">{hlsLalUseMemory ? t('settings.hls.memoryOn') : t('settings.hls.memoryOff')}</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- lalmax fMP4/LL-HLS -->
            <div class="mt-4 pt-4 border-t th-border">
              <h5 class="text-xs font-semibold th-text-primary mb-3">{t('settings.hls.lalmaxTitle')}</h5>
              <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
                <div>
                  <label for="hlsLalmaxSegmentDuration" class="input-label">{t('settings.hls.lalmaxSegmentDuration')}</label>
                  <input id="hlsLalmaxSegmentDuration" type="number" class="input" bind:value={hlsLalmaxSegmentDuration} min={1} max={10} />
                </div>
                <div>
                  <label for="hlsLalmaxPartDuration" class="input-label">{t('settings.hls.lalmaxPartDuration')}</label>
                  <input id="hlsLalmaxPartDuration" type="number" class="input" bind:value={hlsLalmaxPartDuration} min={50} max={1000} step={50} />
                  <p class="text-xs th-text-tertiary mt-1">{t('settings.hls.lalmaxPartDurationHint')}</p>
                </div>
              </div>
            </div>
            </div>
          </div>

          <!-- Resource Usage Estimates -->
          <div class="mt-6 pt-6 border-t th-border">
            <h4 class="text-sm font-semibold th-text-primary mb-1">{t('settings.streaming.resourceEstimate')}</h4>
            <p class="text-xs th-text-tertiary mb-3">{t('settings.streaming.resourceEstimateDesc')}</p>
            <div class="space-y-2">
              <div class="flex items-center gap-2 text-xs th-text-secondary">
                <span class="w-2 h-2 rounded-full bg-[var(--color-danger)]"></span>
                <span>{t('settings.streaming.resource.webrtc')}</span>
              </div>
              <div class="flex items-center gap-2 text-xs th-text-secondary">
                <span class="w-2 h-2 rounded-full bg-[var(--color-warning)]"></span>
                <span>{t('settings.streaming.resource.flv')}</span>
              </div>
              <div class="flex items-center gap-2 text-xs th-text-secondary">
                <span class="w-2 h-2 rounded-full bg-[var(--color-success)]"></span>
                <span>{t('settings.streaming.resource.hls')}</span>
              </div>
            </div>
          </div>

        </div>

        <!-- AI Detection -->
        <div class="card p-8 border th-border">
          <div class="flex items-center justify-between mb-1">
            <div>
              <h3 class="text-lg font-semibold th-text-primary">{t('settings.ai.title')}</h3>
              <p class="text-sm th-text-secondary mt-1">{t('settings.ai.description')}</p>
            </div>
            <button
              id="ai-toggle" aria-label={t('settings.ai.title')}
              type="button"
              class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {aiEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
              onclick={() => { aiEnabled = !aiEnabled; }}
              onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); aiEnabled = !aiEnabled; } }}
              role="switch"
              aria-checked={aiEnabled}
            >
              <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {aiEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
            </button>
          </div>

          {#if aiEnabled}
            <div class="mt-4 pt-4 border-t th-border space-y-6">
              <!-- Backend Type Selection -->
              <div>
                <div class="input-label mb-2">AI 后端类型</div>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <button
                    type="button"
                    class="p-4 rounded-lg border-2 text-left transition-colors {aiBackend === 'http' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20' : 'th-border th-bg-hover'}"
                    onclick={() => { aiBackend = 'http'; }}
                  >
                    <div class="font-medium th-text-primary">HTTP 远程服务</div>
                    <div class="text-sm th-text-secondary mt-1">发送帧到远程 AI 服务进行检测</div>
                  </button>
                  <button
                    type="button"
                    class="p-4 rounded-lg border-2 text-left transition-colors {aiBackend === 'webhook' ? 'border-purple-500 bg-purple-50 dark:bg-purple-900/20' : 'th-border th-bg-hover'}"
                    onclick={() => { aiBackend = 'webhook'; }}
                  >
                    <div class="font-medium th-text-primary">Webhook 推送</div>
                    <div class="text-sm th-text-secondary mt-1">外部 AI 服务主动推送检测结果</div>
                  </button>
                </div>
              </div>

              <!-- HTTP Backend Config -->
              {#if aiBackend === 'http'}
                <div class="p-4 rounded-lg border th-border bg-gray-50 dark:bg-gray-800/50 space-y-4">
                  <h4 class="font-medium th-text-primary">HTTP 配置</h4>
                  <div>
                    <label class="input-label" for="ai-http-endpoint">检测服务地址</label>
                    <input
                      id="ai-http-endpoint"
                      type="text"
                      class="input mt-1"
                      bind:value={aiHttpEndpoint}
                      placeholder="http://192.168.1.100:8080/api/detect"
                    />
                    <p class="text-xs th-text-tertiary mt-1">支持 YOLO 服务、云厂商 API 或任意 HTTP AI 服务</p>
                  </div>
                  <div>
                    <label class="input-label" for="ai-http-apikey">API Key (可选)</label>
                    <input
                      id="ai-http-apikey"
                      type="password"
                      class="input mt-1"
                      bind:value={aiHttpApiKey}
                      placeholder="sk-xxxx"
                    />
                  </div>
                  <div>
                    <label class="input-label" for="ai-http-timeout">请求超时 (ms)</label>
                    <input
                      id="ai-http-timeout"
                      type="number"
                      class="input mt-1"
                      bind:value={aiHttpTimeout}
                      min="1000"
                      max="60000"
                    />
                  </div>
                </div>
              {/if}

              <!-- Webhook Backend Config -->
              {#if aiBackend === 'webhook'}
                <div class="p-4 rounded-lg border th-border bg-gray-50 dark:bg-gray-800/50 space-y-4">
                  <h4 class="font-medium th-text-primary">Webhook 配置</h4>
                  <div class="flex items-center justify-between">
                    <div>
                      <div class="font-medium th-text-primary">启用 Webhook 接收</div>
                      <div class="text-sm th-text-secondary">接受外部 AI 服务的检测结果推送</div>
                    </div>
                    <button
                      type="button"
                      class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {aiWebhookEnabled ? 'bg-purple-600' : 'th-bg-tertiary'}"
                      onclick={() => { aiWebhookEnabled = !aiWebhookEnabled; }}
                      aria-label="启用 Webhook 接收"
                    >
                      <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {aiWebhookEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                    </button>
                  </div>
                  <div class="p-3 rounded-md bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
                    <div class="text-sm text-blue-800 dark:text-blue-200">
                      <div class="font-medium mb-1">Webhook 端点:</div>
                      <code class="text-xs">POST /api/ai/webhook</code>
                      <div class="mt-2 font-medium mb-1">请求格式:</div>
                      <pre class="text-xs overflow-x-auto">{`{
  "camera_id": "cam-1",
  "detections": [
    {"label": "person", "confidence": 0.95, "box": [0.1, 0.2, 0.3, 0.4]}
  ]
}`}</pre>
                    </div>
                  </div>
                </div>
              {/if}

              <!-- Common Settings -->
              <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                <!-- Confidence Threshold -->
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <label class="input-label" for="ai-confidence-threshold">{t('settings.ai.confidenceThreshold')}</label>
                    <span class="text-sm font-medium th-text-primary">{Math.round(aiConfidenceThreshold * 100)}%</span>
                  </div>
                  <input
                    id="ai-confidence-threshold"
                    type="range"
                    class="w-full h-2 rounded-full appearance-none cursor-pointer th-bg-tertiary accent-blue-600"
                    bind:value={aiConfidenceThreshold}
                    min="0.1"
                    max="0.9"
                    step="0.1"
                  />
                  <p class="text-xs th-text-tertiary mt-1">{t('settings.ai.confidenceHint')}</p>
                </div>

                <!-- Frame Skip -->
                <div>
                  <div class="flex items-center justify-between mb-2">
                    <label class="input-label" for="ai-frame-skip">{t('settings.ai.frameSkip')}</label>
                    <span class="text-sm font-medium th-text-primary">{aiFrameSkip}</span>
                  </div>
                  <input
                    id="ai-frame-skip"
                    type="range"
                    class="w-full h-2 rounded-full appearance-none cursor-pointer th-bg-tertiary accent-blue-600"
                    bind:value={aiFrameSkip}
                    min="1"
                    max="10"
                    step="1"
                  />
                  <p class="text-xs th-text-tertiary mt-1">{t('settings.ai.frameSkipHint')}</p>
                </div>
              </div>

              <!-- Inference Timeout -->
              <div>
                <label class="input-label" for="ai-inference-timeout">推理超时 (ms)</label>
                <input
                  id="ai-inference-timeout"
                  type="number"
                  class="input mt-1 w-full"
                  bind:value={aiInferenceTimeout}
                  min="1000"
                  max="60000"
                />
                <p class="text-xs th-text-tertiary mt-1">单帧推理的最大等待时间</p>
              </div>

              <!-- Save AI Settings -->
              <div class="flex justify-end">
                <button
                  type="button"
                  class="btn btn-primary"
                  onclick={saveAiBackendSettings}
                  disabled={aiSaving}
                >
                  {#if aiSaving}
                    <span class="spinner mr-2"></span>
                  {/if}
                  {t('settings.save')}
                </button>
              </div>
            </div>
          {/if}
        </div>

        <!-- Feature Toggles -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.featureToggles.title')}</h3>
          <p class="text-sm th-text-secondary mt-1 mb-3">{t('settings.advanced.features.description')}</p>

          {#if featuresLoading}
            <div class="flex items-center gap-2 py-4 th-text-muted">
              <span class="spinner"></span>
              <span class="text-sm">{t('common.loading')}</span>
            </div>
          {:else}
            <div class="space-y-4">
              {#each Object.entries(featureFlags) as [protocol, enabled] (protocol)}
                <div class="p-4 rounded-md th-bg-hover border th-border">
                  <div class="flex items-center justify-between">
                    <div class="min-w-0 flex-1">
                      <div class="font-medium th-text-primary">{t(`settings.featureToggles.protocols.${protocol}`)}</div>
                      {#if !enabled}
                        <div class="flex items-center gap-1 mt-1 text-xs th-color-warning">
                          <AlertTriangle size={12} />
                          <span>{t('settings.featureToggles.warning')}{#if getAffectedCameraCount(protocol) > 0} <span class="th-color-danger">({getAffectedCameraCount(protocol)} {t('cameras.title').toLowerCase()})</span>{/if}</span>
                        </div>
                      {/if}
                    </div>
                    <div class="flex items-center gap-3">
                      <button
                        id="protocol-toggle-{protocol}" aria-label={t(`settings.featureToggles.protocols.${protocol}`)}
type="button"
                        class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {enabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                        onclick={() => { featureFlags[protocol] = !featureFlags[protocol]; featureFlags = featureFlags; }}
                        onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); featureFlags[protocol] = !featureFlags[protocol]; featureFlags = featureFlags; } }}
                        role="switch"
                        aria-checked={enabled}
                      >
                        <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {enabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                      </button>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Transcoding -->
        <div class="card p-8 border th-border">
          <div class="flex items-center justify-between mb-1">
            <div>
              <h3 class="text-lg font-semibold th-text-primary">{t('transcoding.title')}</h3>
              <p class="text-sm th-text-secondary mt-1">{t('transcoding.description')}</p>
            </div>
            <button
              type="button"
              class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {transcodingEnabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
              onclick={handleTranscodingToggle}
              onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleTranscodingToggle(); } }}
              role="switch"
              aria-checked={transcodingEnabled}
              disabled={checkingTranscoding}
            >
              {#if checkingTranscoding}
                <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform translate-x-1">
                  <span class="spinner !w-4 !h-4 !border-2"></span>
                </span>
              {:else}
                <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {transcodingEnabled ? 'translate-x-6' : 'translate-x-1'}"></span>
              {/if}
            </button>
          </div>

          <!-- Self-check error -->
          {#if transcodingCheckError}
            <div class="mt-3 p-3 rounded-md bg-[var(--color-danger)]/10 border border-[var(--color-danger)]/30">
              <div class="flex items-center gap-2 text-sm text-[var(--color-danger-light)]">
                <AlertCircle size={16} />
                <span>{transcodingCheckError}</span>
              </div>
            </div>
          {/if}

          <!-- Self-check passed indicator -->
          {#if transcodingEnabled && transcodingCheck?.supported}
            <div class="mt-3 p-3 rounded-md bg-[var(--color-success)]/10 border border-[var(--color-success)]/30">
              <div class="flex items-center gap-2 text-sm text-[var(--color-success-light)]">
                <svg class="w-4 h-4" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/></svg>
                <span>{t('transcoding.self_check_passed')}</span>
              </div>
            </div>
          {/if}

          <!-- FFmpeg Status Panel -->
          {#if transcodingEnabled}
            <div class="mt-4 pt-4 border-t th-border">
              <h4 class="text-sm font-semibold th-text-primary mb-3">{t('transcoding.ffmpeg_status')}</h4>

              <div class="p-4 rounded-md th-bg-hover border th-border">
                <!-- Status indicator -->
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    {#if ffmpegStatus.status === 'available'}
                      <span class="w-2.5 h-2.5 rounded-full bg-[var(--color-success)]"></span>
                      <span class="text-sm th-text-primary">{t('transcoding.ffmpeg_available')}</span>
                      {#if ffmpegStatus.version}
                        <span class="text-xs th-text-secondary">{t('transcoding.ffmpeg_version', { version: ffmpegStatus.version })}</span>
                      {/if}
                    {:else if ffmpegStatus.status === 'downloading'}
                      <span class="w-2.5 h-2.5 rounded-full bg-[var(--color-info)] animate-pulse"></span>
                      <span class="text-sm th-text-primary">{t('transcoding.ffmpeg_downloading')}</span>
                    {:else if ffmpegStatus.status === 'failed'}
                      <span class="w-2.5 h-2.5 rounded-full bg-[var(--color-danger)]"></span>
                      <span class="text-sm th-text-primary">{t('transcoding.ffmpeg_failed')}</span>
                    {:else}
                      <span class="w-2.5 h-2.5 rounded-full bg-[var(--color-warning)]"></span>
                      <span class="text-sm th-text-primary">{t('transcoding.ffmpeg_not_installed')}</span>
                    {/if}
                  </div>

                  <!-- Action button -->
                  <div>
                    {#if ffmpegStatus.status === 'not_installed'}
                      <button
                        type="button"
                        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[var(--color-info)] text-white hover:opacity-90 transition-opacity"
                        onclick={handleDownloadFFmpeg}
                      >
                        <Download size={12} />
                        {t('transcoding.ffmpeg_download')}
                      </button>
                    {:else if ffmpegStatus.status === 'failed'}
                      <button
                        type="button"
                        class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-md bg-[var(--color-warning)] text-white hover:opacity-90 transition-opacity"
                        onclick={handleRetryDownload}
                      >
                        <RotateCw size={12} />
                        {t('transcoding.ffmpeg_retry')}
                      </button>
                    {:else if ffmpegStatus.status === 'available'}
                      <!-- no action needed -->
                    {:else}
                      <!-- downloading in progress -->
                    {/if}
                  </div>
                </div>

                <!-- Progress bar (downloading) -->
                {#if ffmpegDownloading || ffmpegStatus.status === 'downloading'}
                  <div class="mt-3">
                    <div class="flex items-center justify-between text-xs th-text-secondary mb-1">
                      <span>{t('transcoding.download_progress')}</span>
                      <span>{ffmpegStatus.progress}%</span>
                    </div>
                    <div class="w-full h-2 rounded-full th-bg-tertiary overflow-hidden">
                      <div
                        class="h-full rounded-full bg-[var(--color-info)] transition-all duration-500"
                        style="width: {Math.max(ffmpegStatus.progress, 2)}%"
                      ></div>
                    </div>
                  </div>

                  <!-- Download speed + ETA -->
                  <div class="flex items-center gap-3 mt-2 text-xs th-text-secondary">
                    {#if downloadInfo.speed > 0}
                      <span>{t('transcoding.download_speed')}: {formatSpeed(downloadInfo.speed)}</span>
                    {/if}
                    {#if downloadInfo.eta > 0}
                      <span>{t('transcoding.download_eta')}: ~{formatEta(downloadInfo.eta)}</span>
                    {/if}
                  </div>
                {/if}

                <!-- Error detail -->
                {#if ffmpegStatus.status === 'failed' && ffmpegStatus.error}
                  <div class="mt-2 text-xs text-[var(--color-danger-light)]">{ffmpegStatus.error}</div>
                {/if}
              </div>

              <!-- Hardware Info Card -->
              {#if hardwareInfo}
                <button
                  type="button"
                  class="mt-3 flex items-center gap-1.5 text-sm font-medium th-text-secondary hover:th-text-primary transition-colors"
                  onclick={() => showHardwareInfo = !showHardwareInfo}
                >
                  <Cpu size={14} />
                  <span>{t('transcoding.hardware_info')}</span>
                  {#if showHardwareInfo}
                    <ChevronUp size={14} />
                  {:else}
                    <ChevronDown size={14} />
                  {/if}
                </button>

                <div class="mt-2 overflow-hidden transition-all duration-200 {showHardwareInfo ? 'max-h-[500px] opacity-100' : 'max-h-0 opacity-0'}" >
                  <div class="p-3 rounded-md th-bg-hover border th-border grid grid-cols-2 gap-3">
                    <div>
                      <div class="text-xs th-text-secondary">{t('transcoding.cpu_cores')}</div>
                      <div class="text-sm font-medium th-text-primary">{hardwareInfo.total_cores}</div>
                    </div>
                    <div>
                      <div class="text-xs th-text-secondary">{t('transcoding.memory')}</div>
                      <div class="text-sm font-medium th-text-primary">{Math.round(hardwareInfo.total_memory_mb)} MB</div>
                    </div>
                    <div>
                      <div class="text-xs th-text-secondary">{t('transcoding.encoder')}</div>
                      <div class="text-sm font-medium th-text-primary">{hardwareInfo.h264_encoder || 'software'}</div>
                    </div>
                    <div>
                      <div class="text-xs th-text-secondary">{t('transcoding.estimated_fps')}</div>
                      <div class="text-sm font-medium th-text-primary">{hardwareInfo.estimated_fps} FPS</div>
                    </div>
                    <div>
                      <div class="text-xs th-text-secondary">{t('transcoding.max_concurrent')}</div>
                      <div class="text-sm font-medium th-text-primary">{hardwareInfo.max_concurrent_streams}</div>
                    </div>
                  </div>

                  {#if hardwareInfo.estimated_fps < 15}
                    <div class="mt-2 p-2 rounded-md bg-[var(--color-warning)]/10 border border-[var(--color-warning)]/30">
                      <div class="flex items-center gap-1.5 text-xs text-[var(--color-warning-light)]">
                        <AlertTriangle size={12} />
                        <span>{t('transcoding.warning_hardware')}</span>
                      </div>
                    </div>
                  {/if}
                </div>
              {/if}
            </div>
          {/if}

          <!-- Transcoding Options -->
          {#if transcodingEnabled && ffmpegStatus.status === 'available'}
            <div class="mt-4 pt-4 border-t th-border">
              <h4 class="text-sm font-semibold th-text-primary mb-3">{t('transcoding.options')}</h4>

              <div class="space-y-3">
                <!-- Max Workers -->
                <div class="flex items-center justify-between">
                  <div>
                    <div class="text-sm th-text-primary">{t('transcoding.max_workers')}</div>
                    <div class="text-xs th-text-secondary">{t('transcoding.max_workers_desc')}</div>
                  </div>
                  <select
                    class="input w-20 text-center"
                    bind:value={transcodingMaxWorkers}
                    onchange={async () => {
                      try {
                        await updateTranscodingSettings({ enabled: true, max_workers: transcodingMaxWorkers, replace_original: transcodingReplaceOriginal });
                        showToast(t('common.saved'), 'success');
                      } catch (e) {
                        showToast(e instanceof Error ? e.message : 'Failed to save settings', 'error');
                      }
                    }}
                  >
                    <option value={1}>1</option>
                    <option value={2}>2</option>
                    <option value={3}>3</option>
                    <option value={4}>4</option>
                  </select>
                </div>

                <!-- Replace Original -->
                <div class="flex items-center justify-between">
                  <div>
                    <div class="text-sm th-text-primary">{t('transcoding.replace_original')}</div>
                    <div class="text-xs th-text-secondary">{t('transcoding.replace_original_desc')}</div>
                  </div>
                  <button
                    id="transcoding-replace-original" aria-label={t('transcoding.replace_original')}
                    type="button"
                    onclick={async () => {
                      try {
                        transcodingReplaceOriginal = !transcodingReplaceOriginal;
                        await updateTranscodingSettings({ enabled: true, max_workers: transcodingMaxWorkers, replace_original: transcodingReplaceOriginal });
                        showToast(t('common.saved'), 'success');
                      } catch (e) {
                        // Revert on error
                        transcodingReplaceOriginal = !transcodingReplaceOriginal;
                        showToast(e instanceof Error ? e.message : 'Failed to save settings', 'error');
                      }
                    }}
                    role="switch"
                    aria-checked={transcodingReplaceOriginal}
                  >
                    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {transcodingReplaceOriginal ? 'translate-x-6' : 'translate-x-1'}"></span>
                  </button>
                </div>
              </div>
            </div>
          {/if}

          <!-- Queue Status (when enabled) -->
          {#if transcodingEnabled && ffmpegStatus.status === 'available'}
            <div class="mt-4 pt-4 border-t th-border">
              <h4 class="text-sm font-semibold th-text-primary mb-3">{t('transcoding.queue_status')}</h4>

              {#if managerStatus}
                <!-- Active Jobs -->
                <div class="space-y-3">
                  {#each managerStatus.recent_results?.filter((t: TranscodeTask) => t.status === 'running') ?? [] as job}
                    <div class="p-3 rounded-md th-bg-hover border th-border">
                      <div class="flex items-center justify-between mb-2">
                        <div class="flex items-center gap-2">
                          <span class="w-2 h-2 rounded-full bg-[var(--color-info)] animate-pulse"></span>
                          <span class="text-sm font-medium th-text-primary">{job.camera_id}</span>
                        </div>
                        <span class="text-xs th-text-secondary">{t('transcoding.queue.codecConversion', { input: job.input_format?.toUpperCase() || '?', output: job.output_format?.toUpperCase() || '?' })}</span>
                      </div>
                      <div class="w-full h-2 rounded-full th-bg-tertiary overflow-hidden">
                        <div
                          class="h-full rounded-full bg-[var(--color-info)] transition-all duration-500"
                          style="width: {Math.max(job.progress || 0, 2)}%"
                        ></div>
                      </div>
                      <div class="flex items-center justify-between mt-1">
                        <span class="text-xs th-text-secondary">{t('transcoding.progress')}</span>
                        <span class="text-xs font-medium th-text-primary">{job.progress || 0}%</span>
                      </div>
                    </div>
                  {/each}

                  {#if (managerStatus.recent_results?.filter((t: TranscodeTask) => t.status === 'running') ?? []).length === 0}
                    <div class="text-sm th-text-tertiary text-center py-2">{t('transcoding.queue.noActive')}</div>
                  {/if}
                </div>

                <!-- Queue Summary -->
                <div class="mt-3 grid grid-cols-3 gap-3">
                  <div class="p-3 rounded-md th-bg-hover border th-border text-center">
                    <div class="text-lg font-semibold th-text-primary">{managerStatus.queue_length || 0}</div>
                    <div class="text-xs th-text-secondary">{t('transcoding.pending_jobs')}</div>
                  </div>
                  <div class="p-3 rounded-md th-bg-hover border th-border text-center">
                    <div class="text-lg font-semibold th-text-primary">{managerStatus.active_jobs || 0}</div>
                    <div class="text-xs th-text-secondary">{t('transcoding.active_jobs')}</div>
                  </div>
                  <div class="p-3 rounded-md th-bg-hover border th-border text-center">
                    <div class="text-lg font-semibold th-text-primary">{managerStatus.recent_results?.filter((t: TranscodeTask) => t.status === 'completed').length ?? 0}<span class="text-xs th-color-danger ml-1">{managerStatus.recent_results?.filter((t: TranscodeTask) => t.status === 'failed').length ?? 0}✗</span></div>
                    <div class="text-xs th-text-secondary">{t('transcoding.recent_results')}</div>
                  </div>
                </div>

                <!-- Recent Results -->
                {#if managerStatus.recent_results && managerStatus.recent_results.length > 0}
                  <div class="mt-3 space-y-1.5">
                    {#each managerStatus.recent_results.slice(0, 5) as task}
                      <div class="py-1 px-2 rounded th-bg-hover">
                        <div class="flex items-center justify-between text-xs">
                          <div class="flex items-center gap-2">
                            {#if task.status === 'completed'}
                              <span class="w-1.5 h-1.5 rounded-full bg-[var(--color-success)]"></span>
                            {:else if task.status === 'failed'}
                              <span class="w-1.5 h-1.5 rounded-full bg-[var(--color-danger)]"></span>
                            {:else if task.status === 'running'}
                              <span class="w-1.5 h-1.5 rounded-full bg-[var(--color-info)] animate-pulse"></span>
                            {:else}
                              <span class="w-1.5 h-1.5 rounded-full th-bg-tertiary"></span>
                            {/if}
                            <span class="th-text-primary">{task.camera_id}</span>
                            <span class="th-text-tertiary">{t('transcoding.queue.codecConversion', { input: task.input_format?.toUpperCase() || '?', output: task.output_format?.toUpperCase() || '?' })}</span>
                          </div>
                          <div class="flex items-center gap-2">
                            {#if task.status === 'completed'}
                              <span class="text-[var(--color-success)]">{task.progress}%</span>
                            {:else if task.status === 'running'}
                              <span class="text-[var(--color-info)]">{task.progress}%</span>
                            {:else if task.status === 'failed'}
                              <span class="text-[var(--color-danger)]">{t('transcoding.failed')}</span>
                            {:else}
                              <span class="th-text-tertiary">{t(`transcoding.${task.status}`) || task.status}</span>
                            {/if}
                          </div>
                        </div>
                        {#if task.status === 'failed' && task.error}
                          <details class="mt-1 group">
                            <summary class="flex items-center gap-1 cursor-pointer text-[10px] th-color-danger select-none">
                              <span>{t('transcoding.error_details')}</span>
                              <span class="th-text-tertiary group-open:rotate-180 transition-transform">▼</span>
                            </summary>
                            <pre class="mt-0.5 p-1.5 rounded text-[10px] th-bg-tertiary th-text-secondary whitespace-pre-wrap break-all max-h-24 overflow-y-auto">{task.error}</pre>
                          </details>
                        {/if}
                      </div>
                    {/each}
                  </div>
                {:else}
                  <div class="mt-3 text-xs th-text-tertiary text-center">{t('transcoding.queue.noRecent')}</div>
                {/if}
              {:else}
                <div class="text-sm th-text-tertiary text-center py-2">{t('common.loading')}</div>
              {/if}
            </div>
          {/if}
        </div>

        <!-- GB28181 -->
        <div class="card p-8 border th-border">
          <h3 class="text-lg font-semibold th-text-primary mb-1">{t('settings.gb28181.title')}</h3>
          <p class="text-sm th-text-secondary mt-1 mb-3">{t('settings.gb28181.description')}</p>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
            <!-- Enable GB28181 -->
            <div>
              <label class="input-label" for="gb28181-toggle">{t('settings.gb28181.enabled')}</label>
              <div class="flex items-center gap-3 mt-2">
                <button
                  id="gb28181-toggle"
                  type="button"
                  aria-label={t('settings.gb28181.enabled')}
                  class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 {gb28181Enabled ? 'bg-blue-600' : 'th-bg-tertiary'}"
                  onclick={() => { gb28181Enabled = !gb28181Enabled; }}
                  onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); gb28181Enabled = !gb28181Enabled; } }}
                  role="switch"
                  aria-checked={gb28181Enabled}
                >
                  <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {gb28181Enabled ? 'translate-x-6' : 'translate-x-1'}"></span>
                </button>
                <span class="text-sm th-text-secondary">{gb28181Enabled ? t('settings.gb28181.enabledState') : t('settings.gb28181.disabledState')}</span>
              </div>
            </div>

            <!-- SIP ID -->
            <div>
              <label for="gb28181Id" class="input-label">{t('settings.gb28181.id')}</label>
              <input id="gb28181Id" type="text" class="input" bind:value={gb28181Id} placeholder="34020000001320000001" maxlength={20} />
            </div>

            <!-- Password -->
            <div>
              <label for="gb28181Password" class="input-label">{t('settings.gb28181.password')}</label>
              <input id="gb28181Password" type="text" class="input" bind:value={gb28181Password} placeholder="12345678" />
            </div>

            <!-- SIP Host -->
            <div>
              <label for="gb28181Host" class="input-label">{t('settings.gb28181.host')}</label>
              <input id="gb28181Host" type="text" class="input" bind:value={gb28181Host} placeholder={t('settings.gb28181.hostPlaceholder')} />
              <p class="text-xs th-text-tertiary mt-1">{t('settings.gb28181.hostHint')}</p>
            </div>

            <!-- SIP Port -->
            <div>
              <label for="gb28181Port" class="input-label">{t('settings.gb28181.port')}</label>
              <input id="gb28181Port" type="number" class="input" bind:value={gb28181Port} min={1} max={65535} />
            </div>

            <!-- Media IP -->
            <div>
              <label for="gb28181MediaIp" class="input-label">{t('settings.gb28181.mediaIp')}</label>
              <input id="gb28181MediaIp" type="text" class="input" bind:value={gb28181MediaIp} placeholder="192.168.1.100" />
              <p class="text-xs th-text-tertiary mt-1">{t('settings.gb28181.mediaIpHint')}</p>
            </div>
          </div>
        </div>

      {/if}

        <!-- Config conflict warning -->
        {#if configChanged}
          <div class="rounded-lg p-4 border th-border flex items-center gap-3" style="background: var(--color-warning-bg, rgba(234,179,8,0.1)); border-color: var(--color-warning, #eab308);">
            <AlertCircle size={20} style="color: var(--color-warning, #eab308); flex-shrink: 0;" />
            <div class="flex-1">
              <p class="text-sm font-medium" style="color: var(--color-warning, #ca8a04);">{t('settings.config.conflict')}</p>
            </div>
            <button onclick={handleReloadConfig} class="btn btn-ghost text-sm" style="color: var(--color-warning, #ca8a04);">
              <RotateCw size={14} class="mr-1" /> {t('settings.config.reload')}
            </button>
          </div>
        {/if}

        <!-- Save + utility buttons -->
        <div class="flex items-center gap-4 pt-2">
          <button
            onclick={save}
            class="btn btn-primary"
            disabled={saving || !isDirty()}
          >
            {#if saving}
              <span class="spinner mr-2"></span>
              {t('settings.saving')}
            {:else}
              {t('settings.save')}
            {/if}
          </button>

          <button onclick={handleReloadConfig} class="btn btn-ghost text-sm" title={t('settings.config.reloadHint')}>
            <RotateCw size={14} class="mr-1" /> {t('settings.config.reload')}
          </button>

          <button onclick={handleRegenerateLalmax} class="btn btn-ghost text-sm" title={t('settings.lalmax.regenerateHint')}>
            <RotateCw size={14} class="mr-1" /> {t('settings.lalmax.regenerate')}
          </button>
        </div>
      </div>
    {/if}
  </main>


  <!-- Unsaved changes navigation guard -->
  {#if showNavGuard}
    <ConfirmDialog
      title={t('settings.unsavedTitle')}
      message={t('settings.unsavedMessage')}
      confirmText={t('settings.unsavedDiscard')}
      onconfirm={confirmNavigation}
      oncancel={cancelNavigation}
      variant="danger"
    />
  {/if}
</div>
