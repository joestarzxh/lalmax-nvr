<script lang="ts">
  import { onMount } from 'svelte';
  import {
    getAiStatus,
    enableAiDetection,
    disableAiDetection,
    subscribeAiEvents,
    listCameras,
  } from '$lib/api';
  import type { AiStatusResponse, AiDetectionEvent, Camera } from '$lib/api';
  import { showToast } from '$lib/toast';
  import {
    Brain, RefreshCw, Play, Square, Activity,
    AlertTriangle, CheckCircle, XCircle, Cpu,
  } from 'lucide-svelte';

  // State
  let aiStatus = $state<AiStatusResponse | null>(null);
  let loading = $state(true);
  let cameras = $state<Camera[]>([]);
  let enabledCameras = $state<Set<string>>(new Set());
  let events = $state<AiDetectionEvent[]>([]);
  let eventCleanup: (() => void) | null = null;

  // Load AI status
  async function loadAiStatus() {
    loading = true;
    try {
      aiStatus = await getAiStatus();
    } catch (e) {
      console.warn('Failed to load AI status:', e);
    } finally {
      loading = false;
    }
  }

  // Load cameras
  async function loadCameras() {
    try {
      const res = await listCameras();
      cameras = res.cameras || [];
    } catch (e) {
      console.warn('Failed to load cameras:', e);
    }
  }

  // Toggle AI for a camera
  async function toggleCameraAi(cameraId: string, enable: boolean) {
    try {
      if (enable) {
        await enableAiDetection(cameraId);
        enabledCameras = new Set([...enabledCameras, cameraId]);
        showToast(`AI detection enabled for ${cameraId}`, 'success');
      } else {
        await disableAiDetection(cameraId);
        const next = new Set(enabledCameras);
        next.delete(cameraId);
        enabledCameras = next;
        showToast(`AI detection disabled for ${cameraId}`, 'success');
      }
    } catch (e) {
      showToast(e instanceof Error ? e.message : 'Failed to toggle AI', 'error');
    }
  }

  // Subscribe to SSE events
  function startEventStream() {
    eventCleanup = subscribeAiEvents(
      (event) => {
        events = [event, ...events.slice(0, 99)]; // Keep last 100 events
      },
      (error) => {
        console.warn('AI SSE error:', error);
      }
    );
  }

  // Format confidence as percentage
  function formatConfidence(confidence: number): string {
    return `${Math.round(confidence * 100)}%`;
  }

  // Get backend label
  function getBackendLabel(backend: string): string {
    switch (backend) {
      case 'http': return 'HTTP 远程服务';
      case 'webhook': return 'Webhook 推送';
      case 'disabled': return '已禁用';
      default: return '未知';
    }
  }

  // Get backend icon
  function getBackendIcon(backend: string) {
    switch (backend) {
      case 'http': return CheckCircle;
      case 'webhook': return CheckCircle;
      case 'disabled': return XCircle;
      default: return AlertTriangle;
    }
  }

  // Get backend color
  function getBackendColor(backend: string): string {
    switch (backend) {
      case 'http': return 'text-blue-500';
      case 'webhook': return 'text-purple-500';
      case 'disabled': return 'text-gray-500';
      default: return 'text-gray-500';
    }
  }

  onMount(() => {
    loadAiStatus();
    loadCameras();
    startEventStream();

    return () => {
      if (eventCleanup) eventCleanup();
    };
  });
</script>

