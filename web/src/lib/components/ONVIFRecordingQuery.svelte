<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    getONVIFRecordings, 
    searchONVIFRecordings, 
    getONVIFReplayURI,
    type ONVIFRecording,
    type ONVIFRecordingSegment 
  } from '$lib/api/onvif-recording';
  import { listCameras, getSnapshotUrl } from '$lib/api';
  import type { Camera } from '$lib/api';
  import { showToast } from '$lib/toast';
  import { formatFileSize, formatDate, formatDuration } from '$lib/format';
  import { 
    Film, Search, Play, RefreshCw, Download, 
    Calendar, Clock, HardDrive, X 
  } from 'lucide-svelte';
  import FlvPlayer from '../../components/FlvPlayer.svelte';

  let cameras = $state<Camera[]>([]);
  let selectedCameraId = $state('');
  let startTime = $state('');
  let endTime = $state('');
  let loading = $state(false);
  let recordings = $state<ONVIFRecording[]>([]);
  let segments = $state<ONVIFRecordingSegment[]>([]);
  let searchMode = $state<'recordings' | 'segments'>('segments');
  
  // Playback state
  let playbackStreamUrl = $state('');
  let playbackStreamId = $state('');
  let playbackLoading = $state(false);

  function getDefaultTimeRange() {
    const now = new Date();
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000);
    return {
      start: yesterday.toISOString().replace('Z', '').slice(0, 19),
      end: now.toISOString().replace('Z', '').slice(0, 19),
    };
  }

  async function loadCameras() {
    try {
      const all = await listCameras();
      cameras = (all || []).filter(c => c.protocol === 'onvif' || c.protocol.startsWith('onvif'));
    } catch (e) {
      console.error('Failed to load cameras:', e);
    }
  }

  async function handleSearch() {
    if (!selectedCameraId) {
      showToast('请选择设备', 'error');
      return;
    }
    if (!startTime) {
      showToast('请选择开始时间', 'error');
      return;
    }

    loading = true;
    recordings = [];
    segments = [];
    playbackStreamUrl = '';

    try {
      if (searchMode === 'recordings') {
        const res = await getONVIFRecordings(selectedCameraId, {
          start_time: startTime,
          end_time: endTime || undefined,
        });
        recordings = res.recordings || [];
        if (recordings.length === 0) {
          showToast('未找到录像记录', 'info');
        }
      } else {
        const res = await searchONVIFRecordings(selectedCameraId, {
          start_time: startTime,
          end_time: endTime || undefined,
          max_results: 100,
        });
        segments = res.segments || [];
        if (segments.length === 0) {
          showToast('未找到录像片段', 'info');
        }
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : '查询失败', 'error');
    } finally {
      loading = false;
    }
  }

  async function handlePlayRecording(recording: ONVIFRecording) {
    playbackLoading = true;
    try {
      const res = await getONVIFReplayURI(selectedCameraId, recording.token);
      if (res.uri) {
        // Convert RTSP to WebSocket FLV if needed
        if (res.protocol === 'rtsp') {
          // Use the NVR's RTSP proxy
          const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
          playbackStreamUrl = `${wsProto}//${location.host}/api/cameras/${selectedCameraId}/stream`;
        } else {
          playbackStreamUrl = res.uri;
        }
        playbackStreamId = `onvif_replay_${recording.token}`;
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : '获取回放地址失败', 'error');
    } finally {
      playbackLoading = false;
    }
  }

  async function handlePlaySegment(segment: ONVIFRecordingSegment) {
    playbackLoading = true;
    try {
      const res = await getONVIFReplayURI(selectedCameraId, segment.recording_token);
      if (res.uri) {
        if (res.protocol === 'rtsp') {
          const wsProto = location.protocol === 'https:' ? 'wss:' : 'ws:';
          playbackStreamUrl = `${wsProto}//${location.host}/api/cameras/${selectedCameraId}/stream`;
        } else {
          playbackStreamUrl = res.uri;
        }
        playbackStreamId = `onvif_replay_${segment.token}`;
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : '获取回放地址失败', 'error');
    } finally {
      playbackLoading = false;
    }
  }

  function formatTime(timeStr: string): string {
    if (!timeStr) return '-';
    try { return new Date(timeStr).toLocaleString('zh-CN'); } catch { return timeStr; }
  }

  onMount(() => {
    loadCameras();
    const range = getDefaultTimeRange();
    startTime = range.start;
    endTime = range.end;
  });
</script>

