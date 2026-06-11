<script lang="ts">
  import { onMount } from 'svelte';
  import { listGB28181Downloads } from '$lib/api';
  import type { GB28181Download } from '$lib/api';
  import { showToast } from '$lib/toast';
  import { RefreshCw, Download, CheckCircle, Clock, XCircle } from 'lucide-svelte';

  let downloads = $state<GB28181Download[]>([]);
  let loading = $state(true);
  let total = $state(0);
  let offset = $state(0);
  const limit = 50;

  async function loadDownloads() {
    loading = true;
    try {
      const res = await listGB28181Downloads(undefined, undefined, limit, offset);
      downloads = res.downloads || [];
      total = res.total || 0;
    } catch (e) {
      showToast(e instanceof Error ? e.message : '加载失败', 'error');
    } finally {
      loading = false;
    }
  }

  function nextPage() {
    if (offset + limit < total) {
      offset += limit;
      loadDownloads();
    }
  }

  function prevPage() {
    if (offset >= limit) {
      offset -= limit;
      loadDownloads();
    }
  }

  function formatTime(timeStr: string): string {
    if (!timeStr) return '-';
    try {
      return new Date(timeStr).toLocaleString('zh-CN');
    } catch {
      return timeStr;
    }
  }

  function formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  }

  function getStatusIcon(status: string) {
    switch (status) {
      case 'completed': return CheckCircle;
      case 'downloading': return Clock;
      case 'failed': return XCircle;
      default: return Clock;
    }
  }

  function getStatusClass(status: string): string {
    switch (status) {
      case 'completed': return 'text-green-600';
      case 'downloading': return 'text-blue-600';
      case 'failed': return 'text-red-600';
      default: return 'text-gray-600';
    }
  }

  function getStatusLabel(status: string): string {
    switch (status) {
      case 'completed': return '已完成';
      case 'downloading': return '下载中';
      case 'failed': return '失败';
      case 'pending': return '等待中';
      default: return status;
    }
  }

  onMount(loadDownloads);
</script>

<div class="min-h-screen th-bg-primary ">
  <main class="mx-auto px-3 sm:px-4 lg:px-6 py-4 sm:py-6" style="max-width: 1200px;">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold th-text-primary">下载记录</h1>
        <p class="text-sm th-text-secondary mt-1">GB28181 设备录像下载记录</p>
      </div>
      <button onclick={loadDownloads} class="btn btn-secondary flex items-center gap-2" disabled={loading}>
        <RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
        刷新
      </button>
    </div>

    {#if loading && downloads.length === 0}
      <div class="flex items-center justify-center py-12">
        <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
        <span class="ml-2 th-text-secondary">加载中...</span>
      </div>
    {:else if downloads.length === 0}
      <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
        <Download class="w-12 h-12 th-text-tertiary mb-4" />
        <p class="text-lg th-text-secondary">暂无下载记录</p>
        <p class="text-sm th-text-tertiary mt-1">录像下载记录将显示在此处</p>
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
                  <td class="px-4 py-3 text-sm th-text-secondary">
                    {formatTime(dl.start_time)} ~ {formatTime(dl.end_time)}
                  </td>
                  <td class="px-4 py-3 text-sm th-text-secondary">{formatFileSize(dl.file_size)}</td>
                  <td class="px-4 py-3 text-sm">
                    <div class="flex items-center gap-1 {getStatusClass(dl.status)}">
                      {#if dl.status === 'completed'}
                        <CheckCircle class="w-4 h-4" />
                      {:else if dl.status === 'downloading'}
                        <Clock class="w-4 h-4" />
                      {:else if dl.status === 'failed'}
                        <XCircle class="w-4 h-4" />
                      {:else}
                        <Clock class="w-4 h-4" />
                      {/if}
                      <span>{getStatusLabel(dl.status)}</span>
                    </div>
                  </td>
                  <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(dl.created_at)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
        {#if total > limit}
          <div class="flex items-center justify-between px-4 py-3 border-t th-border">
            <span class="text-sm th-text-secondary">共 {total} 条记录</span>
            <div class="flex gap-2">
              <button onclick={prevPage} class="btn btn-sm btn-secondary" disabled={offset === 0}>上一页</button>
              <span class="text-sm th-text-secondary px-2 py-1">
                {Math.floor(offset / limit) + 1} / {Math.ceil(total / limit)}
              </span>
              <button onclick={nextPage} class="btn btn-sm btn-secondary" disabled={offset + limit >= total}>下一页</button>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </main>
</div>
