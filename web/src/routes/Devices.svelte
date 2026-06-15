<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    listGB28181Devices, playGB28181Stream, stopGB28181Stream, 
    listStreams, listCameras, deleteCamera, permanentlyDeleteCamera,
    startCamera, stopCamera, updateCamera, pauseRecording, resumeRecording,
    xiaomiDevices, listProtocols, DEFAULT_PROTOCOLS, buildProtocolsMap,
    enableCamera, disableCamera, getHealthStatus, getSnapshotUrl,
    ApiRequestError, queryDeviceRecords, startDevicePlayback,
    setPlaybackSpeed, seekPlayback,
    listGB28181Alarms, startBroadcast, stopBroadcast,
    listArchives, restoreArchiveGroup, setArchiveRetention, deleteArchiveGroup,
    listArchiveRecordings, deleteArchiveRecording, getCameraRecordingStats
  } from '$lib/api';
  import type { 
    GB28181Device, StreamInfo, Camera, XiaomiDevice, ProtocolInfo, CameraHealth,
    DeviceRecordItem, GB28181Alarm, ArchiveGroup, Recording
  } from '$lib/api';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';
  import { formatFileSize, formatDate, formatDuration } from '$lib/format';
  import { 
    Video, RefreshCw, Play, Square, Wifi, WifiOff, 
    Radio, Camera as CameraIcon, Smartphone, 
    Monitor, Search, Pencil, Pause, RotateCw, Eye,
    MoreVertical, Archive, Trash2, Image, Settings,
    Film, AlertTriangle, Download, Mic, MicOff, X,
    ChevronDown, ChevronRight, Clock, FastForward, Rewind
  } from 'lucide-svelte';
  import DiscoveryPanel from '$lib/components/DiscoveryPanel.svelte';
  import CameraCard from '$lib/components/CameraCard.svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
  import ArchiveConfirmDialog from '$lib/components/ArchiveConfirmDialog.svelte';
  import CameraForm from '$lib/components/CameraForm.svelte';
  import FlvPlayer from '../components/FlvPlayer.svelte';
  import ONVIFRecordingQuery from '$lib/components/ONVIFRecordingQuery.svelte';
  import Pagination from '../components/Pagination.svelte';

  // Device type tabs
  type DeviceType = 'onvif' | 'gb28181' | 'xiaomi' | 'push' | 'archives';
  let activeTab = $state<DeviceType>('onvif');

  // ONVIF sub-tabs
  type ONVIFSubTab = 'devices' | 'recordings';
  let onvifSubTab = $state<ONVIFSubTab>('devices');

  // GB28181 state
  let gb28181Devices = $state<GB28181Device[]>([]);
  let gb28181Loading = $state(false);
  let playingStreams = $state<Set<string>>(new Set());
  let gb28181Cameras = $state<Map<string, Camera>>(new Map()); // streamId -> Camera

  // GB28181 sub-tabs
  type GB28181SubTab = 'devices' | 'records' | 'alarms';
  let gb28181SubTab = $state<GB28181SubTab>('devices');

  // GB28181 records state
  let recordDeviceId = $state('');
  let recordChannelId = $state('');
  let recordStartTime = $state('');
  let recordEndTime = $state('');
  let recordLoading = $state(false);
  let records = $state<DeviceRecordItem[]>([]);
  let playbackStreamUrl = $state('');
  let playbackStreamId = $state('');
  let playbackSpeed = $state<0.5 | 1 | 2 | 4>(1);
  let playbackSeekTime = $state(0);
  let speedLoading = $state(false);
  let seekLoading = $state(false);

  // GB28181 alarms state
  let alarms = $state<GB28181Alarm[]>([]);
  let alarmsLoading = $state(false);
  let alarmsTotal = $state(0);
  let alarmsOffset = $state(0);
  const alarmsLimit = 50;

  // GB28181 broadcast state
  let broadcastingChannels = $state<Set<string>>(new Set());

  // ONVIF state (existing cameras)
  let onvifCameras = $state<Camera[]>([]);
  let onvifLoading = $state(false);

  // Xiaomi state (existing cameras)
  let xiaomiCameras = $state<Camera[]>([]);
  let xiaomiCamerasLoading = $state(false);

  // Push streams state
  let pushStreams = $state<StreamInfo[]>([]);
  let pushCameras = $state<Camera[]>([]);
  let pushLoading = $state(false);

  // Discovery state
  let showDiscovery = $state(false);
  let discoveryPanel = $state<DiscoveryPanel | null>(null);

  // Common state
  let loading = $state(false);
  let error = $state('');

  // Camera management state
  let protocols = $state<ProtocolInfo[]>(DEFAULT_PROTOCOLS);
  let protocolsMap = $state<Map<string, ProtocolInfo>>(buildProtocolsMap(DEFAULT_PROTOCOLS));
  let healthData = $state<Record<string, CameraHealth>>({});
  let pausedCameras = $state<Set<string>>(new Set());

  // Form state
  let showForm = $state(false);
  let editingCamera = $state<Camera | null>(null);

  // Confirmation dialog state
  let confirmAction = $state<{ camera: Camera; action: 'stop' | 'restart' } | null>(null);
  let archiveConfirm = $state<Camera | null>(null);
  let archiveLoading = $state(false);
  let permanentDeleteConfirm = $state<Camera | null>(null);
  let permanentDeleteLoading = $state(false);

  // Archive management state
  let archives = $state<ArchiveGroup[]>([]);
  let archiveConfirmCount = $state<number>(0);
  let archiveConfirmSize = $state<number>(0);
  let archiveConfirmStatsLoading = $state(false);
  let confirmDeleteArchive = $state<string | null>(null);
  let deleteArchiveLoading = $state(false);
  let restoreArchiveConfirm = $state<ArchiveGroup | null>(null);
  let restoreArchiveLoading = $state(false);
  let expandedArchiveId = $state<string | null>(null);
  let archiveRecordings = $state<Recording[]>([]);
  let archiveRecordingsTotal = $state(0);
  let archiveRecordingsOffset = $state(0);
  let archiveRecordingsLimit = $state(20);
  let archiveRecordingsLoading = $state(false);
  let deleteRecordingConfirm = $state<Recording | null>(null);
  let showRetDialog = $state(false);
  let selectedArchiveGroup = $state<ArchiveGroup | null>(null);
  let retentionDays = $state(30);

  // Xiaomi
  let xiaomiDeviceList = $state<XiaomiDevice[]>([]);

  const tabs = [
    { id: 'onvif' as DeviceType, label: 'ONVIF 设备', icon: 'camera' },
    { id: 'gb28181' as DeviceType, label: 'GB28181 设备', icon: 'monitor' },
    { id: 'xiaomi' as DeviceType, label: '小米设备', icon: 'smartphone' },
    { id: 'push' as DeviceType, label: '推流设备', icon: 'radio' },
    { id: 'archives' as DeviceType, label: '归档管理', icon: 'archive' },
  ];

  const onvifSubTabs = [
    { id: 'devices' as ONVIFSubTab, label: '设备列表', icon: CameraIcon },
    { id: 'recordings' as ONVIFSubTab, label: '设备录像', icon: Film },
  ];

  const gb28181SubTabs = [
    { id: 'devices' as GB28181SubTab, label: '设备列表', icon: Monitor },
    { id: 'records' as GB28181SubTab, label: '设备录像', icon: Film },
    { id: 'alarms' as GB28181SubTab, label: '报警记录', icon: AlertTriangle },
  ];

  // Check if current tab supports discovery
  let canDiscover = $derived(activeTab === 'onvif' || activeTab === 'xiaomi');

  function syncGB28181PlayingStreams(deviceList: GB28181Device[]) {
    const newPlaying = new Set<string>();
    for (const device of deviceList) {
      for (const channel of device.channels || []) {
        if (channel.is_playing) {
          newPlaying.add(`${device.device_id}:${channel.channel_id}`);
        }
      }
    }
    playingStreams = newPlaying;
  }

  async function loadDevices() {
    loading = true;
    error = '';
    
    try {
      if (activeTab === 'onvif') {
        await loadOnvifCameras();
      } else if (activeTab === 'gb28181') {
        await loadGB28181Devices();
      } else if (activeTab === 'xiaomi') {
        await loadXiaomiCameras();
      } else if (activeTab === 'push') {
        await loadPushStreams();
      }
    } catch (e) {
      error = e instanceof Error ? e.message : '加载失败';
    } finally {
      loading = false;
    }
    loadArchives();
  }

  const ONVIF_TAB_PROTOCOLS = new Set(['onvif', 'rtsp', 'http', 'rtsp_h264', 'rtsp_h265', 'rtsp_mjpeg', 'http_jpeg']);
  const PUSH_SOURCE_TYPES = new Set(['rtmp_push', 'srt_push', 'whip_push']);

  function isPushSourceType(sourceType?: string): boolean {
    return !!sourceType && PUSH_SOURCE_TYPES.has(sourceType);
  }

  function pushSourceLabel(sourceType?: string): string {
    if (sourceType === 'rtmp_push') return 'RTMP 推流';
    if (sourceType === 'srt_push') return 'SRT 推流';
    if (sourceType === 'whip_push') return 'WHIP 推流';
    return sourceType || '推流';
  }

  function isOnvifTabCamera(camera: Camera): boolean {
    if (isPushSourceType(camera.source_type)) return false;
    return ONVIF_TAB_PROTOCOLS.has(camera.protocol);
  }

  function isGB28181Camera(camera: Camera): boolean {
    return camera.protocol === 'gb28181';
  }

  function isXiaomiCamera(camera: Camera): boolean {
    return camera.protocol === 'xiaomi';
  }

  async function loadXiaomiCameras() {
    xiaomiCamerasLoading = true;
    try {
      const cameras = await listCameras();
      xiaomiCameras = (cameras || []).filter(isXiaomiCamera);
    } catch (e) {
      console.error('Failed to load Xiaomi cameras:', e);
    } finally {
      xiaomiCamerasLoading = false;
    }
  }

  async function loadOnvifCameras() {
    onvifLoading = true;
    try {
      const cameras = await listCameras();
      onvifCameras = (cameras || []).filter(isOnvifTabCamera);
      pausedCameras = new Set(onvifCameras.filter(c => c.recording_paused).map(c => c.id));
    } catch (e) {
      console.error('Failed to load ONVIF cameras:', e);
    } finally {
      onvifLoading = false;
    }
  }

  async function loadGB28181Devices() {
    gb28181Loading = true;
    try {
      const [res, allCameras] = await Promise.all([
        listGB28181Devices(),
        listCameras()
      ]);
      gb28181Devices = res.devices || [];
      syncGB28181PlayingStreams(gb28181Devices);
      
      // Build map of GB28181 cameras (streamId -> Camera)
      const cameraMap = new Map<string, Camera>();
      const gb28181Cams = (allCameras || []).filter(isGB28181Camera);
      for (const cam of gb28181Cams) {
        cameraMap.set(cam.id, cam);
      }
      gb28181Cameras = cameraMap;
      
      // Update paused cameras
      const paused = new Set<string>();
      for (const cam of gb28181Cams) {
        if (cam.recording_paused) paused.add(cam.id);
      }
      pausedCameras = paused;
    } catch (e) {
      console.error('Failed to load GB28181 devices:', e);
    } finally {
      gb28181Loading = false;
    }
  }

  async function loadPushStreams() {
    pushLoading = true;
    try {
      const [res, cameras] = await Promise.all([listStreams(), listCameras()]);
      pushStreams = (res.streams || []).filter(s =>
        !s.managed && isPushSourceType(s.source_type)
      );
      pushCameras = (cameras || []).filter(c => isPushSourceType(c.source_type));
    } catch (e) {
      console.error('Failed to load push streams:', e);
    } finally {
      pushLoading = false;
    }
  }

  async function loadHealth() {
    try {
      const res = await getHealthStatus();
      healthData = res;
    } catch (e) {
      console.warn('Failed to load health:', e);
    }
  }

  // Get camera associated with a GB28181 channel
  function getGB28181ChannelCamera(device: GB28181Device, channelId: string): Camera | undefined {
    const streamId = `${device.device_id}_${channelId}`;
    return gb28181Cameras.get(streamId);
  }

  async function handleGB28181Play(device: GB28181Device, channelId: string) {
    const streamKey = `${device.device_id}:${channelId}`;
    try {
      const res = await playGB28181Stream({
        device_id: device.device_id,
        channel_id: channelId,
      });
      playingStreams = new Set([...playingStreams, streamKey]);
      showToast(`推流已启动: ${res.ssrc}`, 'success');
      // Reload to get the newly created camera
      await loadGB28181Devices();
    } catch (e) {
      showToast(e instanceof Error ? e.message : '启动推流失败', 'error');
    }
  }

  async function handleGB28181Stop(device: GB28181Device, channelId: string) {
    const streamKey = `${device.device_id}:${channelId}`;
    try {
      await stopGB28181Stream(device.device_id, channelId);
      playingStreams = new Set([...playingStreams].filter(k => k !== streamKey));
      showToast('推流已停止', 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : '停止推流失败', 'error');
    }
  }

  async function handleStartBroadcast(device: GB28181Device, channelId: string) {
    const channelKey = `${device.device_id}:${channelId}`;
    try {
      await startBroadcast(device.device_id, channelId);
      broadcastingChannels = new Set([...broadcastingChannels, channelKey]);
      showToast('语音广播已启动', 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : '启动广播失败', 'error');
    }
  }

  async function handleStopBroadcast(device: GB28181Device, channelId: string) {
    const channelKey = `${device.device_id}:${channelId}`;
    try {
      await stopBroadcast(device.device_id, channelId);
      broadcastingChannels = new Set([...broadcastingChannels].filter(k => k !== channelKey));
      showToast('语音广播已停止', 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : '停止广播失败', 'error');
    }
  }

  // GB28181 Records functions
  function getDefaultTimeRange() {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    return {
      start: today.toISOString().replace('Z', '').slice(0, 19),
      end: now.toISOString().replace('Z', '').slice(0, 19),
    };
  }

  function getSelectedDeviceChannels() {
    const dev = gb28181Devices.find(d => d.device_id === recordDeviceId);
    return dev?.channels || [];
  }

  async function handleQueryRecords() {
    if (!recordDeviceId || !recordChannelId) {
      showToast('请选择设备和通道', 'error');
      return;
    }
    if (!recordStartTime || !recordEndTime) {
      showToast('请选择时间范围', 'error');
      return;
    }
    recordLoading = true;
    records = [];
    playbackStreamUrl = '';
    try {
      const res = await queryDeviceRecords({
        device_id: recordDeviceId,
        channel_id: recordChannelId,
        start_time: recordStartTime,
        end_time: recordEndTime,
      });
      records = res.records || [];
      if (records.length === 0) {
        showToast('未找到录像记录', 'info');
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : '查询失败', 'error');
    } finally {
      recordLoading = false;
    }
  }

  async function handlePlayRecord(record: DeviceRecordItem) {
    try {
      const res = await startDevicePlayback({
        device_id: recordDeviceId,
        channel_id: recordChannelId,
        start_time: record.start_time,
        end_time: record.end_time,
      });
      if (res.url) {
        const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        playbackStreamUrl = res.url.startsWith('ws') ? res.url : `${wsProto}//${location.host}${res.url}`;
        playbackStreamId = res.stream_id || `${recordDeviceId}_${recordChannelId}_playback`;
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : '播放失败', 'error');
    }
  }

  async function handleSpeedChange(speed: 0.5 | 1 | 2 | 4) {
    if (!recordDeviceId || !recordChannelId) return;
    speedLoading = true;
    try {
      await setPlaybackSpeed({
        device_id: recordDeviceId,
        channel_id: recordChannelId,
        speed,
      });
      playbackSpeed = speed;
      showToast(`倍速已设置为 ${speed}x`, 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : '设置倍速失败', 'error');
    } finally {
      speedLoading = false;
    }
  }

  async function handleSeek(seconds: number) {
    if (!recordDeviceId || !recordChannelId) return;
    seekLoading = true;
    try {
      await seekPlayback({
        device_id: recordDeviceId,
        channel_id: recordChannelId,
        seek_time: seconds,
      });
      playbackSeekTime = seconds;
      showToast(`已拖动到 ${seconds}秒`, 'success');
    } catch (e) {
      showToast(e instanceof Error ? e.message : '拖动失败', 'error');
    } finally {
      seekLoading = false;
    }
  }

  // GB28181 Alarms functions
  async function loadAlarms() {
    alarmsLoading = true;
    try {
      const res = await listGB28181Alarms(undefined, alarmsLimit, alarmsOffset);
      alarms = res.alarms || [];
      alarmsTotal = res.total || 0;
    } catch (e) {
      showToast('加载报警记录失败', 'error');
    } finally {
      alarmsLoading = false;
    }
  }

  function alarmsNextPage() {
    if (alarmsOffset + alarmsLimit < alarmsTotal) {
      alarmsOffset += alarmsLimit;
      loadAlarms();
    }
  }

  function alarmsPrevPage() {
    if (alarmsOffset >= alarmsLimit) {
      alarmsOffset -= alarmsLimit;
      loadAlarms();
    }
  }

  function getPriorityLabel(p: number) { return p >= 5 ? '紧急' : p >= 3 ? '重要' : '一般'; }
  function getPriorityClass(p: number) { return p >= 5 ? 'bg-red-100 text-red-800' : p >= 3 ? 'bg-orange-100 text-orange-800' : 'bg-blue-100 text-blue-800'; }

  function handleGB28181SubTabChange(tab: GB28181SubTab) {
    gb28181SubTab = tab;
    if (tab === 'records' && gb28181Devices.length > 0) {
      const range = getDefaultTimeRange();
      recordStartTime = range.start;
      recordEndTime = range.end;
    }
    if (tab === 'alarms' && alarms.length === 0) {
      loadAlarms();
    }
  }

  function handleONVIFSubTabChange(tab: ONVIFSubTab) {
    onvifSubTab = tab;
  }

  function handleTabChange(tab: DeviceType) {
    activeTab = tab;
    showDiscovery = false;
    loadDevices();
    if (tab === 'onvif') {
      onvifSubTab = 'devices';
    }
    if (tab === 'gb28181') {
      gb28181SubTab = 'devices';
    }
  }

  function handleDiscoveryClick() {
    showDiscovery = !showDiscovery;
    if (showDiscovery && discoveryPanel) {
      discoveryPanel.startDiscovery();
    }
  }

  function handleCameraAdded() {
    loadOnvifCameras();
    showDiscovery = false;
  }

  function getTabIcon(icon: string) {
    switch (icon) {
      case 'camera': return CameraIcon;
      case 'monitor': return Monitor;
      case 'smartphone': return Smartphone;
      case 'radio': return Radio;
      case 'archive': return Archive;
      default: return CameraIcon;
    }
  }

  function formatTime(timeStr: string): string {
    if (!timeStr) return '-';
    try { return new Date(timeStr).toLocaleString('zh-CN'); } catch { return timeStr; }
  }

  // Camera management functions
  function openAddForm() {
    editingCamera = null;
    showForm = true;
  }

  function openEditForm(camera: Camera) {
    editingCamera = camera;
    showForm = true;
  }

  function handleFormSave() {
    showForm = false;
    editingCamera = null;
    loadDevices();
  }

  function handleFormCancel() {
    showForm = false;
    editingCamera = null;
  }

  async function executeConfirmAction() {
    if (!confirmAction) return;
    const { camera, action } = confirmAction;
    confirmAction = null;
    switch (action) {
      case 'stop':
        try {
          await pauseRecording(camera.id);
          showToast(t('cameras.recordingPaused'), 'success');
          await loadDevices();
        } catch (e: any) { showToast(e.message || t('cameras.failedStop'), 'error'); }
        break;
      case 'restart':
        try {
          await stopCamera(camera.id);
          await startCamera(camera.id);
          showToast(t('cameras.cameraUpdated'), 'success');
          await loadDevices();
        } catch (e: any) { showToast(e.message || t('cameras.failedStart'), 'error'); }
        break;
    }
  }

  async function handleStartCamera(camera: Camera) {
    try {
      if (camera.recording_paused) {
        await resumeRecording(camera.id);
        showToast(t('cameras.recordingResumed'), 'success');
      } else {
        await startCamera(camera.id);
        showToast(t('cameras.started'), 'success');
      }
      await loadDevices();
    } catch (e: any) {
      showToast(e.message || t('cameras.failedStart'), 'error');
    }
  }

  async function handleStopCamera(camera: Camera) {
    confirmAction = { camera, action: 'stop' };
  }

  async function handleRestartCamera(camera: Camera) {
    confirmAction = { camera, action: 'restart' };
  }

  async function handleToggleCamera(camera: Camera) {
    await loadDevices();
  }

  async function handleSaveName(camera: Camera, name: string) {
    try {
      await updateCamera(camera.id, { name });
      showToast(t('cameras.nameUpdated'), 'success');
      await loadDevices();
    } catch (e) {
      console.warn('Failed to update camera name:', e);
      showToast(t('cameras.failedUpdate'), 'error');
    }
  }

  async function handlePauseRecording(camera: Camera) {
    try {
      await pauseRecording(camera.id);
      pausedCameras = new Set([...pausedCameras, camera.id]);
      showToast(t('cameras.recordingPaused'), 'success');
      await loadDevices();
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Failed to pause recording', 'error');
    }
  }

  async function handleResumeRecording(camera: Camera) {
    try {
      await resumeRecording(camera.id);
      pausedCameras = new Set([...pausedCameras].filter(id => id !== camera.id));
      showToast(t('cameras.recordingResumed'), 'success');
      await loadDevices();
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Failed to resume recording', 'error');
    }
  }

  async function openArchiveConfirm(camera: Camera) {
    archiveConfirm = camera;
    archiveConfirmCount = 0;
    archiveConfirmSize = 0;
    archiveConfirmStatsLoading = true;
    try {
      const res = await getCameraRecordingStats(camera.id);
      archiveConfirmCount = res.recording_count || 0;
      archiveConfirmSize = res.total_size || 0;
    } catch (e) {
      console.warn('Failed to load archive stats:', e);
    } finally {
      archiveConfirmStatsLoading = false;
    }
  }

  async function loadArchives() {
    try {
      const res = await listArchives();
      archives = res.archives || [];
    } catch (e) {
      console.warn('Failed to load archives:', e);
    }
  }

  async function handleRetentionChange(archiveId: string, days: number) {
    try {
      await setArchiveRetention(archiveId, days);
      showToast(t('cameras.archive.retentionUpdateSuccess'), 'success');
      await loadArchives();
    } catch (e) {
      showToast(t('cameras.failedArchive'), 'error');
    }
  }

  async function handleDeleteArchive(archiveId: string) {
    deleteArchiveLoading = true;
    try {
      await deleteArchiveGroup(archiveId);
      showToast(t('cameras.archive.deleteAllSuccess'), 'success');
      confirmDeleteArchive = null;
      await loadArchives();
    } catch (e) {
      showToast(t('cameras.failedArchive'), 'error');
    } finally {
      deleteArchiveLoading = false;
    }
  }

  async function handleRestoreArchive(archiveId: string) {
    restoreArchiveLoading = true;
    try {
      await restoreArchiveGroup(archiveId);
      showToast(t('cameras.archive.restoreSuccess'), 'success');
      restoreArchiveConfirm = null;
      if (expandedArchiveId === archiveId) {
        expandedArchiveId = null;
        archiveRecordings = [];
        archiveRecordingsTotal = 0;
        archiveRecordingsOffset = 0;
      }
      await Promise.all([loadDevices(), loadArchives()]);
    } catch (e) {
      showToast(t('cameras.archive.restoreFailed'), 'error');
    } finally {
      restoreArchiveLoading = false;
    }
  }

  async function loadArchiveRecordings(cameraId: string) {
    archiveRecordingsLoading = true;
    try {
      const response = await listArchiveRecordings(cameraId, {
        offset: archiveRecordingsOffset,
        limit: archiveRecordingsLimit
      });
      archiveRecordings = response.recordings || [];
      archiveRecordingsTotal = response.total || 0;
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(t('common.error')), 'error');
    } finally {
      archiveRecordingsLoading = false;
    }
  }

  function toggleArchive(group: ArchiveGroup) {
    if (expandedArchiveId === group.id) {
      expandedArchiveId = null;
      archiveRecordings = [];
      archiveRecordingsTotal = 0;
      archiveRecordingsOffset = 0;
    } else {
      expandedArchiveId = group.id;
      archiveRecordingsOffset = 0;
      loadArchiveRecordings(group.id);
    }
  }

  function playRecording(rec: Recording) {
    window.location.hash = `#/recordings/${rec.id}`;
  }

  function downloadRecording(rec: Recording) {
    const url = `/api/archives/${expandedArchiveId}/recordings/${rec.id}/download`;
    const encoded = localStorage.getItem('nvr_auth');
    if (encoded) {
      fetch(url, {
        headers: { 'Authorization': `Basic ${encoded}` }
      })
        .then(res => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return res.blob();
        })
        .then(blob => {
          const objectUrl = URL.createObjectURL(blob);
          const link = document.createElement('a');
          link.href = objectUrl;
          link.download = `archive_${rec.camera_id}_${rec.id}.mp4`;
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
          URL.revokeObjectURL(objectUrl);
        })
        .catch(() => {
          showToast(t('common.error'), 'error');
        });
      return;
    }
    const a = document.createElement('a');
    a.href = url;
    a.download = `archive_${rec.camera_id}_${rec.id}.mp4`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  async function confirmDeleteRecordingFn() {
    if (!deleteRecordingConfirm || !expandedArchiveId) return;
    try {
      await deleteArchiveRecording(expandedArchiveId, deleteRecordingConfirm.id);
      archiveRecordings = archiveRecordings.filter(r => r.id !== deleteRecordingConfirm!.id);
      archiveRecordingsTotal--;
      showToast(t('archives.deleteRecordingSuccess'), 'success');
      deleteRecordingConfirm = null;
      loadArchives();
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(t('common.error')), 'error');
    }
  }

  function openRetDialog(group: ArchiveGroup) {
    selectedArchiveGroup = group;
    retentionDays = group.archive_retention_days;
    showRetDialog = true;
  }

  async function confirmSetRetention() {
    if (!selectedArchiveGroup) return;
    try {
      await setArchiveRetention(selectedArchiveGroup.id, retentionDays);
      archives = archives.map(g =>
        g.id === selectedArchiveGroup!.id ? { ...g, archive_retention_days: retentionDays } : g
      );
      showToast(t('archives.retentionUpdated'), 'success');
      showRetDialog = false;
      selectedArchiveGroup = null;
    } catch (e) {
      showToast(e instanceof Error ? e.message : String(t('common.error')), 'error');
    }
  }

  function formatRetention(days: number): string {
    if (days === 0) return t('archives.keepForever');
    return `${days} ${t('archives.retentionDays')}`;
  }

  let currentArchivePage = $derived(Math.floor(archiveRecordingsOffset / archiveRecordingsLimit) + 1);
  let totalArchivePages = $derived(Math.ceil(archiveRecordingsTotal / archiveRecordingsLimit));

  function handleArchivePageChange(newPage: number) {
    archiveRecordingsOffset = (newPage - 1) * archiveRecordingsLimit;
    if (expandedArchiveId) {
      loadArchiveRecordings(expandedArchiveId);
    }
  }

  async function handlePermanentDelete(cameraId: string) {
    permanentDeleteLoading = true;
    try {
      await permanentlyDeleteCamera(cameraId);
      showToast(t('cameras.deletePermanentSuccess'), 'success');
      permanentDeleteConfirm = null;
      await loadDevices();
    } catch (e) {
      showToast(t('cameras.deletePermanentFailed'), 'error');
    } finally {
      permanentDeleteLoading = false;
    }
  }

  // GB28181 channel management functions
  async function handleGB28181StartRecording(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      await handleStartCamera(camera);
    }
  }

  async function handleGB28181StopRecording(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      await handleStopCamera(camera);
    }
  }

  async function handleGB28181PauseRecording(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      await handlePauseRecording(camera);
    }
  }

  async function handleGB28181ResumeRecording(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      await handleResumeRecording(camera);
    }
  }

  async function handleGB28181Restart(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      await handleRestartCamera(camera);
    }
  }

  function handleGB28181Edit(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      openEditForm(camera);
    }
  }

  function handleGB28181Archive(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      openArchiveConfirm(camera);
    }
  }

  function handleGB28181PermanentDelete(device: GB28181Device, channelId: string) {
    const camera = getGB28181ChannelCamera(device, channelId);
    if (camera) {
      permanentDeleteConfirm = camera;
    }
  }

  onMount(() => {
    loadDevices();
    loadHealth();
    loadArchives();
    
    // Load protocols
    listProtocols().then(list => {
      if (list && list.length > 0) {
        protocols = list;
        protocolsMap = buildProtocolsMap(list);
      }
    }).catch(e => console.warn('Failed to load protocols:', e));

    // Load Xiaomi devices
    xiaomiDevices().then(res => {
      if (res.devices && res.devices.length > 0) {
        xiaomiDeviceList = res.devices;
      }
    }).catch(e => console.warn('Xiaomi not authenticated:', e));

    const healthInterval = window.setInterval(() => loadHealth(), 30000);
    return () => clearInterval(healthInterval);
  });
