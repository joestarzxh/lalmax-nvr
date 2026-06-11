<script lang="ts">
  import { onMount } from 'svelte';
  import { listGB28181Platforms, addGB28181Platform, deleteGB28181Platform } from '$lib/api';
  import type { GB28181Platform, AddPlatformRequest } from '$lib/api';
  import { showToast } from '$lib/toast';
  import { RefreshCw, Plus, Trash2, Server, Wifi, WifiOff, X } from 'lucide-svelte';

  let platforms = $state<GB28181Platform[]>([]);
  let loading = $state(true);
  let showAddDialog = $state(false);
  let saving = $state(false);

  let form = $state<AddPlatformRequest>({
    name: '',
    enable: true,
    server_gb_id: '',
    server_ip: '',
    server_port: 5060,
    transport: 'UDP',
    expires: 3600,
    keep_timeout: 60,
    max_timeout_count: 3,
  });

  async function loadPlatforms() {
    loading = true;
    try {
      const res = await listGB28181Platforms();
      platforms = res.platforms || [];
    } catch (e) {
      showToast(e instanceof Error ? e.message : '加载失败', 'error');
    } finally {
      loading = false;
    }
  }

  async function handleAdd() {
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

  async function handleDelete(id: number) {
    if (!confirm('确定删除此平台？')) return;
    try {
      await deleteGB28181Platform(id);
      showToast('平台已删除', 'success');
      await loadPlatforms();
    } catch (e) {
      showToast(e instanceof Error ? e.message : '删除失败', 'error');
    }
  }

  function resetForm() {
    form = {
      name: '',
      enable: true,
      server_gb_id: '',
      server_ip: '',
      server_port: 5060,
      transport: 'UDP',
      expires: 3600,
      keep_timeout: 60,
      max_timeout_count: 3,
    };
  }

  function openAddDialog() {
    resetForm();
    showAddDialog = true;
  }

  onMount(loadPlatforms);
</script>

<div class="min-h-screen th-bg-primary ">
  <main class="mx-auto px-3 sm:px-4 lg:px-6 py-4 sm:py-6" style="max-width: 1200px;">
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold th-text-primary">平台级联</h1>
        <p class="text-sm th-text-secondary mt-1">管理上级平台连接，共享通道资源</p>
      </div>
      <div class="flex gap-2">
        <button onclick={loadPlatforms} class="btn btn-secondary flex items-center gap-2" disabled={loading}>
          <RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
          刷新
        </button>
        <button onclick={openAddDialog} class="btn btn-primary flex items-center gap-2">
          <Plus class="w-4 h-4" />
          添加平台
        </button>
      </div>
    </div>

    {#if loading && platforms.length === 0}
      <div class="flex items-center justify-center py-12">
        <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
        <span class="ml-2 th-text-secondary">加载中...</span>
      </div>
    {:else if platforms.length === 0}
      <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
        <Server class="w-12 h-12 th-text-tertiary mb-4" />
        <p class="text-lg th-text-secondary">暂无级联平台</p>
        <p class="text-sm th-text-tertiary mt-1">点击"添加平台"按钮添加上级平台</p>
      </div>
    {:else}
      <div class="grid gap-4">
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
                    {platform.server_ip}:{platform.server_port}
                    · {platform.transport}
                    {#if platform.status}
                      · <span class="text-green-600">已注册</span>
                    {:else}
                      · <span class="text-gray-500">未连接</span>
                    {/if}
                  </p>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span class="px-2 py-1 text-xs rounded-full {platform.enable ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-600'}">
                  {platform.enable ? '已启用' : '已禁用'}
                </span>
                <button
                  onclick={() => handleDelete(platform.id)}
                  class="btn btn-sm btn-danger flex items-center gap-1"
                >
                  <Trash2 class="w-3 h-3" />
                  删除
                </button>
              </div>
            </div>
            <div class="mt-3 grid grid-cols-2 md:grid-cols-4 gap-2 text-sm">
              <div>
                <span class="th-text-tertiary">上级 ID:</span>
                <span class="th-text-primary ml-1 font-mono text-xs">{platform.server_gb_id}</span>
              </div>
              <div>
                <span class="th-text-tertiary">本端 ID:</span>
                <span class="th-text-primary ml-1 font-mono text-xs">{platform.device_gb_id}</span>
              </div>
              <div>
                <span class="th-text-tertiary">本端 IP:</span>
                <span class="th-text-primary ml-1">{platform.device_ip}:{platform.device_port}</span>
              </div>
              <div>
                <span class="th-text-tertiary">平台 ID:</span>
                <span class="th-text-primary ml-1 font-mono text-xs">#{platform.id}</span>
              </div>
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </main>
</div>

{#if showAddDialog}
  <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onclick={() => showAddDialog = false} role="button" tabindex="-1" aria-label="Close dialog">
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
    <div class="th-bg-secondary rounded-xl shadow-xl w-full max-w-lg mx-4 max-h-[90vh] overflow-y-auto" onclick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label="添加上级平台" tabindex="-1">
      <div class="flex items-center justify-between p-4 border-b th-border">
        <h2 class="text-lg font-semibold th-text-primary">添加上级平台</h2>
        <button onclick={() => showAddDialog = false} class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700">
          <X class="w-5 h-5" />
        </button>
      </div>
      <div class="p-4 space-y-4">
        <div>
          <label for="platform-name" class="block text-sm font-medium th-text-secondary mb-1">平台名称</label>
          <input id="platform-name" type="text" bind:value={form.name} class="input w-full" placeholder="上级视频平台" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label for="platform-server-gb-id" class="block text-sm font-medium th-text-secondary mb-1">上级 SIP ID *</label>
            <input id="platform-server-gb-id" type="text" bind:value={form.server_gb_id} class="input w-full" placeholder="34020000002000000002" />
          </div>
          <div>
            <label for="platform-server-ip" class="block text-sm font-medium th-text-secondary mb-1">上级 IP *</label>
            <input id="platform-server-ip" type="text" bind:value={form.server_ip} class="input w-full" placeholder="192.168.1.200" />
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label for="platform-server-port" class="block text-sm font-medium th-text-secondary mb-1">上级端口</label>
            <input id="platform-server-port" type="number" bind:value={form.server_port} class="input w-full" />
          </div>
          <div>
            <label for="platform-transport" class="block text-sm font-medium th-text-secondary mb-1">传输协议</label>
            <select id="platform-transport" bind:value={form.transport} class="input w-full">
              <option value="UDP">UDP</option>
              <option value="TCP">TCP</option>
            </select>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label for="platform-username" class="block text-sm font-medium th-text-secondary mb-1">用户名</label>
            <input id="platform-username" type="text" bind:value={form.username} class="input w-full" placeholder="SIP 认证用户名" />
          </div>
          <div>
            <label for="platform-password" class="block text-sm font-medium th-text-secondary mb-1">密码</label>
            <input id="platform-password" type="password" bind:value={form.password} class="input w-full" placeholder="SIP 认证密码" />
          </div>
        </div>
        <div class="grid grid-cols-3 gap-4">
          <div>
            <label for="platform-expires" class="block text-sm font-medium th-text-secondary mb-1">注册有效期</label>
            <input id="platform-expires" type="number" bind:value={form.expires} class="input w-full" />
          </div>
          <div>
            <label for="platform-keep-timeout" class="block text-sm font-medium th-text-secondary mb-1">心跳间隔</label>
            <input id="platform-keep-timeout" type="number" bind:value={form.keep_timeout} class="input w-full" />
          </div>
          <div>
            <label for="platform-max-timeout" class="block text-sm font-medium th-text-secondary mb-1">超时次数</label>
            <input id="platform-max-timeout" type="number" bind:value={form.max_timeout_count} class="input w-full" />
          </div>
        </div>
        <div class="flex items-center gap-2">
          <input type="checkbox" bind:checked={form.enable} id="enable-platform" class="rounded" />
          <label for="enable-platform" class="text-sm th-text-secondary">立即启用</label>
        </div>
      </div>
      <div class="flex justify-end gap-2 p-4 border-t th-border">
        <button onclick={() => showAddDialog = false} class="btn btn-secondary">取消</button>
        <button onclick={handleAdd} class="btn btn-primary" disabled={saving}>
          {saving ? '保存中...' : '添加'}
        </button>
      </div>
    </div>
  </div>
{/if}
