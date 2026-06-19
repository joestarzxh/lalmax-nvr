<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    getONVIFRecordings, 
    getONVIFRecordingInformation,
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
    Calendar, Clock, HardDrive, X, ChevronLeft
  } from 'lucide-svelte';
  import FlvPlayer from '../../components/FlvPlayer.svelte';

  let cameras = $state<Camera[]>([]);
  let selectedCameraId = $state('');
  let startTime = $state('');
  let endTime = $state('');
  let loading = $state(false);
  let recordings = $state<ONVIFRecording[]>([]);
  
  // Drill-down state
  let selectedRecording = $state<ONVIFRecording | null>(null);
  let recordingSegments = $state<ONVIFRecordingSegment[]>([]);
  let loadingSegments = $state(false);
  
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
    selectedRecording = null;
    recordingSegments = [];
    playbackStreamUrl = '';

    try {
      const res = await getONVIFRecordings(selectedCameraId, {
        start_time: startTime,
        end_time: endTime || undefined,
      });
      recordings = res.recordings || [];
      if (recordings.length === 0) {
        showToast('未找到录像记录', 'info');
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : '查询失败';
      if (msg.includes('does not support ONVIF recording')) {
        showToast('该设备不支持 ONVIF 录像查询功能', 'error');
      } else {
        showToast(msg, 'error');
      }
    } finally {
      loading = false;
    }
  }

  async function handleViewRecording(recording: ONVIFRecording) {
    // Get recording details from the already loaded recordings
    selectedRecording = recording;
    recordingSegments = [];
    
    // Note: Dahua cameras don't support GetRecordingInformation API
    // so we can't get actual segments. Show tracks info instead.
    showToast('该设备不支持获取录像片段详情', 'info');
  }

  function handleBackToRecordings() {
    selectedRecording = null;
    recordingSegments = [];
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
    
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 mb-3">
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
  {#if recordings.length > 0 && !selectedRecording}
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
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">轨道数</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">状态</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">操作</th>
            </tr>
          </thead>
          <tbody>
            {#each recordings as rec}
              <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50 cursor-pointer" onclick={() => handleViewRecording(rec)}>
                <td class="px-4 py-3 text-sm font-medium th-text-primary">{rec.name || rec.token}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{rec.source.name || '-'}</td>
                <td class="px-4 py-3 text-sm th-text-secondary">{rec.tracks?.length || 0}</td>
                <td class="px-4 py-3 text-sm">
                  <span class="px-2 py-1 text-xs rounded-full {rec.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                    {rec.status}
                  </span>
                </td>
                <td class="px-4 py-3 text-sm">
                  <button 
                    class="btn btn-sm btn-primary flex items-center gap-1"
                    onclick={(e) => { e.stopPropagation(); handleViewRecording(rec); }}
                  >
                    <Film class="w-3 h-3" />
                    查看片段
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Recording Detail -->
  {#if selectedRecording}
    <div class="card border th-border overflow-hidden">
      <div class="px-4 py-3 border-b th-border flex items-center justify-between">
        <div class="flex items-center gap-2">
          <button class="btn btn-ghost btn-sm" onclick={handleBackToRecordings}>
            <ChevronLeft size={16} />
            返回
          </button>
          <span class="text-sm font-medium th-text-primary">{selectedRecording.description || selectedRecording.token}</span>
        </div>
      </div>
      
      <div class="p-4">
        <div class="grid grid-cols-2 gap-4 mb-4">
          <div>
            <span class="text-xs th-text-tertiary">Token</span>
            <p class="text-sm font-mono th-text-secondary">{selectedRecording.token}</p>
          </div>
          <div>
            <span class="text-xs th-text-tertiary">状态</span>
            <p class="text-sm">
              <span class="px-2 py-1 text-xs rounded-full {selectedRecording.status === 'active' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                {selectedRecording.status}
              </span>
            </p>
          </div>
          <div>
            <span class="text-xs th-text-tertiary">来源</span>
            <p class="text-sm th-text-secondary">{selectedRecording.source?.name || '-'}</p>
          </div>
          <div>
            <span class="text-xs th-text-tertiary">描述</span>
            <p class="text-sm th-text-secondary">{selectedRecording.description || '-'}</p>
          </div>
        </div>

        {#if selectedRecording.tracks && selectedRecording.tracks.length > 0}
          <div>
            <h4 class="text-sm font-medium th-text-primary mb-2">轨道信息</h4>
            <div class="overflow-x-auto">
              <table class="w-full">
                <thead>
                  <tr class="border-b th-border bg-gray-50 dark:bg-gray-800">
                    <th class="px-4 py-2 text-left text-sm font-medium th-text-secondary">Token</th>
                    <th class="px-4 py-2 text-left text-sm font-medium th-text-secondary">类型</th>
                    <th class="px-4 py-2 text-left text-sm font-medium th-text-secondary">描述</th>
                  </tr>
                </thead>
                <tbody>
                  {#each selectedRecording.tracks as track}
                    <tr class="border-b th-border">
                      <td class="px-4 py-2 text-sm font-mono th-text-secondary">{track.token}</td>
                      <td class="px-4 py-2 text-sm th-text-secondary">
                        <span class="px-2 py-1 text-xs rounded-full {track.track_type === 'Video' ? 'bg-blue-100 text-blue-800' : 'bg-purple-100 text-purple-800'}">
                          {track.track_type}
                        </span>
                      </td>
                      <td class="px-4 py-2 text-sm th-text-secondary">{track.description || '-'}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        {/if}

        <div class="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
          <p class="text-sm text-yellow-800 dark:text-yellow-200">
            注意：该设备不支持获取录像片段详情。如需查看具体录像片段，请通过设备的 Web 界面或客户端软件。
          </p>
        </div>
      </div>
    </div>
  {/if}

  <!-- Empty State -->
  {#if !loading && recordings.length === 0 && !selectedRecording && selectedCameraId}
    <div class="flex flex-col items-center justify-center py-8 th-bg-secondary rounded-lg">
      <Film class="w-10 h-10 th-text-tertiary mb-3" />
      <p class="text-sm th-text-secondary">选择设备和时间范围后点击"查询录像"</p>
    </div>
  {/if}
</div>