</script>

<div class="min-h-screen th-bg-primary ">
  <main class="mx-auto px-3 sm:px-4 lg:px-6 py-4 sm:py-6" style="max-width: 1200px;">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold th-text-primary">设备管理</h1>
        <p class="text-sm th-text-secondary mt-1">管理所有类型的视频设备</p>
      </div>
      <div class="flex items-center gap-2">
        {#if canDiscover}
          <button
            onclick={handleDiscoveryClick}
            class="btn btn-primary flex items-center gap-2"
          >
            <Search class="w-4 h-4" />
            {showDiscovery ? '关闭扫描' : '扫描设备'}
          </button>
        {/if}
        <button
          onclick={openAddForm}
          class="btn btn-primary flex items-center gap-2"
        >
          <CameraIcon class="w-4 h-4" />
          添加摄像头
        </button>
        <button
          onclick={loadDevices}
          class="btn btn-secondary flex items-center gap-2"
          disabled={loading}
        >
          <RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
          刷新
        </button>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 p-1 th-bg-secondary rounded-lg mb-6">
      {#each tabs as tab}
        {@const TabIcon = getTabIcon(tab.icon)}
        <button
          onclick={() => handleTabChange(tab.id)}
          class="flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors
            {activeTab === tab.id 
              ? 'bg-white dark:bg-gray-700 shadow-sm th-text-primary' 
              : 'th-text-secondary hover:th-text-primary'}"
        >
          <TabIcon class="w-4 h-4" />
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Discovery Panel -->
    {#if showDiscovery && canDiscover}
      <div class="mb-6">
        <DiscoveryPanel 
          bind:this={discoveryPanel}
          protocol={activeTab === 'onvif' ? 'onvif' : 'xiaomi'}
          cameras={onvifCameras}
          oncameraadded={handleCameraAdded}
        />
      </div>
    {/if}

    <!-- Add/Edit Form Modal -->
    {#if showForm}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="fixed inset-0 z-50 flex items-center justify-center"
        role="presentation"
        onmousedown={(e) => { if (e.target === e.currentTarget) handleFormCancel(); }}
      >
        <!-- Backdrop -->
        <div class="fixed inset-0 bg-black/60 backdrop-blur-sm" aria-hidden="true"></div>
        
        <!-- Modal Content -->
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="relative card border th-border max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto"
          onmousedown={(e) => e.stopPropagation()}
        >
          <CameraForm
            {editingCamera}
            {protocols}
            {protocolsMap}
            {xiaomiDeviceList}
            globalTranscodingEnabled={false}
            h265Available={true}
            onsave={handleFormSave}
            oncancel={handleFormCancel}
          />
        </div>
      </div>
    {/if}

    <!-- Confirmation Dialog (stop/restart) -->
    {#if confirmAction}
      <ConfirmDialog
        title={confirmAction.action === 'stop' ? t('cameras.stopTitle') : t('cameras.restartTitle')}
        message={confirmAction.action === 'stop'
          ? t('cameras.stopMessage', { name: confirmAction.camera.name })
          : t('cameras.restartMessage', { name: confirmAction.camera.name })}
        confirmText={t('common.confirm')}
        onconfirm={executeConfirmAction}
        oncancel={() => confirmAction = null}
        variant="primary"
      />
    {/if}

    <!-- Error display -->
    {#if error}
      <div class="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg">
        <p class="text-red-600">{error}</p>
      </div>
    {/if}

    <!-- ONVIF Devices -->
    {#if activeTab === 'onvif'}
      <!-- ONVIF Sub Tabs -->
      <div class="flex gap-1 p-1 rounded-lg th-bg-tertiary border th-border mb-6">
        {#each onvifSubTabs as subTab}
          {@const Icon = subTab.icon}
          <button
            class="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium rounded-md transition-colors {onvifSubTab === subTab.id ? 'bg-[var(--color-primary)] text-white' : 'th-text-secondary hover:th-text-primary'}"
            onclick={() => handleONVIFSubTabChange(subTab.id)}
          >
            <Icon size={16} />
            {subTab.label}
          </button>
        {/each}
      </div>

      <!-- ONVIF Devices Sub Tab -->
      {#if onvifSubTab === 'devices'}
        {#if onvifLoading}
          <div class="flex items-center justify-center py-12">
            <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
            <span class="ml-2 th-text-secondary">加载中...</span>
          </div>
        {:else if onvifCameras.length === 0}
          <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
            <CameraIcon class="w-12 h-12 th-text-tertiary mb-4" />
            <p class="text-lg th-text-secondary">暂无 ONVIF 设备</p>
            <p class="text-sm th-text-tertiary mt-1">点击上方「扫描设备」按钮发现局域网中的 ONVIF 摄像头</p>
          </div>
        {:else}
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {#each onvifCameras as camera (camera.id)}
              <CameraCard
                {camera}
                {protocolsMap}
                health={healthData[camera.id]}
                onedit={openEditForm}
                ondelete={openArchiveConfirm}
                onpermadelete={(camera) => permanentDeleteConfirm = camera}
                onstart={handleStartCamera}
                onstop={handleStopCamera}
                onrestart={handleRestartCamera}
                ontoggle={handleToggleCamera}
                onsaveName={handleSaveName}
                onpause={handlePauseRecording}
                onresume={handleResumeRecording}
                recordingPaused={camera.recording_paused || pausedCameras.has(camera.id)}
              />
            {/each}
          </div>
        {/if}

      <!-- ONVIF Recordings Sub Tab -->
      {:else if onvifSubTab === 'recordings'}
        <ONVIFRecordingQuery />
      {/if}

    <!-- GB28181 Devices -->
    {:else if activeTab === 'gb28181'}
      <!-- GB28181 Sub Tabs -->
      <div class="flex gap-1 p-1 rounded-lg th-bg-tertiary border th-border mb-6">
        {#each gb28181SubTabs as subTab}
          {@const Icon = subTab.icon}
          <button
            class="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium rounded-md transition-colors {gb28181SubTab === subTab.id ? 'bg-[var(--color-primary)] text-white' : 'th-text-secondary hover:th-text-primary'}"
            onclick={() => handleGB28181SubTabChange(subTab.id)}
          >
            <Icon size={16} />
            {subTab.label}
          </button>
        {/each}
      </div>

      <!-- GB28181 Devices Sub Tab -->
      {#if gb28181SubTab === 'devices'}
        {#if gb28181Loading}
          <div class="flex items-center justify-center py-12">
            <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
            <span class="ml-2 th-text-secondary">加载中...</span>
          </div>
        {:else if gb28181Devices.length === 0}
          <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
            <Monitor class="w-12 h-12 th-text-tertiary mb-4" />
            <p class="text-lg th-text-secondary">暂无 GB28181 设备</p>
            <p class="text-sm th-text-tertiary mt-1">请在设置中启用 GB28181 并配置 SIP 平台 ID，设备将自动注册</p>
          </div>
        {:else}
          <div class="grid gap-4">
            {#each gb28181Devices as device}
              <div class="th-bg-secondary rounded-lg border th-border p-4">
                <div class="flex items-start justify-between">
                  <div class="flex items-center gap-3">
                    <div class="p-2 rounded-lg {device.is_online ? 'bg-green-100' : 'bg-gray-100'}">
                      {#if device.is_online}
                        <Wifi class="w-5 h-5 text-green-600" />
                      {:else}
                        <WifiOff class="w-5 h-5 text-gray-400" />
                      {/if}
                    </div>
                    <div>
                      <h3 class="font-semibold th-text-primary">{device.name || device.device_id}</h3>
                      <p class="text-sm th-text-secondary">
                        {device.is_online ? '在线' : '离线'}
                        {#if device.address}
                          · {device.address}
                        {/if}
                      </p>
                    </div>
                  </div>
                  <span class="px-2 py-1 text-xs rounded-full {device.is_online ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                    {device.is_online ? '在线' : '离线'}
                  </span>
                </div>

                <!-- Device Info -->
                {#if device.manufacturer || device.model || device.firmware || device.last_keepalive_at || device.last_register_at}
                  <div class="mt-3 pt-3 border-t th-border">
                    <div class="grid grid-cols-2 sm:grid-cols-3 gap-2 text-xs">
                      {#if device.manufacturer}
                        <div>
                          <span class="th-text-tertiary">制造商:</span>
                          <span class="th-text-secondary ml-1">{device.manufacturer}</span>
                        </div>
                      {/if}
                      {#if device.model}
                        <div>
                          <span class="th-text-tertiary">型号:</span>
                          <span class="th-text-secondary ml-1">{device.model}</span>
                        </div>
                      {/if}
                      {#if device.firmware}
                        <div>
                          <span class="th-text-tertiary">固件:</span>
                          <span class="th-text-secondary ml-1">{device.firmware}</span>
                        </div>
                      {/if}
                      {#if device.last_register_at}
                        <div>
                          <span class="th-text-tertiary">注册时间:</span>
                          <span class="th-text-secondary ml-1">{formatTime(device.last_register_at)}</span>
                        </div>
                      {/if}
                      {#if device.last_keepalive_at}
                        <div>
                          <span class="th-text-tertiary">心跳时间:</span>
                          <span class="th-text-secondary ml-1">{formatTime(device.last_keepalive_at)}</span>
                        </div>
                      {/if}
                    </div>
                  </div>
                {/if}

                {#if device.channels && device.channels.length > 0}
                  <div class="mt-4 border-t th-border pt-4">
                    <h4 class="text-sm font-medium th-text-secondary mb-3">通道列表 ({device.channels.length})</h4>
                    <div class="grid gap-3">
                      {#each device.channels as channel}
                        {@const streamKey = `${device.device_id}:${channel.channel_id}`}
                        {@const channelCamera = getGB28181ChannelCamera(device, channel.channel_id)}
                        {@const isChannelPlaying = playingStreams.has(streamKey) || (channelCamera && channelCamera.status === 'recording')}
                        {@const isChannelPaused = channelCamera && (channelCamera.recording_paused || pausedCameras.has(channelCamera.id))}
                        
                        <div class="bg-white dark:bg-gray-800 rounded-lg border th-border overflow-hidden">
                          <!-- Channel Header -->
                          <div class="flex items-center justify-between p-3">
                            <div class="flex items-center gap-2">
                              <Video class="w-4 h-4 th-text-tertiary" />
                              <span class="text-sm font-medium th-text-primary">{channel.name || channel.channel_id}</span>
                              {#if channelCamera}
                        <span class="px-1.5 py-0.5 text-xs rounded {isChannelPaused ? 'bg-yellow-100 text-yellow-800' : channelCamera.status === 'recording' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                          {isChannelPaused ? '已暂停' : channelCamera.status === 'recording' ? '录制中' : '已停止'}
                        </span>
                      {/if}
                            </div>
                            
                            <div class="flex items-center gap-1">
                              <!-- Broadcast button -->
                              {#if broadcastingChannels.has(streamKey)}
                                <button
                                  onclick={() => handleStopBroadcast(device, channel.channel_id)}
                                  class="btn btn-ghost px-2 py-1 text-sm text-orange-500"
                                  title="停止广播"
                                >
                                  <MicOff size={14} />
                                </button>
                              {:else}
                                <button
                                  onclick={() => handleStartBroadcast(device, channel.channel_id)}
                                  class="btn btn-ghost px-2 py-1 text-sm"
                                  disabled={!device.is_online}
                                  title="语音广播"
                                >
                                  <Mic size={14} />
                                </button>
                              {/if}

                              <!-- Recording control -->
                              {#if channelCamera}
                                {#if isChannelPaused}
                                  <button
                                    onclick={() => handleGB28181ResumeRecording(device, channel.channel_id)}
                                    class="btn btn-ghost px-2 py-1 text-sm"
                                    title="恢复录制"
                                  >
                                    <Play size={14} />
                                  </button>
                                {:else if channelCamera.status === 'recording'}
                                  <button
                                    onclick={() => handleGB28181PauseRecording(device, channel.channel_id)}
                                    class="btn btn-ghost px-2 py-1 text-sm"
                                    title="暂停录制"
                                  >
                                    <Pause size={14} />
                                  </button>
                                  <button
                                    onclick={() => handleGB28181StopRecording(device, channel.channel_id)}
                                    class="btn btn-ghost px-2 py-1 text-sm"
                                    title="停止录制"
                                  >
                                    <Square size={14} />
                                  </button>
                                {:else}
                                  <button
                                    onclick={() => handleGB28181StartRecording(device, channel.channel_id)}
                                    class="btn btn-ghost px-2 py-1 text-sm"
                                    disabled={!device.is_online}
                                    title="开始录制"
                                  >
                                    <Play size={14} />
                                  </button>
                                {/if}
                                
                                <button
                                  onclick={() => handleGB28181Restart(device, channel.channel_id)}
                                  class="btn btn-ghost px-2 py-1 text-sm"
                                  title="重启"
                                >
                                  <RotateCw size={14} />
                                </button>
                              {:else}
                                <!-- Play/Stop streaming -->
                                {#if isChannelPlaying}
                                  <button
                                    onclick={() => handleGB28181Stop(device, channel.channel_id)}
                                    class="btn btn-ghost px-2 py-1 text-sm text-red-500"
                                    title="停止推流"
                                  >
                                    <Square size={14} />
                                  </button>
                                {:else}
                                  <button
                                    onclick={() => handleGB28181Play(device, channel.channel_id)}
                                    class="btn btn-ghost px-2 py-1 text-sm"
                                    disabled={!device.is_online}
                                    title="开始推流"
                                  >
                                    <Play size={14} />
                                  </button>
                                {/if}
                              {/if}

                              <!-- Live view -->
                              {#if isChannelPlaying || (channelCamera && channelCamera.status === 'recording')}
                                <a 
                                  href="#/live/{channelCamera?.id || `${device.device_id}:${channel.channel_id}`}" 
                                  class="btn btn-ghost px-2 py-1 text-sm"
                                  title="实时预览"
                                >
                                  <Eye size={14} />
                                </a>
                              {/if}

                              <!-- More actions for channels with camera -->
                              {#if channelCamera}
                                <button
                                  onclick={() => handleGB28181Edit(device, channel.channel_id)}
                                  class="btn btn-ghost px-2 py-1 text-sm"
                                  title="编辑"
                                >
                                  <Pencil size={14} />
                                </button>
                                <button
                                  onclick={() => handleGB28181Archive(device, channel.channel_id)}
                                  class="btn btn-ghost px-2 py-1 text-sm text-orange-500"
                                  title="归档"
                                >
                                  <Archive size={14} />
                                </button>
                                <button
                                  onclick={() => handleGB28181PermanentDelete(device, channel.channel_id)}
                                  class="btn btn-ghost px-2 py-1 text-sm text-red-500"
                                  title="永久删除"
                                >
                                  <Trash2 size={14} />
                                </button>
                              {/if}
                            </div>
                          </div>

                          <!-- Snapshot Preview for channels with camera -->
                          {#if channelCamera && channelCamera.enabled}
                            <div class="aspect-video bg-gray-100 max-h-48 overflow-hidden">
                              <img
                                src="{getSnapshotUrl(channelCamera.id)}&_t={Date.now()}"
                                alt={channelCamera.name}
                                class="w-full h-full object-cover"
                                loading="lazy"
                                onerror={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                              />
                            </div>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}

      <!-- GB28181 Records Sub Tab -->
      {:else if gb28181SubTab === 'records'}
        <div class="space-y-4">
          <!-- Query form -->
          <div class="card border th-border p-4">
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
              <div>
                <label for="rec-device" class="input-label">设备</label>
                <select id="rec-device" class="input mt-1 w-full" bind:value={recordDeviceId} onchange={() => { recordChannelId = ''; records = []; playbackStreamUrl = ''; }}>
                  <option value="">选择设备</option>
                  {#each gb28181Devices as dev}
                    <option value={dev.device_id}>{dev.name || dev.device_id}</option>
                  {/each}
                </select>
              </div>
              <div>
                <label for="rec-channel" class="input-label">通道</label>
                <select id="rec-channel" class="input mt-1 w-full" bind:value={recordChannelId} disabled={!recordDeviceId}>
                  <option value="">选择通道</option>
                  {#each getSelectedDeviceChannels() as ch}
                    <option value={ch.channel_id}>{ch.name || ch.channel_id}</option>
                  {/each}
                </select>
              </div>
              <div>
                <label for="rec-start" class="input-label">开始时间</label>
                <input id="rec-start" type="datetime-local" class="input mt-1 w-full" bind:value={recordStartTime} step="1" />
              </div>
              <div>
                <label for="rec-end" class="input-label">结束时间</label>
                <input id="rec-end" type="datetime-local" class="input mt-1 w-full" bind:value={recordEndTime} step="1" />
              </div>
              <div class="flex items-end">
                <button class="btn btn-primary w-full" onclick={handleQueryRecords} disabled={recordLoading}>
                  {#if recordLoading}
                    <RefreshCw class="w-4 h-4 animate-spin mr-1" /> 查询中...
                  {:else}
                    <Search class="w-4 h-4 mr-1" /> 查询录像
                  {/if}
                </button>
              </div>
            </div>
          </div>

          <!-- Playback player -->
          {#if playbackStreamUrl}
            <div class="card border th-border p-4">
              <div class="flex items-center justify-between mb-3">
                <h3 class="text-sm font-semibold th-text-primary">录像回放</h3>
                <button class="btn btn-ghost btn-sm" onclick={() => { playbackStreamUrl = ''; }}>
                  <X size={14} /> 关闭
                </button>
              </div>
              <div class="aspect-video bg-black rounded-lg overflow-hidden max-w-3xl">
                <FlvPlayer
                  cameraId={playbackStreamId}
                  cameraName="录像回放"
                  streamUrl={playbackStreamUrl}
                  protocol="ws-flv"
                  expanded={false}
                  tabVisible={true}
                />
              </div>
              <!-- Playback controls -->
              <div class="mt-4 space-y-3">
                <!-- Speed control -->
                <div class="flex items-center gap-3">
                  <span class="text-sm th-text-secondary flex items-center gap-1">
                    <FastForward size={14} /> 倍速:
                  </span>
                  <div class="flex gap-1">
                    {#each [0.5, 1, 2, 4] as speed}
                      <button
                        class="btn btn-sm {playbackSpeed === speed ? 'btn-primary' : 'btn-ghost'}"
                        onclick={() => handleSpeedChange(speed as 0.5 | 1 | 2 | 4)}
                        disabled={speedLoading}
                      >
                        {speed}x
                      </button>
                    {/each}
                  </div>
                </div>
                <!-- Seek control -->
                <div class="flex items-center gap-3">
                  <span class="text-sm th-text-secondary flex items-center gap-1">
                    <Rewind size={14} /> 拖动:
                  </span>
                  <div class="flex gap-1">
                    {#each [0, 30, 60, 300, 600] as seconds}
                      <button
                        class="btn btn-sm btn-ghost"
                        onclick={() => handleSeek(seconds)}
                        disabled={seekLoading}
                      >
                        {seconds === 0 ? '起点' : seconds < 60 ? `${seconds}秒` : `${seconds / 60}分`}
                      </button>
                    {/each}
                  </div>
                </div>
              </div>
            </div>
          {/if}

          <!-- Records list -->
          {#if records.length > 0}
            <div class="card border th-border overflow-hidden">
              <div class="px-4 py-3 border-b th-border">
                <span class="text-sm th-text-secondary">共 {records.length} 条录像</span>
              </div>
              <div class="overflow-x-auto max-h-96 overflow-y-auto">
                <table class="w-full">
                  <thead class="sticky top-0 bg-white dark:bg-gray-900">
                    <tr class="border-b th-border">
                      <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">文件名</th>
                      <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">开始时间</th>
                      <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">结束时间</th>
                      <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">类型</th>
                      <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each records as rec}
                      <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50">
                        <td class="px-4 py-3 text-sm font-mono th-text-primary max-w-xs truncate">{rec.name || rec.path || '-'}</td>
                        <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(rec.start_time)}</td>
                        <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(rec.end_time)}</td>
                        <td class="px-4 py-3 text-sm th-text-secondary">{rec.type || '-'}</td>
                        <td class="px-4 py-3 text-sm">
                          <button class="btn btn-sm btn-primary" onclick={() => handlePlayRecord(rec)}>
                            <Play size={14} class="mr-1" /> 播放
                          </button>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>
          {:else if !recordLoading && recordDeviceId && recordChannelId && records.length === 0}
            <div class="flex flex-col items-center justify-center py-8 th-bg-secondary rounded-lg">
              <Film class="w-10 h-10 th-text-tertiary mb-3" />
              <p class="text-sm th-text-secondary">点击"查询录像"查看设备录像记录</p>
            </div>
          {/if}
        </div>

      <!-- GB28181 Alarms Sub Tab -->
      {:else if gb28181SubTab === 'alarms'}
        {#if alarmsLoading && alarms.length === 0}
          <div class="flex justify-center py-12"><RefreshCw class="w-6 h-6 animate-spin th-text-secondary" /></div>
        {:else if alarms.length === 0}
          <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
            <AlertTriangle class="w-12 h-12 th-text-tertiary mb-4" />
            <p class="text-lg th-text-secondary">暂无报警记录</p>
          </div>
        {:else}
          <div class="th-bg-secondary rounded-lg border th-border overflow-hidden">
            <div class="overflow-x-auto">
              <table class="w-full">
                <thead>
                  <tr class="border-b th-border bg-gray-50 dark:bg-gray-800">
                    <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">设备 ID</th>
                    <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">通道 ID</th>
                    <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">报警类型</th>
                    <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">优先级</th>
                    <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">报警时间</th>
                    <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">描述</th>
                  </tr>
                </thead>
                <tbody>
                  {#each alarms as alarm}
                    <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50">
                      <td class="px-4 py-3 text-sm font-mono th-text-primary">{alarm.device_id}</td>
                      <td class="px-4 py-3 text-sm font-mono th-text-primary">{alarm.channel_id || '-'}</td>
                      <td class="px-4 py-3 text-sm">
                        <div class="flex items-center gap-1">
                          <AlertTriangle class="w-4 h-4 text-orange-500" />
                          <span class="th-text-primary">{alarm.alarm_type || '未知'}</span>
                        </div>
                      </td>
                      <td class="px-4 py-3 text-sm">
                        <span class="px-2 py-1 text-xs rounded-full {getPriorityClass(alarm.priority)}">{getPriorityLabel(alarm.priority)}</span>
                      </td>
                      <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(alarm.alarm_time)}</td>
                      <td class="px-4 py-3 text-sm th-text-secondary max-w-xs truncate">{alarm.description || '-'}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
            {#if alarmsTotal > alarmsLimit}
              <div class="flex items-center justify-between px-4 py-3 border-t th-border">
                <span class="text-sm th-text-secondary">共 {alarmsTotal} 条</span>
                <div class="flex gap-2">
                  <button onclick={alarmsPrevPage} class="btn btn-sm btn-secondary" disabled={alarmsOffset === 0}>上一页</button>
                  <span class="text-sm th-text-secondary px-2 py-1">{Math.floor(alarmsOffset / alarmsLimit) + 1} / {Math.ceil(alarmsTotal / alarmsLimit)}</span>
                  <button onclick={alarmsNextPage} class="btn btn-sm btn-secondary" disabled={alarmsOffset + alarmsLimit >= alarmsTotal}>下一页</button>
                </div>
              </div>
            {/if}
          </div>
        {/if}
      {/if}

    <!-- Xiaomi Devices -->
    {:else if activeTab === 'xiaomi'}
      {#if xiaomiCamerasLoading}
        <div class="flex items-center justify-center py-12">
          <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
          <span class="ml-2 th-text-secondary">加载中...</span>
        </div>
      {:else if xiaomiCameras.length === 0}
        <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
          <Smartphone class="w-12 h-12 th-text-tertiary mb-4" />
          <p class="text-lg th-text-secondary">暂无小米设备</p>
          <p class="text-sm th-text-tertiary mt-1">点击上方「扫描设备」按钮登录小米账号并发现设备</p>
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {#each xiaomiCameras as camera (camera.id)}
            <CameraCard
              {camera}
              {protocolsMap}
              health={healthData[camera.id]}
              onedit={openEditForm}
              ondelete={openArchiveConfirm}
              onpermadelete={(camera) => permanentDeleteConfirm = camera}
              onstart={handleStartCamera}
              onstop={handleStopCamera}
              onrestart={handleRestartCamera}
              ontoggle={handleToggleCamera}
              onsaveName={handleSaveName}
              onpause={handlePauseRecording}
              onresume={handleResumeRecording}
              recordingPaused={camera.recording_paused || pausedCameras.has(camera.id)}
            />
          {/each}
        </div>
      {/if}

    <!-- Push Streams -->
    {:else if activeTab === 'push'}
      {#if pushLoading}
        <div class="flex items-center justify-center py-12">
          <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
          <span class="ml-2 th-text-secondary">加载中...</span>
        </div>
      {:else if pushStreams.length === 0 && pushCameras.length === 0}
        <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
          <Radio class="w-12 h-12 th-text-tertiary mb-4" />
          <p class="text-lg th-text-secondary">暂无推流</p>
          <p class="text-sm th-text-tertiary mt-1">使用 RTMP、SRT 或 WHIP 推流后将显示在这里，可在流详情页升级为摄像头</p>
        </div>
      {:else}
        <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {#each pushCameras as camera}
            <div class="th-bg-secondary rounded-lg border th-border p-4">
              <div class="flex items-center justify-between mb-3">
                <div class="flex items-center gap-3">
                  <div class="p-2 rounded-lg {camera.enabled ? 'bg-purple-100' : 'bg-gray-100'}">
                    <Radio class="w-5 h-5 {camera.enabled ? 'text-purple-600' : 'text-gray-400'}" />
                  </div>
                  <div>
                    <h3 class="font-semibold th-text-primary">{camera.name}</h3>
                    <p class="text-sm th-text-secondary">
                      {pushSourceLabel(camera.source_type)} · 已升级为摄像头
                      {#if camera.encoding}
                        · {camera.encoding}
                      {/if}
                    </p>
                  </div>
                </div>
                <span class="px-2 py-1 text-xs rounded-full {camera.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                  {camera.enabled ? '启用' : '禁用'}
                </span>
              </div>
              
              <!-- Snapshot Preview -->
              {#if camera.enabled}
                <div class="mb-3 rounded-lg overflow-hidden aspect-video bg-gray-100">
                  <img
                    src="{getSnapshotUrl(camera.id)}&_t={Date.now()}"
                    alt={camera.name}
                    class="w-full h-full object-cover"
                    loading="lazy"
                    onerror={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                  />
                </div>
              {/if}

              <!-- Status Badge -->
              <div class="mb-3">
                {#if camera.recording_paused || pausedCameras.has(camera.id)}
                  <span class="px-2 py-1 text-xs rounded-full bg-yellow-100 text-yellow-800">已暂停</span>
                {:else if camera.status === 'recording'}
                  <span class="px-2 py-1 text-xs rounded-full bg-green-100 text-green-800">录制中</span>
                {:else if camera.status === 'error'}
                  <span class="px-2 py-1 text-xs rounded-full bg-red-100 text-red-800">错误</span>
                {:else}
                  <span class="px-2 py-1 text-xs rounded-full bg-gray-100 text-gray-600">已停止</span>
                {/if}
              </div>

              <!-- Action Buttons -->
              <div class="flex items-center justify-between pt-3 border-t th-border">
                <div class="flex items-center gap-2">
                  {#if camera.recording_paused || pausedCameras.has(camera.id)}
                    <!-- Paused state: show resume and stop -->
                    <button
                      class="btn btn-ghost px-2 py-1 text-sm"
                      onclick={() => handleResumeRecording(camera)}
                      title="恢复录制"
                    >
                      <Play size={14} />
                    </button>
                    <button
                      class="btn btn-ghost px-2 py-1 text-sm"
                      onclick={() => handleStopCamera(camera)}
                      title="停止录制"
                    >
                      <Square size={14} />
                    </button>
                  {:else if camera.status === 'recording'}
                    <!-- Recording state: show pause and stop -->
                    <button
                      class="btn btn-ghost px-2 py-1 text-sm"
                      onclick={() => handlePauseRecording(camera)}
                      title="暂停录制"
                    >
                      <Pause size={14} />
                    </button>
                    <button
                      class="btn btn-ghost px-2 py-1 text-sm"
                      onclick={() => handleStopCamera(camera)}
                      title="停止录制"
                    >
                      <Square size={14} />
                    </button>
                  {:else}
                    <!-- Stopped state: show start -->
                    <button
                      class="btn btn-ghost px-2 py-1 text-sm"
                      onclick={() => handleStartCamera(camera)}
                      title="开始录制"
                    >
                      <Play size={14} />
                    </button>
                  {/if}
                  
                  <button
                    class="btn btn-ghost px-2 py-1 text-sm"
                    onclick={() => openEditForm(camera)}
                    title="编辑"
                  >
                    <Pencil size={14} />
                  </button>
                </div>

                <div class="flex items-center gap-2">
                  <a href="#/live/{camera.id}" class="btn btn-ghost px-2 py-1 text-sm" title="实时预览">
                    <Eye size={14} />
                  </a>
                  <button
                    class="btn btn-ghost px-2 py-1 text-sm"
                    onclick={() => handleRestartCamera(camera)}
                    title="重启"
                  >
                    <RotateCw size={14} />
                  </button>
                  <button
                    class="btn btn-ghost px-2 py-1 text-sm text-red-500"
                    onclick={() => openArchiveConfirm(camera)}
                    title="归档"
                  >
                    <Archive size={14} />
                  </button>
                </div>
              </div>
            </div>
          {/each}
          
          {#each pushStreams as stream}
            <div class="th-bg-secondary rounded-lg border th-border p-4">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div class="p-2 rounded-lg bg-purple-100">
                    <Radio class="w-5 h-5 text-purple-600" />
                  </div>
                  <div>
                    <h3 class="font-semibold th-text-primary">{stream.stream_id}</h3>
                    <p class="text-sm th-text-secondary">
                      {stream.engine}
                      {#if stream.video_codec}
                        · {stream.video_codec}
                      {/if}
                      {#if stream.source_type}
                        · {pushSourceLabel(stream.source_type)}
                      {/if}
                    </p>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <span class="px-2 py-1 text-xs rounded-full {stream.active ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                    {stream.active ? '活跃' : '空闲'}
                  </span>
                  <a href="#/streams/{encodeURIComponent(stream.stream_id)}" class="btn btn-sm btn-secondary">
                    详情
                  </a>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}

    <!-- Archives -->
    {:else if activeTab === 'archives'}
      {#if archives.length === 0}
        <div class="card border th-border p-12 text-center mt-6">
          <div class="flex justify-center mb-4 th-text-muted">
            <Archive size={48} />
          </div>
          <h3 class="text-lg font-medium th-text-primary mb-2">{t('cameras.archive.noArchives')}</h3>
          <p class="text-sm th-text-muted mb-4">{t('cameras.archive.noArchivesHint')}</p>
        </div>
      {:else}
        <div class="space-y-3 mt-6">
          {#each archives as group (group.id)}
            <div class="card border th-border overflow-hidden">
              <!-- Group header -->
              <div
                class="w-full p-5 text-left hover:th-bg-hover transition-colors duration-200 cursor-pointer"
                onclick={() => toggleArchive(group)}
                role="button"
                tabindex="0"
                onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleArchive(group); } }}
              >
                <div class="flex items-center justify-between gap-4">
                  <div class="flex items-center gap-3 min-w-0">
                    {#if expandedArchiveId === group.id}
                      <ChevronDown size={20} class="th-text-secondary shrink-0" />
                    {:else}
                      <ChevronRight size={20} class="th-text-secondary shrink-0" />
                    {/if}
                    <div class="min-w-0">
                      <h3 class="font-semibold th-text-primary truncate">{group.name}</h3>
                      <div class="flex flex-wrap gap-x-5 gap-y-1 mt-1.5 text-sm th-text-secondary">
                        <span class="flex items-center gap-1.5">
                          <Video size={14} />
                          {group.recording_count} {t('archives.recordings')}
                        </span>
                        <span class="flex items-center gap-1.5">
                          <Archive size={14} />
                          {formatFileSize(group.total_size)}
                        </span>
                        <span class="flex items-center gap-1.5">
                          <Clock size={14} />
                          {t('archives.archivedAt')}: {formatDate(group.archived_at)}
                        </span>
                        <span class="flex items-center gap-1.5">
                          <Settings size={14} />
                          {formatRetention(group.archive_retention_days)}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div class="flex items-center gap-2 shrink-0" role="group" aria-label={t('archives.actions')}>
                    <button
                      class="btn btn-ghost btn-sm"
                      onclick={(e) => { e.stopPropagation(); restoreArchiveConfirm = group; }}
                      title={t('cameras.archive.restore')}
                    >
                      <RotateCw size={16} />
                    </button>
                    <button
                      class="btn btn-ghost btn-sm"
                      onclick={(e) => { e.stopPropagation(); openRetDialog(group); }}
                      title={t('archives.setRetention')}
                    >
                      <Clock size={16} />
                    </button>
                    <button
                      class="btn btn-ghost btn-sm th-color-danger"
                      onclick={(e) => { e.stopPropagation(); confirmDeleteArchive = group.id; }}
                      title={t('archives.deleteGroup')}
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                </div>
              </div>

              <!-- Expanded recordings -->
              {#if expandedArchiveId === group.id}
                <div class="border-t th-border">
                  {#if archiveRecordingsLoading}
                    <div class="p-6 space-y-3">
                      {#each Array(3) as _}
                        <div class="flex gap-4 items-center">
                          <div class="h-4 w-32 th-bg-tertiary rounded animate-pulse"></div>
                          <div class="h-4 w-16 th-bg-tertiary rounded animate-pulse"></div>
                          <div class="h-4 w-16 th-bg-tertiary rounded animate-pulse"></div>
                          <div class="h-4 w-20 th-bg-tertiary rounded animate-pulse ml-auto"></div>
                        </div>
                      {/each}
                    </div>
                  {:else if archiveRecordings.length === 0}
                    <div class="p-6 text-center th-text-muted text-sm">
                      {t('archives.noArchives')}
                    </div>
                  {:else}
                    <div class="table-container">
                      <table class="table">
                        <thead>
                          <tr>
                            <th>{t('archives.camera')}</th>
                            <th>{t('archives.date')}</th>
                            <th>{t('archives.duration')}</th>
                            <th>{t('archives.size')}</th>
                            <th class="text-right">{t('archives.actions')}</th>
                          </tr>
                        </thead>
                        <tbody>
                          {#each archiveRecordings as rec (rec.id)}
                            <tr class="transition-all duration-200 hover:th-bg-hover">
                              <td>
                                <span class="font-mono text-xs th-text-tertiary">{rec.camera_id}</span>
                              </td>
                              <td class="whitespace-nowrap">{formatDate(rec.started_at)}</td>
                              <td class="font-mono text-sm">{formatDuration(rec.duration)}</td>
                              <td>{formatFileSize(rec.file_size)}</td>
                              <td class="text-right">
                                <div class="flex justify-end gap-1">
                                  <button
                                    class="btn btn-ghost px-2 py-1.5 text-sm"
                                    onclick={() => playRecording(rec)}
                                    title={t('archives.play')}
                                  >
                                    <Play size={16} />
                                  </button>
                                  <button
                                    class="btn btn-ghost px-2 py-1.5 text-sm"
                                    onclick={() => downloadRecording(rec)}
                                    title={t('archives.download')}
                                  >
                                    <Download size={16} />
                                  </button>
                                  <button
                                    class="btn btn-ghost px-2 py-1.5 text-sm th-color-danger"
                                    onclick={() => deleteRecordingConfirm = rec}
                                    title={t('archives.delete')}
                                  >
                                    <Trash2 size={16} />
                                  </button>
                                </div>
                              </td>
                            </tr>
                          {/each}
                        </tbody>
                      </table>
                    </div>

                    {#if totalArchivePages > 1}
                      <div class="px-4 py-2 border-t th-border">
                        <span class="text-sm th-text-muted">
                          {t('recordings.showing', {
                            start: String(archiveRecordingsOffset + 1),
                            end: String(Math.min(archiveRecordingsOffset + archiveRecordings.length, archiveRecordingsTotal)),
                            total: String(archiveRecordingsTotal)
                          })}
                        </span>
                      </div>
                      <Pagination
                        currentPage={currentArchivePage}
                        totalPages={totalArchivePages}
                        onPageChange={handleArchivePageChange}
                      />
                    {/if}
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    {/if}
  </main>

  <!-- Archive Confirm Dialog -->
  {#if archiveConfirm}
    <ArchiveConfirmDialog
      cameraName={archiveConfirm.name}
      recordingCount={archiveConfirmCount}
      totalSize={archiveConfirmStatsLoading ? '...' : formatFileSize(archiveConfirmSize)}
      loading={archiveLoading}
      onconfirm={async () => {
        archiveLoading = true;
        try {
          await deleteCamera(archiveConfirm!.id);
          showToast(t('cameras.cameraArchived'), 'success');
          archiveConfirm = null;
          await Promise.all([loadDevices(), loadArchives()]);
        } catch (e) {
          showToast(t('cameras.failedArchive'), 'error');
        } finally {
          archiveLoading = false;
        }
      }}
      oncancel={() => { if (!archiveLoading) archiveConfirm = null; }}
    />
  {/if}

  <!-- Archive Delete Confirm Dialog -->
  {#if confirmDeleteArchive}
    <ConfirmDialog
      title={t('cameras.action.deleteAll')}
      message={t('cameras.archive.deleteAllConfirm')}
      onconfirm={() => handleDeleteArchive(confirmDeleteArchive!)}
      oncancel={() => { if (!deleteArchiveLoading) confirmDeleteArchive = null; }}
      variant="danger"
      loading={deleteArchiveLoading}
    />
  {/if}

  {#if restoreArchiveConfirm}
    <ConfirmDialog
      title={t('cameras.archive.restore')}
      message={t('cameras.archive.restoreConfirm', { name: restoreArchiveConfirm.name })}
      confirmText={t('cameras.archive.restore')}
      onconfirm={() => handleRestoreArchive(restoreArchiveConfirm!.id)}
      oncancel={() => { if (!restoreArchiveLoading) restoreArchiveConfirm = null; }}
      variant="primary"
      loading={restoreArchiveLoading}
    />
  {/if}

  {#if permanentDeleteConfirm}
    <ConfirmDialog
      title={t('cameras.action.deletePermanent')}
      message={t('cameras.deletePermanentConfirm', { name: permanentDeleteConfirm.name })}
      confirmText={t('cameras.action.deletePermanent')}
      onconfirm={() => handlePermanentDelete(permanentDeleteConfirm!.id)}
      oncancel={() => { if (!permanentDeleteLoading) permanentDeleteConfirm = null; }}
      variant="danger"
      loading={permanentDeleteLoading}
    />
  {/if}

  <!-- Retention dialog -->
  {#if showRetDialog && selectedArchiveGroup}
    <div class="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50" role="dialog" aria-modal="true">
      <div class="card max-w-md w-full p-6">
        <h3 class="text-lg font-semibold th-text-primary mb-4">{t('archives.setRetention')}</h3>
        <p class="th-text-secondary mb-4">{selectedArchiveGroup.name}</p>
        <div class="mb-6">
          <label for="retention-select" class="input-label">{t('archives.retention')}</label>
          <select id="retention-select" class="input mt-1" bind:value={retentionDays}>
            <option value={0}>{t('archives.keepForever')}</option>
            <option value={7}>7 {t('archives.retentionDays')}</option>
            <option value={14}>14 {t('archives.retentionDays')}</option>
            <option value={30}>30 {t('archives.retentionDays')}</option>
            <option value={60}>60 {t('archives.retentionDays')}</option>
            <option value={90}>90 {t('archives.retentionDays')}</option>
            <option value={180}>180 {t('archives.retentionDays')}</option>
            <option value={365}>365 {t('archives.retentionDays')}</option>
          </select>
        </div>
        <div class="flex gap-3 justify-end">
          <button onclick={() => { showRetDialog = false; selectedArchiveGroup = null; }} class="btn btn-secondary">
            {t('recordings.cancel')}
          </button>
          <button onclick={confirmSetRetention} class="btn btn-primary">
            {t('archives.setRetention')}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Delete recording dialog -->
  {#if deleteRecordingConfirm}
    <div class="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50" role="dialog" aria-modal="true">
      <div class="card max-w-md w-full p-6">
        <h3 class="text-lg font-semibold th-text-primary mb-4">{t('archives.delete')}</h3>
        <p class="th-text-secondary mb-6">
          {t('archives.confirmDeleteRecording')}
        </p>
        <div class="flex gap-3 justify-end">
          <button onclick={() => { deleteRecordingConfirm = null; }} class="btn btn-secondary">
            {t('recordings.cancel')}
          </button>
          <button onclick={confirmDeleteRecordingFn} class="btn btn-danger">
            {t('recordings.deleteConfirm')}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
