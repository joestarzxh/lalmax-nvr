# 设备管理页面合并实现说明

## 概述

本实现将摄像头管理功能整合到设备管理页面中，提供统一的设备管理体验。

## 主要变更

### 1. 新增导入

```typescript
import { 
  // 原有导入
  listGB28181Devices, playGB28181Stream, stopGB28181Stream, 
  listStreams, listCameras,
  
  // 新增摄像头管理 API
  deleteCamera, permanentlyDeleteCamera,
  startCamera, stopCamera, updateCamera, 
  pauseRecording, resumeRecording,
  xiaomiDevices, listProtocols, DEFAULT_PROTOCOLS, buildProtocolsMap,
  enableCamera, disableCamera, getHealthStatus, getSnapshotUrl,
  ApiRequestError
} from '$lib/api';

import type { 
  GB28181Device, StreamInfo, Camera, 
  XiaomiDevice, ProtocolInfo, CameraHealth 
} from '$lib/api';

// 新增组件
import CameraCard from '$lib/components/CameraCard.svelte';
import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
import ArchiveConfirmDialog from '$lib/components/ArchiveConfirmDialog.svelte';
import CameraForm from '$lib/components/CameraForm.svelte';
```

### 2. 新增状态变量

```typescript
// 摄像头管理状态
let protocols = $state<ProtocolInfo[]>(DEFAULT_PROTOCOLS);
let protocolsMap = $state<Map<string, ProtocolInfo>>(buildProtocolsMap(DEFAULT_PROTOCOLS));
let healthData = $state<Record<string, CameraHealth>>({});
let pausedCameras = $state<Set<string>>(new Set());

// 表单状态
let showForm = $state(false);
let editingCamera = $state<Camera | null>(null);

// 确认对话框状态
let confirmAction = $state<{ camera: Camera; action: 'stop' | 'restart' } | null>(null);
let archiveConfirm = $state<Camera | null>(null);
let archiveLoading = $state(false);
let permanentDeleteConfirm = $state<Camera | null>(null);
let permanentDeleteLoading = $state(false);

// 小米设备
let xiaomiDeviceList = $state<XiaomiDevice[]>([]);
```

### 3. 新增函数

```typescript
// 加载健康状态
async function loadHealth() { ... }

// 摄像头管理函数
function openAddForm() { ... }
function openEditForm(camera: Camera) { ... }
function handleFormSave() { ... }
function handleFormCancel() { ... }
async function executeConfirmAction() { ... }
async function handleStartCamera(camera: Camera) { ... }
async function handleStopCamera(camera: Camera) { ... }
async function handleRestartCamera(camera: Camera) { ... }
async function handleToggleCamera(camera: Camera) { ... }
async function handleSaveName(camera: Camera, name: string) { ... }
async function handlePauseRecording(camera: Camera) { ... }
async function handleResumeRecording(camera: Camera) { ... }
async function openArchiveConfirm(camera: Camera) { ... }
async function handlePermanentDelete(cameraId: string) { ... }
```

### 4. UI 变更

#### 4.1 顶部操作栏

新增「添加摄像头」按钮：

```svelte
<button onclick={openAddForm} class="btn btn-primary flex items-center gap-2">
  <CameraIcon class="w-4 h-4" />
  添加摄像头
</button>
```

#### 4.2 ONVIF 设备 Tab

使用 `CameraCard` 组件替代原来的简单卡片：

```svelte
{#each onvifCameras as camera (camera.id)}
  <CameraCard
    {camera}
    {protocolsMap}
    health={healthData[camera.id]}
    onedit={openEditForm}
    ondelete={openArchiveConfirm}
    onpermadelete={(camera) => permanentDeleteConfirm = camera}
    onstart={handleStartCamera}
    onstop={handleStopCamera}
    onrestart={handleRestartCamera}
    ontoggle={handleToggleCamera}
    onsaveName={handleSaveName}
    onpause={handlePauseRecording}
    onresume={handleResumeRecording}
    recordingPaused={camera.recording_paused || pausedCameras.has(camera.id)}
  />
{/each}
```

#### 4.3 推流设备 Tab

为已升级为摄像头的推流设备添加管理功能：

