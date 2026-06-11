<script lang="ts">
  import { onMount } from 'svelte';
  import { listUsers, createUser, updateUser, deleteUser } from '$lib/api';
  import type { User, CreateUserRequest, UpdateUserRequest } from '$lib/api';
  import { t } from '$lib/i18n';
  import { showToast } from '$lib/toast';
  import { formatDate } from '$lib/format';
  import { 
    Users as UsersIcon, Plus, Pencil, Trash2, RefreshCw, 
    Shield, User as UserIcon, Check, X, Eye, EyeOff, Copy, Key
  } from 'lucide-svelte';
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';

  let users = $state<User[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Form state
  let showForm = $state(false);
  let editingUser = $state<User | null>(null);
  let formUsername = $state('');
  let formPassword = $state('');
  let formDisplayName = $state('');
  let formRole = $state<'super_admin' | 'user'>('user');
  let formEnabled = $state(true);
  let showPassword = $state(false);
  let saving = $state(false);

  // Credential display state
  let showCredentialDialog = $state(false);
  let credentialUsername = $state('');
  let credentialPassword = $state('');

  // Delete confirmation
  let deleteConfirm = $state<User | null>(null);
  let deleteLoading = $state(false);

  async function loadUsers() {
    loading = true;
    error = '';
    try {
      users = await listUsers();
    } catch (e) {
      error = e instanceof Error ? e.message : '加载用户列表失败';
    } finally {
      loading = false;
    }
  }

  function generatePassword(): string {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*';
    let password = '';
    for (let i = 0; i < 16; i++) {
      password += chars.charAt(Math.floor(Math.random() * chars.length));
    }
    return password;
  }

  function openAddForm() {
    editingUser = null;
    formUsername = '';
    formPassword = generatePassword();
    formDisplayName = '';
    formRole = 'user';
    formEnabled = true;
    showPassword = true;
    showForm = true;
  }

  function openEditForm(user: User) {
    editingUser = user;
    formUsername = user.username;
    formPassword = '';
    formDisplayName = user.display_name;
    formRole = user.role;
    formEnabled = user.enabled;
    showPassword = false;
    showForm = true;
  }

  function closeForm() {
    showForm = false;
    editingUser = null;
  }

  async function handleSave() {
    if (!formUsername.trim()) {
      showToast('用户名不能为空', 'error');
      return;
    }

    if (!editingUser && !formPassword) {
      showToast('密码不能为空', 'error');
      return;
    }

    if (formPassword && formPassword.length < 8) {
      showToast('密码长度至少8位', 'error');
      return;
    }

    saving = true;
    try {
      if (editingUser) {
        const req: UpdateUserRequest = {};
        if (formPassword) req.password = formPassword;
        if (formRole !== editingUser.role) req.role = formRole;
        if (formDisplayName !== editingUser.display_name) req.display_name = formDisplayName;
        if (formEnabled !== editingUser.enabled) req.enabled = formEnabled;
        
        await updateUser(editingUser.id, req);
        showToast('用户更新成功', 'success');
        
        // If password was changed, show credential dialog
        if (formPassword) {
          credentialUsername = editingUser.username;
          credentialPassword = formPassword;
          showCredentialDialog = true;
        }
      } else {
        const req: CreateUserRequest = {
          username: formUsername.trim(),
          password: formPassword,
          role: formRole,
          display_name: formDisplayName.trim() || formUsername.trim(),
        };
        await createUser(req);
        showToast('用户创建成功', 'success');
        
        // Show credential dialog for new user
        credentialUsername = formUsername.trim();
        credentialPassword = formPassword;
        showCredentialDialog = true;
      }
      closeForm();
      await loadUsers();
    } catch (e) {
      showToast(e instanceof Error ? e.message : '保存失败', 'error');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!deleteConfirm) return;
    deleteLoading = true;
    try {
      await deleteUser(deleteConfirm.id);
      showToast('用户删除成功', 'success');
      deleteConfirm = null;
      await loadUsers();
    } catch (e) {
      showToast(e instanceof Error ? e.message : '删除失败', 'error');
    } finally {
      deleteLoading = false;
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => {
      showToast('已复制到剪贴板', 'success');
    }).catch(() => {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      textArea.value = text;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      showToast('已复制到剪贴板', 'success');
    });
  }

  function copyCredentials() {
    const text = `用户名: ${credentialUsername}\n密码: ${credentialPassword}`;
    copyToClipboard(text);
  }

  function getRoleBadge(role: string) {
    return role === 'super_admin' 
      ? { label: '超级管理员', class: 'bg-purple-100 text-purple-800' }
      : { label: '普通用户', class: 'bg-blue-100 text-blue-800' };
  }

  function getStatusBadge(enabled: boolean) {
    return enabled 
      ? { label: '启用', class: 'bg-green-100 text-green-800' }
      : { label: '禁用', class: 'bg-red-100 text-red-800' };
  }

  onMount(() => {
    loadUsers();
  });
</script>

<div class="min-h-screen th-bg-primary">
  <main class="mx-auto px-3 sm:px-4 lg:px-6 py-4 sm:py-6" style="max-width: 1200px;">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold th-text-primary">用户管理</h1>
        <p class="text-sm th-text-secondary mt-1">管理系统用户和权限</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          onclick={openAddForm}
          class="btn btn-primary flex items-center gap-2"
        >
          <Plus class="w-4 h-4" />
          添加用户
        </button>
        <button
          onclick={loadUsers}
          class="btn btn-secondary flex items-center gap-2"
          disabled={loading}
        >
          <RefreshCw class="w-4 h-4 {loading ? 'animate-spin' : ''}" />
          刷新
        </button>
      </div>
    </div>

    <!-- Error display -->
    {#if error}
      <div class="mb-4 p-4 bg-red-50 border border-red-200 rounded-lg">
        <p class="text-red-600">{error}</p>
      </div>
    {/if}

    <!-- User Form -->
    {#if showForm}
      <div class="card border th-border p-6 mb-6">
        <h3 class="text-lg font-semibold th-text-primary mb-4">
          {editingUser ? '编辑用户' : '添加用户'}
        </h3>
        
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <!-- Username -->
          <div>
            <label for="user-username" class="input-label">用户名 *</label>
            <input
              id="user-username"
              type="text"
              class="input mt-1 w-full"
              bind:value={formUsername}
              disabled={!!editingUser}
              placeholder="请输入用户名"
            />
          </div>

          <!-- Display Name -->
          <div>
            <label for="user-display-name" class="input-label">显示名称</label>
            <input
              id="user-display-name"
              type="text"
              class="input mt-1 w-full"
              bind:value={formDisplayName}
              placeholder="请输入显示名称"
            />
          </div>

          <!-- Password -->
          <div>
            <label for="user-password" class="input-label">
              {editingUser ? '新密码（留空不修改）' : '密码 *'}
            </label>
            <div class="relative">
              <input
                id="user-password"
                type={showPassword ? 'text' : 'password'}
                class="input mt-1 w-full pr-20"
                bind:value={formPassword}
                placeholder={editingUser ? '留空不修改' : '请输入密码（至少8位）'}
              />
              <div class="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1">
                <button
                  type="button"
                  class="p-1 th-text-tertiary hover:th-text-primary"
                  onclick={() => showPassword = !showPassword}
                  title={showPassword ? '隐藏密码' : '显示密码'}
                >
                  {#if showPassword}
                    <EyeOff size={16} />
                  {:else}
                    <Eye size={16} />
                  {/if}
                </button>
                {#if !editingUser}
                  <button
                    type="button"
                    class="p-1 th-text-tertiary hover:th-text-primary"
                    onclick={() => formPassword = generatePassword()}
                    title="生成随机密码"
                  >
                    <Key size={16} />
                  </button>
                {/if}
              </div>
            </div>
          </div>

          <!-- Role -->
          <div>
            <label for="user-role" class="input-label">角色</label>
            <select
              id="user-role"
              class="input mt-1 w-full"
              bind:value={formRole}
            >
              <option value="user">普通用户</option>
              <option value="super_admin">超级管理员</option>
            </select>
          </div>

          <!-- Enabled -->
          <div class="flex items-center gap-2 mt-6">
            <input
              id="user-enabled"
              type="checkbox"
              class="checkbox"
              bind:checked={formEnabled}
            />
            <label for="user-enabled" class="text-sm th-text-primary cursor-pointer">
              启用此用户
            </label>
          </div>
        </div>

        <!-- Actions -->
        <div class="flex justify-end gap-3 mt-6 pt-4 border-t th-border">
          <button onclick={closeForm} class="btn btn-secondary">
            取消
          </button>
          <button 
            onclick={handleSave} 
            class="btn btn-primary"
            disabled={saving}
          >
            {#if saving}
              <RefreshCw class="w-4 h-4 animate-spin mr-1" />
            {/if}
            {editingUser ? '保存' : '创建'}
          </button>
        </div>
      </div>
    {/if}

    <!-- Users List -->
    {#if loading}
      <div class="flex items-center justify-center py-12">
        <RefreshCw class="w-6 h-6 animate-spin th-text-secondary" />
        <span class="ml-2 th-text-secondary">加载中...</span>
      </div>
    {:else if users.length === 0}
      <div class="flex flex-col items-center justify-center py-12 th-bg-secondary rounded-lg">
        <UsersIcon class="w-12 h-12 th-text-tertiary mb-4" />
        <p class="text-lg th-text-secondary">暂无用户</p>
        <p class="text-sm th-text-tertiary mt-1">点击上方「添加用户」按钮创建第一个用户</p>
      </div>
    {:else}
      <div class="th-bg-secondary rounded-lg border th-border overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b th-border bg-gray-50 dark:bg-gray-800">
                <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">用户</th>
                <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">角色</th>
                <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">状态</th>
                <th class="px-4 py-3 text-left text-sm font-medium th-text-secondary">创建时间</th>
                <th class="px-4 py-3 text-right text-sm font-medium th-text-secondary">操作</th>
              </tr>
            </thead>
            <tbody>
              {#each users as user (user.id)}
                {@const roleBadge = getRoleBadge(user.role)}
                {@const statusBadge = getStatusBadge(user.enabled)}
                <tr class="border-b th-border hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors">
                  <td class="px-4 py-3">
                    <div class="flex items-center gap-3">
                      <div class="p-2 rounded-full th-bg-tertiary">
                        {#if user.role === 'super_admin'}
                          <Shield size={16} class="text-purple-500" />
                        {:else}
                          <UserIcon size={16} class="th-text-secondary" />
                        {/if}
                      </div>
                      <div>
                        <div class="font-medium th-text-primary">{user.username}</div>
                        {#if user.display_name && user.display_name !== user.username}
                          <div class="text-sm th-text-tertiary">{user.display_name}</div>
                        {/if}
                      </div>
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <span class="px-2 py-1 text-xs rounded-full {roleBadge.class}">
                      {roleBadge.label}
                    </span>
                  </td>
                  <td class="px-4 py-3">
                    <span class="px-2 py-1 text-xs rounded-full {statusBadge.class}">
                      {statusBadge.label}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-sm th-text-secondary">
                    {formatDate(user.created_at)}
                  </td>
                  <td class="px-4 py-3 text-right">
                    <div class="flex justify-end gap-1">
                      <button
                        class="btn btn-ghost px-2 py-1.5 text-sm"
                        onclick={() => openEditForm(user)}
                        title="编辑"
                      >
                        <Pencil size={16} />
                      </button>
                      <button
                        class="btn btn-ghost px-2 py-1.5 text-sm text-red-500"
                        onclick={() => deleteConfirm = user}
                        title="删除"
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
      </div>
    {/if}
  </main>

  <!-- Credential Display Dialog -->
  {#if showCredentialDialog}
    <div class="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50" role="dialog" aria-modal="true">
      <div class="card max-w-md w-full p-6">
        <div class="flex items-center gap-3 mb-4">
          <div class="p-2 rounded-full bg-green-100">
            <Check size={20} class="text-green-600" />
          </div>
          <h3 class="text-lg font-semibold th-text-primary">用户凭据</h3>
        </div>
        
        <p class="th-text-secondary mb-4 text-sm">
          请保存以下用户凭据，密码仅在此处显示一次：
        </p>

        <div class="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 mb-4">
          <div class="mb-3">
            <span class="text-xs th-text-tertiary block mb-1">用户名</span>
            <div class="flex items-center justify-between">
              <code class="font-mono text-sm th-text-primary">{credentialUsername}</code>
              <button 
                class="btn btn-ghost px-2 py-1 text-sm"
                onclick={() => copyToClipboard(credentialUsername)}
                title="复制用户名"
              >
                <Copy size={14} />
              </button>
            </div>
          </div>
          <div>
            <span class="text-xs th-text-tertiary block mb-1">密码</span>
            <div class="flex items-center justify-between">
              <code class="font-mono text-sm th-text-primary break-all">{credentialPassword}</code>
              <button 
                class="btn btn-ghost px-2 py-1 text-sm shrink-0"
                onclick={() => copyToClipboard(credentialPassword)}
                title="复制密码"
              >
                <Copy size={14} />
              </button>
            </div>
          </div>
        </div>

        <div class="flex gap-3 justify-end">
          <button 
            onclick={copyCredentials} 
            class="btn btn-secondary flex items-center gap-2"
          >
            <Copy size={16} />
            复制全部
          </button>
          <button 
            onclick={() => showCredentialDialog = false} 
            class="btn btn-primary"
          >
            我已保存
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Delete Confirmation Dialog -->
  {#if deleteConfirm}
    <ConfirmDialog
      title="删除用户"
      message="确定要删除用户「{deleteConfirm.username}」吗？此操作不可撤销。"
      confirmText="删除"
      onconfirm={handleDelete}
      oncancel={() => deleteConfirm = null}
      variant="danger"
      loading={deleteLoading}
    />
  {/if}
</div>