<main class="max-w-[1400px] mx-auto px-4 sm:px-6 lg:px-8 py-6">
  <div class="flex items-center justify-between mb-6">
    <h1 class="text-2xl font-bold th-text-primary flex items-center gap-2">
      <Brain size={28} />
      AI 检测
    </h1>
    <button class="btn btn-ghost" onclick={() => { loadAiStatus(); loadCameras(); }}>
      <RefreshCw size={16} class={loading ? 'animate-spin' : ''} />
    </button>
  </div>

  <!-- AI Status Card -->
  <div class="card border th-border p-6 mb-6">
    <h2 class="text-lg font-semibold th-text-primary mb-4 flex items-center gap-2">
      <Cpu size={20} /> 服务状态
    </h2>

    {#if loading}
      <div class="flex items-center justify-center py-8">
        <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
      </div>
    {:else if aiStatus}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <!-- Backend Type -->
        <div class="p-4 rounded-lg bg-gray-50 dark:bg-gray-800">
          <div class="text-sm th-text-secondary mb-1">后端类型</div>
          <div class="flex items-center gap-2">
            {#if aiStatus.backend === 'http'}
              <CheckCircle size={20} class="text-blue-500" />
            {:else if aiStatus.backend === 'webhook'}
              <CheckCircle size={20} class="text-purple-500" />
            {:else}
              <XCircle size={20} class="text-gray-500" />
            {/if}
            <span class="font-medium th-text-primary">{getBackendLabel(aiStatus.backend)}</span>
          </div>
        </div>

        <!-- Available -->
        <div class="p-4 rounded-lg bg-gray-50 dark:bg-gray-800">
          <div class="text-sm th-text-secondary mb-1">可用性</div>
          <div class="flex items-center gap-2">
            {#if aiStatus.available}
              <span class="px-2 py-1 text-xs rounded-full bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
                可用
              </span>
            {:else}
              <span class="px-2 py-1 text-xs rounded-full bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                不可用
              </span>
            {/if}
          </div>
        </div>
      </div>

      <!-- Status Message -->
      {#if aiStatus.reason}
        <div class="mt-4 p-3 rounded-lg bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800">
          <div class="flex items-center gap-2 text-yellow-800 dark:text-yellow-200">
            <AlertTriangle size={16} />
            <span class="text-sm">{aiStatus.reason}</span>
          </div>
        </div>
      {/if}

      <!-- Backend Description -->
      {#if aiStatus.backend === 'http' && aiStatus.available}
        <div class="mt-4 p-3 rounded-lg bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800">
          <div class="text-sm text-blue-800 dark:text-blue-200">
            NVR 会将视频帧发送到配置的远程 AI 服务进行检测。
          </div>
        </div>
      {:else if aiStatus.backend === 'webhook' && aiStatus.available}
        <div class="mt-4 p-3 rounded-lg bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800">
          <div class="text-sm text-purple-800 dark:text-purple-200">
            外部 AI 服务可以通过 POST /api/ai/webhook 推送检测结果。
          </div>
        </div>
      {/if}
    {:else}
      <div class="text-center py-8 th-text-secondary">
        无法获取 AI 服务状态
      </div>
    {/if}
  </div>

  <!-- Camera AI Toggle -->
  {#if aiStatus && aiStatus.backend === 'http'}
    <div class="card border th-border p-6 mb-6">
      <h2 class="text-lg font-semibold th-text-primary mb-4 flex items-center gap-2">
        <Activity size={20} /> 摄像头 AI 检测
      </h2>

      {#if cameras.length === 0}
        <div class="text-center py-8 th-text-secondary">
          暂无摄像头
        </div>
      {:else}
        <div class="space-y-3">
          {#each cameras as camera}
            <div class="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-800">
              <div>
                <div class="font-medium th-text-primary">{camera.name}</div>
                <div class="text-sm th-text-secondary">{camera.id}</div>
              </div>
              <button
                class="btn btn-sm {enabledCameras.has(camera.id) ? 'btn-danger' : 'btn-primary'}"
                onclick={() => toggleCameraAi(camera.id, !enabledCameras.has(camera.id))}
              >
                {#if enabledCameras.has(camera.id)}
                  <Square size={14} /> 禁用
                {:else}
                  <Play size={14} /> 启用
                {/if}
              </button>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  <!-- Detection Events -->
  <div class="card border th-border p-6">
    <h2 class="text-lg font-semibold th-text-primary mb-4 flex items-center gap-2">
      <Activity size={20} /> 检测事件
    </h2>

    {#if events.length === 0}
      <div class="text-center py-8 th-text-secondary">
        暂无检测事件
      </div>
    {:else}
      <div class="overflow-x-auto max-h-96 overflow-y-auto">
        <table class="w-full">
          <thead class="sticky top-0 bg-white dark:bg-gray-900">
            <tr class="border-b th-border">
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">时间</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">摄像头</th>
              <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">检测结果</th>
            </tr>
          </thead>
          <tbody>
            {#each events as event}
              <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800">
                <td class="px-4 py-3 text-sm th-text-secondary">
                  {new Date(event.timestamp || event.pts / 1000).toLocaleTimeString()}
                </td>
                <td class="px-4 py-3 text-sm font-mono th-text-primary">
                  {event.camera_id}
                </td>
                <td class="px-4 py-3">
                  <div class="flex flex-wrap gap-1">
                    {#each event.detections as det}
                      <span class="px-2 py-1 text-xs rounded-full bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
                        {det.label} ({formatConfidence(det.confidence)})
                      </span>
                    {/each}
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</main>