```svelte
{#each pushCameras as camera}
  <div class="th-bg-secondary rounded-lg border th-border p-4">
    <!-- 设备信息 -->
    
    <!-- 快照预览 -->
    {#if camera.enabled}
      <div class="mb-3 rounded-lg overflow-hidden aspect-video bg-gray-100">
        <img
          src="{getSnapshotUrl(camera.id)}&_t={Date.now()}"
          alt={camera.name}
          class="w-full h-full object-cover"
          loading="lazy"
        />
      </div>
    {/if}

    <!-- 操作按钮 -->
    <div class="flex items-center justify-between pt-3 border-t th-border">
      <div class="flex items-center gap-2">
        <!-- 录制控制 -->
        {#if camera.status === 'recording'}
          <button onclick={() => handleStopCamera(camera)}>
            <Square size={14} />
          </button>
        {:else}
          <button onclick={() => handleStartCamera(camera)}>
            <Play size={14} />
          </button>
        {/if}
        
        <!-- 编辑 -->
        <button onclick={() => openEditForm(camera)}>
          <Pencil size={14} />
        </button>
      </div>

      <div class="flex items-center gap-2">
        <!-- 实时预览 -->
        <a href="#/live/{camera.id}">
          <Eye size={14} />
        </a>
        
        <!-- 重启 -->
        <button onclick={() => handleRestartCamera(camera)}>
          <RotateCw size={14} />
        </button>
        
        <!-- 归档 -->
        <button onclick={() => openArchiveConfirm(camera)}>
          <Archive size={14} />
        </button>
      </div>
    </div>
  </div>
{/each}
```

#### 4.4 表单集成

在页面中集成 `CameraForm` 组件：

```svelte
{#if showForm}
  <div class="mb-6">
    <CameraForm
      {editingCamera}
      {protocols}
      {protocolsMap}
      {xiaomiDeviceList}
      globalTranscodingEnabled={false}
      h265Available={true}
      onsave={handleFormSave}
      oncancel={handleFormCancel}
    />
  </div>
{/if}
```

#### 4.5 确认对话框

集成确认对话框组件：

```svelte
<!-- 停止/重启确认 -->
{#if confirmAction}
  <ConfirmDialog
    title={confirmAction.action === 'stop' ? t('cameras.stopTitle') : t('cameras.restartTitle')}
    message={confirmAction.action === 'stop'
      ? t('cameras.stopMessage', { name: confirmAction.camera.name })
      : t('cameras.restartMessage', { name: confirmAction.camera.name })}
    confirmText={t('common.confirm')}
    onconfirm={executeConfirmAction}
    oncancel={() => confirmAction = null}
    variant="primary"
  />
{/if}

<!-- 归档确认 -->
{#if archiveConfirm}
  <ArchiveConfirmDialog
    cameraName={archiveConfirm.name}
    recordingCount={0}
    totalSize="..."
    loading={archiveLoading}
    onconfirm={async () => {
      archiveLoading = true;
      try {
        await deleteCamera(archiveConfirm!.id);
        showToast(t('cameras.cameraArchived'), 'success');
        archiveConfirm = null;
        await loadDevices();
      } catch (e) {
        showToast(t('cameras.failedArchive'), 'error');
      } finally {
        archiveLoading = false;
      }
    }}
    oncancel={() => { if (!archiveLoading) archiveConfirm = null; }}
  />
{/if}

<!-- 永久删除确认 -->
{#if permanentDeleteConfirm}
  <ConfirmDialog
    title={t('cameras.action.deletePermanent')}
    message={t('cameras.deletePermanentConfirm', { name: permanentDeleteConfirm.name })}
    confirmText={t('cameras.action.deletePermanent')}
    onconfirm={() => handlePermanentDelete(permanentDeleteConfirm!.id)}
    oncancel={() => { if (!permanentDeleteLoading) permanentDeleteConfirm = null; }}
    variant="danger"
    loading={permanentDeleteLoading}
  />
{/if}
```

### 5. 生命周期

在 `onMount` 中加载摄像头相关数据：

```typescript
onMount(() => {
  loadDevices();
  loadHealth();
  
  // 加载协议
  listProtocols().then(list => {
    if (list && list.length > 0) {
      protocols = list;
      protocolsMap = buildProtocolsMap(list);
    }
  }).catch(e => console.warn('Failed to load protocols:', e));

  // 加载小米设备
  xiaomiDevices().then(res => {
    if (res.devices && res.devices.length > 0) {
      xiaomiDeviceList = res.devices;
    }
  }).catch(e => console.warn('Xiaomi not authenticated:', e));

  // 定期刷新健康状态
  const healthInterval = window.setInterval(() => loadHealth(), 30000);
  return () => clearInterval(healthInterval);
});
```

## 功能对比

### 原设备管理页面

