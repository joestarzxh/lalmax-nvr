<script lang="ts">
  import { onMount } from 'svelte';
  import {
    listGB28181Platforms,
    addGB28181Platform,
    deleteGB28181Platform,
    listGB28181Alarms,
    listGB28181Downloads,
    listGB28181Devices,
    queryDeviceRecords,
    startDevicePlayback,
  } from '$lib/api';
  import type { GB28181Platform, GB28181Alarm, GB28181Download, AddPlatformRequest, GB28181Device, DeviceRecordItem } from '$lib/api';
  import { showToast } from '$lib/toast';
  import FlvPlayer from '../components/FlvPlayer.svelte';
  import {
    Link, AlertTriangle, Download, RefreshCw, Plus, Trash2,
    Server, Wifi, WifiOff, X, Bell, CheckCircle, Clock,
    Search, Play, Film,
  } from 'lucide-svelte';

  type TabId = 'platforms' | 'alarms' | 'downloads' | 'records';
  let activeTab = $state<TabId>('platforms');

  const tabs: { id: TabId; label: string; icon: any }[] = [
    { id: 'platforms', label: '平台级联', icon: Link },
    { id: 'alarms', label: '报警记录', icon: AlertTriangle },
    { id: 'downloads', label: '下载记录', icon: Download },
    { id: 'records', label: '设备录像', icon: Film },
  ];

  // --- Platforms state ---
  let platforms = $state<GB28181Platform[]>([]);
  let platformsLoading = $state(true);
  let showAddDialog = $state(false);
  let saving = $state(false);
  let form = $state<AddPlatformRequest>({
    name: '', enable: true, server_gb_id: '', server_ip: '',
    server_port: 5060, transport: 'UDP', expires: 3600,
    keep_timeout: 60, max_timeout_count: 3,
  });

  // --- Alarms state ---
  let alarms = $state<GB28181Alarm[]>([]);
  let alarmsLoading = $state(false);
  let alarmsTotal = $state(0);
  let alarmsOffset = $state(0);
  const alarmsLimit = 50;

  // --- Downloads state ---
  let downloads = $state<GB28181Download[]>([]);
  let downloadsLoading = $state(false);
  let downloadsTotal = $state(0);
  let downloadsOffset = $state(0);
  const downloadsLimit = 50;

  // --- Platforms ---
  async function loadPlatforms() {
    platformsLoading = true;
    try {
      const res = await listGB28181Platforms();
      platforms = res.platforms || [];
    } catch (e) {
      showToast(e instanceof Error ? e.message : '加载失败', 'error');
    } finally {
      platformsLoading = false;
    }
  }

  function resetForm() {
    form = {
      name: '', enable: true, server_gb_id: '', server_ip: '',
      server_port: 5060, transport: 'UDP', expires: 3600,
      keep_timeout: 60, max_timeout_count: 3,
    };
  }

  async function handleAddPlatform() {
    if (!form.server_gb_id || !form.server_ip) {
      showToast('请填写必填字段', 'error');
      return;
    }
    saving = true;
    try {
      await addGB28181Platform(form);
      showToast('平台添加成功', 'success');
      showAddDialog = false;
      resetForm();
      await loadPlatforms();
    } catch (e) {
      showToast(e instanceof Error ? e.message : '添加失败', 'error');
    } finally {
      saving = false;
    }
  }

  async function handleDeletePlatform(id: number) {
    if (!confirm('确定删除此平台？')) return;
    try {
      await deleteGB28181Platform(id);
      showToast('平台已删除', 'success');
      await loadPlatforms();
    } catch (e) {
      showToast('删除失败', 'error');
    }
  }

  // --- Alarms ---
  async function loadAlarms() {
    alarmsLoading = true;
    try {
      const res = await listGB28181Alarms(undefined, alarmsLimit, alarmsOffset);
      alarms = res.alarms || [];
      alarmsTotal = res.total || 0;
    } catch (e) {
      showToast('加载失败', 'error');
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

  // --- Downloads ---
  async function loadDownloads() {
    downloadsLoading = true;
    try {
      const res = await listGB28181Downloads(undefined, undefined, downloadsLimit, downloadsOffset);
      downloads = res.downloads || [];
      downloadsTotal = res.total || 0;
    } catch (e) {
      showToast('加载失败', 'error');
    } finally {
      downloadsLoading = false;
    }
  }

  function downloadsNextPage() {
    if (downloadsOffset + downloadsLimit < downloadsTotal) {
      downloadsOffset += downloadsLimit;
      loadDownloads();
    }
  }
  function downloadsPrevPage() {
    if (downloadsOffset >= downloadsLimit) {
      downloadsOffset -= downloadsLimit;
      loadDownloads();
    }
  }

  function getStatusClass(s: string) {
    if (s === 'completed') return 'text-green-600';
    if (s === 'downloading') return 'text-blue-600';
    if (s === 'failed') return 'text-red-600';
    return 'text-gray-600';
  }
  function getStatusLabel(s: string) {
    if (s === 'completed') return '已完成';
    if (s === 'downloading') return '下载中';
    if (s === 'failed') return '失败';
    if (s === 'pending') return '等待中';
    return s;
  }
  function formatFileSize(bytes: number) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  function formatTime(timeStr: string): string {
    if (!timeStr) return '-';
    try { return new Date(timeStr).toLocaleString('zh-CN'); } catch { return timeStr; }
  }

  // --- Device Recording state ---
  let devices = $state<GB28181Device[]>([]);
  let recordDeviceId = $state('');
  let recordChannelId = $state('');
  let recordStartTime = $state('');
  let recordEndTime = $state('');
  let recordLoading = $state(false);
  let records = $state<DeviceRecordItem[]>([]);
  let playbackStreamUrl = $state('');
  let playbackStreamId = $state('');

  function getDefaultTimeRange() {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    return {
      start: today.toISOString().replace('Z', '').slice(0, 19),
      end: now.toISOString().replace('Z', '').slice(0, 19),
    };
  }

  async function loadDevices() {
    try {
      const res = await listGB28181Devices();
      devices = res.devices || [];
    } catch (e) {
      console.warn('Failed to load devices:', e);
    }
  }

  function getSelectedDeviceChannels() {
    const dev = devices.find(d => d.device_id === recordDeviceId);
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

  function switchTab(tab: TabId) {
    activeTab = tab;
    if (tab === 'platforms' && platforms.length === 0 && !platformsLoading) loadPlatforms();
    if (tab === 'alarms' && alarms.length === 0 && !alarmsLoading) loadAlarms();
    if (tab === 'downloads' && downloads.length === 0 && !downloadsLoading) loadDownloads();
    if (tab === 'records' && devices.length === 0) {
      loadDevices();
      const range = getDefaultTimeRange();
      recordStartTime = range.start;
      recordEndTime = range.end;
    }
  }

  onMount(loadPlatforms);
</script>

<div class="min-h-screen th-bg-primary">
  <main class="max-w-[1400px] mx-auto px-4 sm:px-6 lg:px-8 py-6">
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold th-text-primary flex items-center gap-2">
        <Link size={24} class="text-accent" />
        GB28181
      </h1>
      <button onclick={() => activeTab === 'platforms' ? loadPlatforms() : activeTab === 'alarms' ? loadAlarms() : loadDownloads()} class="btn btn-secondary flex items-center gap-2">
        <RefreshCw class="w-4 h-4" />
        刷新
      </button>
    </div>

    <!-- Tabs -->
    <div class="flex gap-1 p-1 rounded-lg th-bg-tertiary border th-border mb-6">
      {#each tabs as tab}
        {@const Icon = tab.icon}
        <button
          class="flex-1 flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium rounded-md transition-colors {activeTab === tab.id ? 'bg-[var(--color-primary)] text-white' : 'th-text-secondary hover:th-text-primary'}"
          onclick={() => switchTab(tab.id)}
        >
          <Icon size={16} />
          {tab.label}
        </button>
      {/each}
    </div>

    <!-- Platforms Tab -->
    {#if activeTab === 'platforms'}
      <div class="flex justify-end mb-4">
        <button onclick={() => { resetForm(); showAddDialog = true; }} class="btn btn-primary flex items-center gap-2">
          <Plus size={14} />
          添加平台
        </button>
      </div>

      {#if platformsLoading}
        <div class="flex justify-center py-12"><RefreshCw class="w-6 h-6 animate-spin th-text-secondary" /></div>
      {:else if platforms.length === 0}
        <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
          <Server class="w-12 h-12 th-text-tertiary mb-4" />
          <p class="text-lg th-text-secondary">暂无级联平台</p>
        </div>
      {:else}
        <div class="space-y-4">
          {#each platforms as platform}
            <div class="th-bg-secondary rounded-lg border th-border p-4">
              <div class="flex items-start justify-between">
                <div class="flex items-center gap-3">
                  <div class="p-2 rounded-lg {platform.status ? 'bg-green-100' : 'bg-gray-100'}">
                    {#if platform.status}
                      <Wifi class="w-5 h-5 text-green-600" />
                    {:else}
                      <WifiOff class="w-5 h-5 text-gray-400" />
                    {/if}
                  </div>
                  <div>
                    <h3 class="font-semibold th-text-primary">{platform.name || platform.server_gb_id}</h3>
                    <p class="text-sm th-text-secondary">
                      {platform.server_ip}:{platform.server_port} · {platform.transport}
                      {#if platform.status} · <span class="text-green-600">已注册</span>
                      {:else} · <span class="text-gray-500">未连接</span>{/if}
                    </p>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <span class="px-2 py-1 text-xs rounded-full {platform.enable ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-600'}">
                    {platform.enable ? '已启用' : '已禁用'}
                  </span>
                  <button onclick={() => handleDeletePlatform(platform.id)} class="btn btn-sm btn-danger flex items-center gap-1">
                    <Trash2 class="w-3 h-3" /> 删除
                  </button>
                </div>
              </div>
              <div class="mt-3 grid grid-cols-2 md:grid-cols-4 gap-2 text-sm">
                <div><span class="th-text-tertiary">上级 ID:</span><span class="th-text-primary ml-1 font-mono text-xs">{platform.server_gb_id}</span></div>
                <div><span class="th-text-tertiary">本端 ID:</span><span class="th-text-primary ml-1 font-mono text-xs">{platform.device_gb_id}</span></div>
                <div><span class="th-text-tertiary">本端 IP:</span><span class="th-text-primary ml-1">{platform.device_ip}:{platform.device_port}</span></div>
                <div><span class="th-text-tertiary">平台 ID:</span><span class="th-text-primary ml-1 font-mono text-xs">#{platform.id}</span></div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    {/if}

    <!-- Alarms Tab -->
    {#if activeTab === 'alarms'}
      {#if alarmsLoading && alarms.length === 0}
        <div class="flex justify-center py-12"><RefreshCw class="w-6 h-6 animate-spin th-text-secondary" /></div>
      {:else if alarms.length === 0}
        <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
          <Bell class="w-12 h-12 th-text-tertiary mb-4" />
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

    <!-- Downloads Tab -->
    {#if activeTab === 'downloads'}
      {#if downloadsLoading && downloads.length === 0}
        <div class="flex justify-center py-12"><RefreshCw class="w-6 h-6 animate-spin th-text-secondary" /></div>
      {:else if downloads.length === 0}
        <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
          <Download class="w-12 h-12 th-text-tertiary mb-4" />
          <p class="text-lg th-text-secondary">暂无下载记录</p>
        </div>
      {:else}
        <div class="th-bg-secondary rounded-lg border th-border overflow-hidden">
          <div class="overflow-x-auto">
            <table class="w-full">
              <thead>
                <tr class="border-b th-border bg-gray-50 dark:bg-gray-800">
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">ID</th>
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">设备 ID</th>
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">通道 ID</th>
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">时间范围</th>
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">文件大小</th>
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">状态</th>
                  <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">创建时间</th>
                </tr>
              </thead>
              <tbody>
                {#each downloads as dl}
                  <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50">
                    <td class="px-4 py-3 text-sm font-mono th-text-primary">#{dl.id}</td>
                    <td class="px-4 py-3 text-sm font-mono th-text-primary">{dl.device_id}</td>
                    <td class="px-4 py-3 text-sm font-mono th-text-primary">{dl.channel_id}</td>
                    <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(dl.start_time)} ~ {formatTime(dl.end_time)}</td>
                    <td class="px-4 py-3 text-sm th-text-secondary">{formatFileSize(dl.file_size)}</td>
                    <td class="px-4 py-3 text-sm">
                      <div class="flex items-center gap-1 {getStatusClass(dl.status)}">
                        {#if dl.status === 'completed'}<CheckCircle class="w-4 h-4" />
                        {:else}<Clock class="w-4 h-4" />{/if}
                        <span>{getStatusLabel(dl.status)}</span>
                      </div>
                    </td>
                    <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(dl.created_at)}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
          {#if downloadsTotal > downloadsLimit}
            <div class="flex items-center justify-between px-4 py-3 border-t th-border">
              <span class="text-sm th-text-secondary">共 {downloadsTotal} 条</span>
              <div class="flex gap-2">
                <button onclick={downloadsPrevPage} class="btn btn-sm btn-secondary" disabled={downloadsOffset === 0}>上一页</button>
                <span class="text-sm th-text-secondary px-2 py-1">{Math.floor(downloadsOffset / downloadsLimit) + 1} / {Math.ceil(downloadsTotal / downloadsLimit)}</span>
                <button onclick={downloadsNextPage} class="btn btn-sm btn-secondary" disabled={downloadsOffset + downloadsLimit >= downloadsTotal}>下一页</button>
              </div>
            </div>
          {/if}
        </div>
      {/if}
    {/if}

    <!-- Device Records Tab -->
    {#if activeTab === 'records'}
      <div class="space-y-4">
        <!-- Query form -->
        <div class="card border th-border p-4">
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
            <div>
              <label for="rec-device" class="input-label">设备</label>
              <select id="rec-device" class="input mt-1 w-full" bind:value={recordDeviceId} onchange={() => { recordChannelId = ''; records = []; playbackStreamUrl = ''; }}>
                <option value="">选择设备</option>
                {#each devices as dev}
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
    {/if}
  </main>
</div>

<!-- Add Platform Dialog -->
{#if showAddDialog}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onclick={() => showAddDialog = false} role="presentation">
    <div class="th-bg-secondary rounded-xl shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" tabindex="-1">
      <div class="flex items-center justify-between p-4 border-b th-border">
        <h2 class="text-lg font-semibold th-text-primary">添加上级平台</h2>
        <button onclick={() => showAddDialog = false} class="p-1 rounded hover:bg-gray-700"><X class="w-5 h-5" /></button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="gb-name" class="block text-sm font-medium th-text-secondary mb-1">平台名称</label>
          <input id="gb-name" type="text" bind:value={form.name} class="input w-full" placeholder="上级视频平台" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label for="gb-server-id" class="block text-sm font-medium th-text-secondary mb-1">上级 SIP ID *</label><input id="gb-server-id" type="text" bind:value={form.server_gb_id} class="input w-full" placeholder="34020000002000000002" /></div>
          <div><label for="gb-server-ip" class="block text-sm font-medium th-text-secondary mb-1">上级 IP *</label><input id="gb-server-ip" type="text" bind:value={form.server_ip} class="input w-full" placeholder="192.168.1.200" /></div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label for="gb-server-port" class="block text-sm font-medium th-text-secondary mb-1">上级端口</label><input id="gb-server-port" type="number" bind:value={form.server_port} class="input w-full" /></div>
          <div><label for="gb-transport" class="block text-sm font-medium th-text-secondary mb-1">传输协议</label><select id="gb-transport" bind:value={form.transport} class="input w-full"><option value="UDP">UDP</option><option value="TCP">TCP</option></select></div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div><label for="gb-username" class="block text-sm font-medium th-text-secondary mb-1">用户名</label><input id="gb-username" type="text" bind:value={form.username} class="input w-full" /></div>
          <div><label for="gb-password" class="block text-sm font-medium th-text-secondary mb-1">密码</label><input id="gb-password" type="password" bind:value={form.password} class="input w-full" /></div>
        </div>
        <div class="grid grid-cols-3 gap-4">
          <div><label for="gb-expires" class="block text-sm font-medium th-text-secondary mb-1">注册有效期</label><input id="gb-expires" type="number" bind:value={form.expires} class="input w-full" /></div>
          <div><label for="gb-keep-timeout" class="block text-sm font-medium th-text-secondary mb-1">心跳间隔</label><input id="gb-keep-timeout" type="number" bind:value={form.keep_timeout} class="input w-full" /></div>
          <div><label for="gb-max-timeout" class="block text-sm font-medium th-text-secondary mb-1">超时次数</label><input id="gb-max-timeout" type="number" bind:value={form.max_timeout_count} class="input w-full" /></div>
        </div>
        <label class="flex items-center gap-2"><input type="checkbox" bind:checked={form.enable} class="rounded" /><span class="text-sm th-text-secondary">立即启用</span></label>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t th-border">
        <button onclick={() => showAddDialog = false} class="btn btn-secondary">取消</button>
        <button onclick={handleAddPlatform} class="btn btn-primary" disabled={saving}>{saving ? '保存中...' : '添加'}</button>
      </div>
    </div>
  </div>
{/if}
