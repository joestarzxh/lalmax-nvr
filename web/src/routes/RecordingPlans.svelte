<script lang="ts">
  import { onMount } from 'svelte';
  import {
    listRecordingPlans,
    createRecordingPlan,
    updateRecordingPlan,
    deleteRecordingPlan,
    setPlanChannels,
    listCameras,
  } from '$lib/api';
  import type {
    RecordingPlan,
    RecordingPlanTimeRange,
    Camera,
  } from '$lib/api';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';
  import { Plus, Trash2, Edit2, Clock, X, Check, Copy } from 'lucide-svelte';

  const DAY_NAMES = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
  const DAY_NAMES_SHORT = ['日', '一', '二', '三', '四', '五', '六'];

  let plans = $state<RecordingPlan[]>([]);
  let cameras = $state<Camera[]>([]);
  let loading = $state(true);

  // Dialog state
  let showEditDialog = $state(false);
  let editingPlan = $state<RecordingPlan | null>(null);
  let formName = $state('');
  let formEnabled = $state(true);
  let formTimeRanges = $state<RecordingPlanTimeRange[]>([]);
  let formChannelIds = $state<string[]>([]);
  let saving = $state(false);

  // Channel picker state
  let showChannelPicker = $state(false);
  let channelSearch = $state('');

  async function loadPlans() {
    loading = true;
    try {
      const res = await listRecordingPlans();
      plans = res.plans || [];
    } catch (e) {
      showToast('加载失败', 'error');
    } finally {
      loading = false;
    }
  }

  async function loadCameras() {
    try {
      const data = await listCameras();
      cameras = (data || []).filter(c => !c.archived);
    } catch (e) {
      console.warn('Failed to load cameras:', e);
    }
  }

  function openCreateDialog() {
    editingPlan = null;
    formName = '';
    formEnabled = true;
    formTimeRanges = [];
    formChannelIds = [];
    showEditDialog = true;
  }

  function openEditDialog(plan: RecordingPlan) {
    editingPlan = plan;
    formName = plan.name;
    formEnabled = plan.enabled;
    formTimeRanges = plan.time_ranges.map(tr => ({ ...tr }));
    formChannelIds = plan.channels.map(c => c.camera_id);
    showEditDialog = true;
  }

  function addTimeRange(dayOfWeek: number) {
    formTimeRanges = [...formTimeRanges, {
      day_of_week: dayOfWeek,
      start_time: '00:00',
      end_time: '23:59',
    }];
  }

  function removeTimeRange(index: number) {
    formTimeRanges = formTimeRanges.filter((_, i) => i !== index);
  }

  function copyTimeRangeToAll(index: number) {
    const tr = formTimeRanges[index];
    const newRanges: RecordingPlanTimeRange[] = [];
    for (let day = 0; day < 7; day++) {
      if (day === tr.day_of_week) {
        newRanges.push(tr);
      } else {
        newRanges.push({
          day_of_week: day,
          start_time: tr.start_time,
          end_time: tr.end_time,
        });
      }
    }
    formTimeRanges = newRanges;
  }

  function copyTimeRangeToWeekdays(index: number) {
    const tr = formTimeRanges[index];
    const existing = formTimeRanges.filter(r => r.day_of_week !== tr.day_of_week);
    const newRanges: RecordingPlanTimeRange[] = [...existing];
    for (let day = 1; day <= 5; day++) {
      if (day !== tr.day_of_week) {
        newRanges.push({
          day_of_week: day,
          start_time: tr.start_time,
          end_time: tr.end_time,
        });
      }
    }
    formTimeRanges = newRanges;
  }

  function setTimeRangeDay(index: number, day: number) {
    formTimeRanges[index] = { ...formTimeRanges[index], day_of_week: day };
    formTimeRanges = [...formTimeRanges];
  }

  function setTimeRangeStart(index: number, val: string) {
    formTimeRanges[index] = { ...formTimeRanges[index], start_time: val };
    formTimeRanges = [...formTimeRanges];
  }

  function setTimeRangeEnd(index: number, val: string) {
    formTimeRanges[index] = { ...formTimeRanges[index], end_time: val };
    formTimeRanges = [...formTimeRanges];
  }

  function toggleChannel(cameraId: string) {
    if (formChannelIds.includes(cameraId)) {
      formChannelIds = formChannelIds.filter(id => id !== cameraId);
    } else {
      formChannelIds = [...formChannelIds, cameraId];
    }
  }

  async function handleSave() {
    if (!formName.trim()) {
      showToast('请输入计划名称', 'error');
      return;
    }
    if (formTimeRanges.length === 0) {
      showToast('请添加至少一个时间段', 'error');
      return;
    }

    saving = true;
    try {
      if (editingPlan) {
        await updateRecordingPlan(editingPlan.id, {
          name: formName.trim(),
          enabled: formEnabled,
          time_ranges: formTimeRanges,
        });
        await setPlanChannels(editingPlan.id, formChannelIds);
        showToast('计划已更新', 'success');
      } else {
        await createRecordingPlan({
          name: formName.trim(),
          enabled: formEnabled,
          time_ranges: formTimeRanges,
          channels: formChannelIds.map(id => ({ camera_id: id })),
        });
        showToast('计划已创建', 'success');
      }
      showEditDialog = false;
      await loadPlans();
    } catch (e) {
      showToast(e instanceof Error ? e.message : '保存失败', 'error');
    } finally {
      saving = false;
    }
  }

  async function handleDelete(plan: RecordingPlan) {
    if (!confirm(`确定删除计划"${plan.name}"？`)) return;
    try {
      await deleteRecordingPlan(plan.id);
      showToast('计划已删除', 'success');
      await loadPlans();
    } catch (e) {
      showToast('删除失败', 'error');
    }
  }

  async function handleToggleEnabled(plan: RecordingPlan) {
    try {
      await updateRecordingPlan(plan.id, { enabled: !plan.enabled });
      await loadPlans();
    } catch (e) {
      showToast('操作失败', 'error');
    }
  }

  function getTimeRangesForDay(plan: RecordingPlan, day: number): RecordingPlanTimeRange[] {
    return (plan.time_ranges || []).filter(tr => tr.day_of_week === day);
  }

  function getFilteredCameras() {
    const q = channelSearch.trim().toLowerCase();
    if (!q) return cameras;
    return cameras.filter(c => c.id.toLowerCase().includes(q) || (c.name || '').toLowerCase().includes(q));
  }

  onMount(() => {
    loadPlans();
    loadCameras();
  });