| 功能 | ONVIF | GB28181 | 小米 | 推流 |
|------|-------|---------|------|------|
| 基本信息 | ✅ | ✅ | ✅ | ✅ |
| 状态显示 | ✅ | ✅ | ✅ | ✅ |
| 快照预览 | ❌ | ❌ | ❌ | ❌ |
| 录制控制 | ❌ | ❌ | ❌ | ❌ |
| 编辑配置 | ❌ | ❌ | ❌ | ❌ |
| 实时预览 | ❌ | ❌ | ❌ | ❌ |
| 归档/删除 | ❌ | ❌ | ❌ | ❌ |
| 健康监控 | ❌ | ❌ | ❌ | ❌ |

### 合并后的设备管理页面

| 功能 | ONVIF | GB28181 | 小米 | 推流 |
|------|-------|---------|------|------|
| 基本信息 | ✅ | ✅ | ✅ | ✅ |
| 状态显示 | ✅ | ✅ | ✅ | ✅ |
| 快照预览 | ✅ | ❌ | ❌ | ✅ |
| 录制控制 | ✅ | ❌ | ❌ | ✅ |
| 编辑配置 | ✅ | ❌ | ❌ | ✅ |
| 实时预览 | ✅ | ❌ | ❌ | ✅ |
| 归档/删除 | ✅ | ❌ | ❌ | ✅ |
| 健康监控 | ✅ | ❌ | ❌ | ❌ |

## 测试用例

### 1. ONVIF 设备管理

**测试步骤：**
1. 切换到 ONVIF 设备 tab
2. 验证摄像头卡片显示快照
3. 点击播放按钮启动录制
4. 点击暂停按钮暂停录制
5. 点击恢复按钮恢复录制
6. 点击停止按钮停止录制
7. 点击编辑按钮打开编辑表单
8. 点击实时预览跳转到 LiveView
9. 点击归档按钮归档摄像头
10. 点击删除按钮永久删除摄像头

**预期结果：**
- 所有操作正常执行
- 状态实时更新
- 提示信息正确显示

### 2. 推流设备管理

**测试步骤：**
1. 切换到推流设备 tab
2. 验证已升级的摄像头显示快照
3. 验证操作按钮可用
4. 测试录制控制功能
5. 测试编辑功能
6. 测试实时预览跳转

**预期结果：**
- 推流设备正确分类显示
- 升级的摄像头支持完整管理功能
- 未升级的推流只显示基本信息

### 3. 设备发现

**测试步骤：**
1. 点击「扫描设备」按钮
2. 等待扫描完成
3. 选择发现的设备添加
4. 验证设备列表更新

**预期结果：**
- 扫描功能正常
- 添加设备成功
- 列表自动刷新

### 4. 添加/编辑摄像头

**测试步骤：**
1. 点击「添加摄像头」按钮
2. 填写摄像头信息
3. 保存摄像头
4. 点击编辑按钮修改摄像头
5. 保存修改

**预期结果：**
- 表单正常显示
- 验证功能正常
- 保存成功后列表更新

## 性能优化

### 1. 快照加载优化

```typescript
// 使用懒加载
<img loading="lazy" ... />

// 添加缓存破坏参数
src="{getSnapshotUrl(camera.id)}&_t={Date.now()}"

// 错误处理
onerror={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
```

### 2. 健康状态轮询优化

```typescript
// 使用合理的轮询间隔（30秒）
const healthInterval = window.setInterval(() => loadHealth(), 30000);

// 组件卸载时清理
return () => clearInterval(healthInterval);
```

### 3. 状态管理优化

```typescript
// 使用 Set 提高查找效率
let pausedCameras = $state<Set<string>>(new Set());

// 使用 derived 减少重复计算
let canDiscover = $derived(activeTab === 'onvif' || activeTab === 'xiaomi');
```

## 已知限制

1. **GB28181 设备** - 暂不支持快照和录制控制
2. **小米设备** - 暂未实现设备发现和管理
3. **批量操作** - 暂不支持批量选择和操作

## 后续优化建议

1. **GB28181 设备增强**
   - 为 GB28181 通道添加快照功能
   - 支持通道级别的录制控制

2. **小米设备集成**
   - 实现小米设备的发现和管理
   - 支持小米设备的快照和录制

3. **批量操作**
   - 支持批量选择设备
   - 支持批量启动/停止录制

4. **设备分组**
   - 支持按位置/类型分组
   - 支持分组级别的操作

5. **性能优化**
   - 实现快照缓存
   - 优化轮询策略
   - 添加虚拟滚动支持大量设备

## 总结

本实现成功将摄像头管理功能整合到设备管理页面中，提供了统一的设备管理体验。通过复用现有组件，最小化了代码重复，同时保持了功能的完整性和用户体验的一致性。