<div class="space-y-4">
  <!-- Query Form -->
  <div class="card border th-border p-4">
    <div class="flex items-center gap-2 mb-4">
      <Film size={20} class="th-text-secondary" />
      <h3 class="text-lg font-semibold th-text-primary">设备录像查询</h3>
    </div>
    
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 mb-3">
      <!-- Camera Selection -->
      <div>
        <label for="onvif-rec-camera" class="input-label">设备</label>
        <select id="onvif-rec-camera" class="input mt-1 w-full" bind:value={selectedCameraId}>
          <option value="">选择 ONVIF 设备</option>
          {#each cameras as cam}
            <option value={cam.id}>{cam.name}</option>
          {/each}
        </select>
      </div>

      <!-- Start Time -->
      <div>
        <label for="onvif-rec-start" class="input-label">开始时间</label>
        <input 
          id="onvif-rec-start" 
          type="datetime-local" 
          class="input mt-1 w-full" 
          bind:value={startTime} 
          step="1" 
        />
      </div>

      <!-- End Time -->
      <div>
        <label for="onvif-rec-end" class="input-label">结束时间</label>
        <input 
          id="onvif-rec-end" 
          type="datetime-local" 
          class="input mt-1 w-full" 
          bind:value={endTime} 
          step="1" 
        />
      </div>

      <!-- Search Mode -->
      <div>
        <label for="onvif-rec-mode" class="input-label">查询模式</label>
        <select id="onvif-rec-mode" class="input mt-1 w-full" bind:value={searchMode}>
          <option value="segments">片段搜索</option>
          <option value="recordings">录像列表</option>
        </select>
      </div>
    </div>

    <div class="flex justify-end">
      <button 
        class="btn btn-primary flex items-center gap-2" 
        onclick={handleSearch}
        disabled={loading || !selectedCameraId}
      >
        {#if loading}
          <RefreshCw class="w-4 h-4 animate-spin" />
          查询中...
        {:else}
          <Search class="w-4 h-4" />
          查询录像
        {/if}
      </button>
    </div>
  </div>

  <!-- Playback Player -->
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
    </div>
  {/if}

  <!-- Recordings List -->
  {#if searchMode === 'recordings' && recordings.length > 0}
    <div class="card border th-border overflow-hidden">
      <div class="px-4 py-3 border-b th-border flex items-center justify-between">
        <span class="text-sm th-text-secondary">共 {recordings.length} 个录像</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b th-border bg-gray-50 dark:bg-gray-800">
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">名称</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">来源</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">开始时间</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">结束时间</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">状态</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each recordings as rec}
              <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50">
                <td class="px-4 py-3 text-sm font-medium th-text-primary">{rec.name}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{rec.source.name || '-'}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(rec.start_time)}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(rec.end_time)}</td>
                <td class="px-4 py-3 text-sm">
                  <span class="px-2 py-1 text-xs rounded-full {rec.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                    {rec.status}
                  </span>
                </td>
                <td class="px-4 py-3 text-sm">
                  <button 
                    class="btn btn-sm btn-primary flex items-center gap-1"
                    onclick={() => handlePlayRecording(rec)}
                    disabled={playbackLoading}
                  >
                    {#if playbackLoading}
                      <RefreshCw class="w-3 h-3 animate-spin" />
                    {:else}
                      <Play class="w-3 h-3" />
                    {/if}
                    播放
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Segments List -->
  {#if searchMode === 'segments' && segments.length > 0}
    <div class="card border th-border overflow-hidden">
      <div class="px-4 py-3 border-b th-border flex items-center justify-between">
        <span class="text-sm th-text-secondary">共 {segments.length} 个片段</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full">
          <thead>
            <tr class="border-b th-border bg-gray-50 dark:bg-gray-800">
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">录像</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">开始时间</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">结束时间</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">时长</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">大小</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each segments as seg}
              <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50">
                <td class="px-4 py-3 text-sm font-mono th-text-primary max-w-xs truncate">{seg.recording_token}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(seg.start_time)}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(seg.end_time)}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">
                  {#if seg.duration}
                    {Math.floor(seg.duration / 60)}分{seg.duration % 60}秒
                  {:else}
                    -
                  {/if}
                </td>
                <td class="px-4 py-3 text-sm th-text-secondary">
                  {#if seg.size}
                    {formatFileSize(seg.size)}
                  {:else}
                    -
                  {/if}
                </td>
                <td class="px-4 py-3 text-sm">
                  <button 
                    class="btn btn-sm btn-primary flex items-center gap-1"
                    onclick={() => handlePlaySegment(seg)}
                    disabled={playbackLoading}
                  >
                    {#if playbackLoading}
                      <RefreshCw class="w-3 h-3 animate-spin" />
                    {:else}
                      <Play class="w-3 h-3" />
                    {/if}
                    播放
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Empty State -->
  {#if !loading && recordings.length === 0 && segments.length === 0 && selectedCameraId}
    <div class="flex flex-col items-center justify-center py-8 th-bg-secondary rounded-lg">
      <Film class="w-10 h-10 th-text-tertiary mb-3" />
      <p class="text-sm th-text-secondary">选择设备和时间范围后点击"查询录像"</p>
    </div>
  {/if}
</div>