</script>

<div class="min-h-screen th-bg-primary">
  <main class="max-w-[1400px] mx-auto px-4 sm:px-6 lg:px-8 py-6">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold th-text-primary flex items-center gap-2">
          <Clock size={24} class="text-accent" />
          录像计划
        </h1>
        <p class="text-sm th-text-secondary mt-1">按时间段自动控制摄像头录制</p>
      </div>
      <button onclick={openCreateDialog} class="btn btn-primary flex items-center gap-2">
        <Plus size={16} />
        新建计划
      </button>
    </div>

    {#if loading}
      <div class="flex justify-center items-center h-32">
        <div class="spinner spinner-lg"></div>
      </div>
    {:else if plans.length === 0}
      <div class="card p-12 text-center">
        <Clock size={48} class="mx-auto mb-4 opacity-30" />
        <p class="text-lg th-text-secondary mb-2">暂无录像计划</p>
        <p class="text-sm th-text-tertiary mb-4">创建计划后，摄像头将在指定时间段内自动录制</p>
        <button onclick={openCreateDialog} class="btn btn-primary">
          <Plus size={16} />
          新建计划
        </button>
      </div>
    {:else}
      <div class="space-y-4">
        {#each plans as plan}
          <div class="card border th-border p-4">
            <div class="flex items-center justify-between mb-3">
              <div class="flex items-center gap-3">
                <button
                  class="w-10 h-6 rounded-full transition-colors {plan.enabled ? 'bg-green-500' : 'bg-gray-400'} relative"
                  onclick={() => handleToggleEnabled(plan)}
                  aria-label={plan.enabled ? '禁用计划' : '启用计划'}
                >
                  <span class="absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform {plan.enabled ? 'translate-x-4' : ''}"></span>
                </button>
                <div>
                  <h3 class="font-semibold th-text-primary">{plan.name}</h3>
                  <p class="text-xs th-text-tertiary">
                    {plan.channels?.length || 0} 个摄像头 · {plan.time_ranges?.length || 0} 个时间段
                  </p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <button onclick={() => openEditDialog(plan)} class="btn btn-ghost btn-sm">
                  <Edit2 size={14} />
                </button>
                <button onclick={() => handleDelete(plan)} class="btn btn-ghost btn-sm text-red-500">
                  <Trash2 size={14} />
                </button>
              </div>
            </div>

            <!-- Week overview -->
            <div class="grid grid-cols-7 gap-1">
              {#each DAY_NAMES_SHORT as dayName, dayIndex}
                {@const ranges = getTimeRangesForDay(plan, dayIndex)}
                <div class="text-center">
                  <div class="text-xs font-medium th-text-secondary mb-1">{dayName}</div>
                  {#if ranges.length > 0}
                    <div class="space-y-0.5">
                      {#each ranges as range}
                        <div class="text-[10px] bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400 rounded px-1 py-0.5">
                          {range.start_time}-{range.end_time}
                        </div>
                      {/each}
                    </div>
                  {:else}
                    <div class="text-[10px] th-text-muted">-</div>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </main>
</div>

<!-- Edit/Create Dialog -->
{#if showEditDialog}
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50" role="dialog">
    <div class="card w-full max-w-3xl max-h-[90vh] overflow-hidden flex flex-col">
      <div class="flex items-center justify-between p-4 border-b th-border">
        <h2 class="text-lg font-semibold th-text-primary">
          {editingPlan ? '编辑录像计划' : '新建录像计划'}
        </h2>
        <button onclick={() => showEditDialog = false} class="btn btn-ghost btn-sm">
          <X size={16} />
        </button>
      </div>

      <div class="flex-1 overflow-y-auto p-4 space-y-6">
        <!-- Basic info -->
        <div class="flex items-end gap-4">
          <div class="flex-1">
            <label for="plan-name" class="input-label">计划名称</label>
            <input id="plan-name" type="text" class="input mt-1 w-full" bind:value={formName} placeholder="例：全天录制、工作时间录制" />
          </div>
          <label class="flex items-center gap-2 mb-1">
            <input type="checkbox" bind:checked={formEnabled} class="rounded" />
            <span class="text-sm th-text-secondary">启用</span>
          </label>
        </div>

        <!-- Time ranges -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-semibold th-text-primary">时间段设置</h3>
          </div>

          <div class="space-y-3">
            {#each DAY_NAMES as dayName, dayIndex}
              {@const dayRanges = formTimeRanges.filter(tr => tr.day_of_week === dayIndex)}
              <div class="flex items-start gap-3">
                <div class="w-12 pt-2 text-sm font-medium th-text-secondary">{dayName}</div>
                <div class="flex-1 space-y-1">
                  {#each formTimeRanges as tr, trIndex}
                    {#if tr.day_of_week === dayIndex}
                      <div class="flex items-center gap-2">
                        <input
                          type="time"
                          class="input w-28"
                          value={tr.start_time}
                          oninput={(e) => setTimeRangeStart(trIndex, (e.target as HTMLInputElement).value)}
                        />
                        <span class="th-text-muted">至</span>
                        <input
                          type="time"
                          class="input w-28"
                          value={tr.end_time}
                          oninput={(e) => setTimeRangeEnd(trIndex, (e.target as HTMLInputElement).value)}
                        />
                        <button
                          class="btn btn-ghost btn-sm p-1"
                          title="复制到所有天"
                          onclick={() => copyTimeRangeToAll(trIndex)}
                        >
                          <Copy size={14} />
                        </button>
                        <button
                          class="btn btn-ghost btn-sm p-1"
                          title="复制到工作日"
                          onclick={() => copyTimeRangeToWeekdays(trIndex)}
                        >
                          <span class="text-xs">工</span>
                        </button>
                        <button
                          class="btn btn-ghost btn-sm p-1 text-red-500"
                          onclick={() => removeTimeRange(trIndex)}
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    {/if}
                  {/each}
                </div>
                <button
                  class="btn btn-ghost btn-sm text-xs mt-1"
                  onclick={() => addTimeRange(dayIndex)}
                >
                  <Plus size={12} />
                  添加
                </button>
              </div>
            {/each}
          </div>
        </div>

        <!-- Channels -->
        <div>
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-sm font-semibold th-text-primary">
              关联摄像头 ({formChannelIds.length})
            </h3>
            <button class="btn btn-ghost btn-sm" onclick={() => showChannelPicker = !showChannelPicker}>
              {showChannelPicker ? '收起' : '选择摄像头'}
            </button>
          </div>

          {#if showChannelPicker}
            <div class="border th-border rounded-lg p-3 max-h-60 overflow-y-auto">
              <input
                type="search"
                class="input w-full mb-2"
                placeholder="搜索摄像头..."
                bind:value={channelSearch}
              />
              <div class="space-y-1">
                {#each getFilteredCameras() as cam}
                  <button
                    class="w-full text-left px-3 py-2 rounded text-sm transition-colors {formChannelIds.includes(cam.id) ? 'bg-[var(--color-primary)]/10 th-text-primary' : 'hover:th-bg-hover th-text-secondary'}"
                    onclick={() => toggleChannel(cam.id)}
                  >
                    <div class="flex items-center justify-between">
                      <span>{cam.name || cam.id}</span>
                      {#if formChannelIds.includes(cam.id)}
                        <Check size={14} class="text-[var(--color-primary)]" />
                      {/if}
                    </div>
                  </button>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </div>

      <div class="flex justify-end gap-2 p-4 border-t th-border">
        <button onclick={() => showEditDialog = false} class="btn btn-secondary">取消</button>
        <button onclick={handleSave} class="btn btn-primary" disabled={saving}>
          {saving ? '保存中...' : '保存'}
        </button>
      </div>
    </div>
  </div>
{/if}
