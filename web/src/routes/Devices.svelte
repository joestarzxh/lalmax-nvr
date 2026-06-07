<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    listGB28181Devices, playGB28181Stream, stopGB28181Stream, 
    listStreams, listCameras
  } from '$lib/api';
  import type { GB28181Device, StreamInfo, Camera } from '$lib/api';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';
  import { 
    Video, RefreshCw, Play, Square, Wifi, WifiOff, 
    Radio, Camera as CameraIcon, Smartphone, 
    Monitor, Search 
  } from 'lucide-svelte';
  import DiscoveryPanel from '$lib/components/DiscoveryPanel.svelte';

  // Device type tabs
  type DeviceType = 'onvif' | 'gb28181' | 'xiaomi' | 'push';
  let activeTab = $state<DeviceType>('onvif');

  // GB28181 state
  let gb28181Devices = $state<GB28181Device[]>([]);
  let gb28181Loading = $state(false);
  let playingStreams = $state<Set<string>>(new Set());

  // ONVIF state (existing cameras)
  let onvifCameras = $state<Camera[]>([]);
  let onvifLoading = $state(false);

  // Push streams state
  let pushStreams = $state<StreamInfo[]>([]);
  let pushLoading = $state(false);

  // Discovery state
  let showDiscovery = $state(false);
  let discoveryPanel = $state<DiscoveryPanel | null>(null);

  // Common state
  let loading = $state(false);
  let error = $state('');

  const tabs = [
    { id: 'onvif' as DeviceType, label: 'ONVIF 设备', icon: 'camera' },
    { id: 'gb28181' as DeviceType, label: 'GB28181 设备', icon: 'monitor' },
    { id: 'xiaomi' as DeviceType, label: '小米设备', icon: 'smartphone' },
    { id: 'push' as DeviceType, label: '推流设备', icon: 'radio' },
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
      } else if (activeTab === 'push') {
        await loadPushStreams();
      }
    } catch (e) {
      error = e instanceof Error ? e.message : '加载失败';
    } finally {
      loading = false;
    }
  }

  const ONVIF_TAB_PROTOCOLS = new Set(['onvif', 'rtsp', 'http', 'rtsp_h264', 'rtsp_h265', 'rtsp_mjpeg', 'http_jpeg']);

  function isOnvifTabCamera(camera: Camera): boolean {
    return ONVIF_TAB_PROTOCOLS.has(camera.protocol);
  }

  async function loadOnvifCameras() {
    onvifLoading = true;
    try {
      const cameras = await listCameras();
      onvifCameras = (cameras || []).filter(isOnvifTabCamera);
    } catch (e) {
      console.error('Failed to load ONVIF cameras:', e);
    } finally {
      onvifLoading = false;
    }
  }

  async function loadGB28181Devices() {
    gb28181Loading = true;
    try {
      const res = await listGB28181Devices();
      gb28181Devices = res.devices || [];
      syncGB28181PlayingStreams(gb28181Devices);
    } catch (e) {
      console.error('Failed to load GB28181 devices:', e);
    } finally {
      gb28181Loading = false;
    }
  }

  async function loadPushStreams() {
    pushLoading = true;
    try {
      const res = await listStreams();
      pushStreams = (res.streams || []).filter(s => 
        !s.managed && s.source_type === 'push'
      );
    } catch (e) {
      console.error('Failed to load push streams:', e);
    } finally {
      pushLoading = false;
    }
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

  function handleTabChange(tab: DeviceType) {
    activeTab = tab;
    showDiscovery = false;
    loadDevices();
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
      default: return CameraIcon;
    }
  }

  onMount(() => {
    loadDevices();
  });
</script>

<div class="min-h-screen th-bg-primary pt-[68px]">
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

    <!-- Error display -->
    {#if error}
      <div class="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg">
        <p class="text-red-600">{error}</p>
      </div>
    {/if}

    <!-- ONVIF Devices -->
    {#if activeTab === 'onvif'}
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
        <div class="grid gap-4">
          {#each onvifCameras as camera}
            <div class="th-bg-secondary rounded-lg border th-border p-4">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div class="p-2 rounded-lg {camera.enabled ? 'bg-blue-100' : 'bg-gray-100'}">
                    <CameraIcon class="w-5 h-5 {camera.enabled ? 'text-blue-600' : 'text-gray-400'}" />
                  </div>
                  <div>
                    <h3 class="font-semibold th-text-primary">{camera.name}</h3>
                    <p class="text-sm th-text-secondary">
                      {camera.protocol} · {camera.encoding}
                      {#if camera.url}
                        · {camera.url}
                      {/if}
                    </p>
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <span class="px-2 py-1 text-xs rounded-full {camera.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}">
                    {camera.enabled ? '启用' : '禁用'}
                  </span>
                  <a href="#/live/{camera.id}" class="btn btn-sm btn-secondary">
                    详情
                  </a>
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}

    <!-- GB28181 Devices -->
    {:else if activeTab === 'gb28181'}
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
                    <h3 class="font-semibold th-text-primary">{device.device_id}</h3>
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

              {#if device.channels && device.channels.length > 0}
                <div class="mt-4 border-t th-border pt-4">
                  <h4 class="text-sm font-medium th-text-secondary mb-3">通道列表 ({device.channels.length})</h4>
                  <div class="grid gap-2">
                    {#each device.channels as channel}
                      {@const streamKey = `${device.device_id}:${channel.channel_id}`}
                      <div class="flex items-center justify-between p-3 bg-white dark:bg-gray-800 rounded-lg border th-border">
                        <div class="flex items-center gap-2">
                          <Video class="w-4 h-4 th-text-tertiary" />
                          <span class="text-sm th-text-primary">{channel.channel_id}</span>
                        </div>
                        <div class="flex items-center gap-2">
                          {#if playingStreams.has(streamKey)}
                            <button
                              onclick={() => handleGB28181Stop(device, channel.channel_id)}
                              class="btn btn-sm btn-danger flex items-center gap-1"
                            >
                              <Square class="w-3 h-3" />
                              停止
                            </button>
                          {:else}
                            <button
                              onclick={() => handleGB28181Play(device, channel.channel_id)}
                              class="btn btn-sm btn-primary flex items-center gap-1"
                              disabled={!device.is_online}
                            >
                              <Play class="w-3 h-3" />
                              播放
                            </button>
                          {/if}
                        </div>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}

    <!-- Xiaomi Devices -->
    {:else if activeTab === 'xiaomi'}
      <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
        <Smartphone class="w-12 h-12 th-text-tertiary mb-4" />
        <p class="text-lg th-text-secondary">暂无小米设备</p>
        <p class="text-sm th-text-tertiary mt-1">点击上方「扫描设备」按钮登录小米账号并发现设备</p>
      </div>

    <!-- Push Streams -->
    {:else if activeTab === 'push'}
      {#if pushLoading}
        <div class="flex items-center justify-center py-12">
          <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
          <span class="ml-2 th-text-secondary">加载中...</span>
        </div>
      {:else if pushStreams.length === 0}
        <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
          <Radio class="w-12 h-12 th-text-tertiary mb-4" />
          <p class="text-lg th-text-secondary">暂无推流</p>
          <p class="text-sm th-text-tertiary mt-1">使用 RTMP 或 SRT 推流后将显示在这里，可在「推流设备」页面升级为摄像头</p>
        </div>
      {:else}
        <div class="grid gap-4">
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
                        · 来源: {stream.source_type}
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
    {/if}
  </main>
</div>
