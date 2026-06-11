<script lang="ts">
  import { onMount } from 'svelte';
  import { listGB28181Alarms } from '$lib/api';
  import type { GB28181Alarm } from '$lib/api';
  import { showToast } from '$lib/toast';
  import { RefreshCw, AlertTriangle, Bell } from 'lucide-svelte';

  let alarms = $state<GB28181Alarm[]>([]);
  let loading = $state(true);
  let total = $state(0);
  let offset = $state(0);
  const limit = 50;

  async function loadAlarms() {
    loading = true;
    try {
      const res = await listGB28181Alarms(undefined, limit, offset);
      alarms = res.alarms || [];
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
      loadAlarms();
    }
  }

  function prevPage() {
    if (offset >= limit) {
      offset -= limit;
      loadAlarms();
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

  function getPriorityLabel(priority: number): string {
    if (priority >= 5) return '紧急';
    if (priority >= 3) return '重要';
    return '一般';
  }

  function getPriorityClass(priority: number): string {
    if (priority >= 5) return 'bg-red-100 text-red-800';
    if (priority >= 3) return 'bg-orange-100 text-orange-800';
    return 'bg-blue-100 text-blue-800';
  }

  onMount(loadAlarms);
</script>

<div class="min-h-screen th-bg-primary ">
  <main class="mx-auto px-3 sm:px-4 lg:px-6 py-4 sm:py-6" style="max-width: 1200px;">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold th-text-primary">报警记录</h1>
        <p class="text-sm th-text-secondary mt-1">GB28181 设备报警信息</p>
      </div>
      <button onclick={loadAlarms} class="btn btn-secondary flex items-center gap-2" disabled={loading}>
        <RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
        刷新
      </button>
    </div>

    {#if loading && alarms.length === 0}
      <div class="flex items-center justify-center py-12">
        <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
        <span class="ml-2 th-text-secondary">加载中...</span>
      </div>
    {:else if alarms.length === 0}
      <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
        <Bell class="w-12 h-12 th-text-tertiary mb-4" />
        <p class="text-lg th-text-secondary">暂无报警记录</p>
        <p class="text-sm th-text-tertiary mt-1">设备报警信息将显示在此处</p>
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
                    <span class="px-2 py-1 text-xs rounded-full {getPriorityClass(alarm.priority)}">
                      {getPriorityLabel(alarm.priority)}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-sm th-text-secondary">{formatTime(alarm.alarm_time)}</td>
                  <td class="px-4 py-3 text-sm th-text-secondary max-w-xs truncate">{alarm.description || '-'}</td>
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
