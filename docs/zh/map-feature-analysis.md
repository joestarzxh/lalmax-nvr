# 地图功能分析与集成方案

## 1. 参考项目地图功能分析

### 1.1 技术架构

参考项目 `royi-qos-nvr` 的地图功能采用以下技术栈：

| 组件 | 技术 | 说明 |
|------|------|------|
| 地图引擎 | 天地图API | 国家地理信息公共服务平台 |
| 前端框架 | Vue 3 + Element Plus | 响应式UI |
| 数据存储 | MySQL | 设备经纬度字段 |
| 通信协议 | REST API | 前后端交互 |

### 1.2 核心功能模块

```
┌─────────────────────────────────────────────────────────────┐
│                      地图功能模块                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  地图展示   │  │  设备标记   │  │  信息窗口   │         │
│  │  Map View   │  │   Marker    │  │ InfoWindow  │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │  位置编辑   │  │  视频播放   │  │  云台控制   │         │
│  │  Position   │  │   Player    │  │    PTZ      │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### 1.3 数据模型

```sql
-- 设备表（包含位置信息）
CREATE TABLE `qs_device` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `device_name` varchar(255) DEFAULT NULL,
  `device_code` varchar(255) DEFAULT NULL,
  `ip_address` varchar(50) DEFAULT NULL,
  `longitude` decimal(12,8) DEFAULT NULL,  -- 经度
  `latitude` decimal(12,8) DEFAULT NULL,   -- 纬度
  `address` varchar(500) DEFAULT NULL,      -- 地址描述
  `manufacturer` varchar(100) DEFAULT NULL, -- 厂商
  `ptz_type` int DEFAULT NULL,             -- 云台类型
  `status` varchar(10) DEFAULT 'OFF',      -- 在线状态
  -- ... 其他字段
  PRIMARY KEY (`id`)
);

-- 区域表（行政区划）
CREATE TABLE `qs_region` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` varchar(255) DEFAULT NULL,
  `parent_id` bigint DEFAULT NULL,
  `code` varchar(50) DEFAULT NULL,  -- 行政区划代码
  `level` int DEFAULT NULL,         -- 层级
  `longitude` decimal(12,8) DEFAULT NULL,
  `latitude` decimal(12,8) DEFAULT NULL,
  PRIMARY KEY (`id`)
);
```

### 1.4 前端实现分析

#### 天地图API使用

```javascript
// 1. 初始化地图
map = new T.Map('Map', {
  projection: 'EPSG:4326'  // WGS84坐标系
});
let center = new T.LngLat(lng, lat, zoom);
map.centerAndZoom(center, zoom);

// 2. 添加控件
map.addControl(new T.Control.Zoom());      // 缩放控件
map.addControl(new T.Control.Scale());     // 比例尺
map.addControl(new T.Control.OverviewMap()); // 鹰眼图
map.addControl(new T.Control.MapType());   // 地图类型切换

// 3. 创建标记
var icon = new T.Icon({
  iconUrl: "marker.png",
  iconSize: new T.Point(30, 27),
  iconAnchor: new T.Point(10, 25)
});
var marker = new T.Marker(new T.LngLat(lng, lat), {icon: icon});
map.addOverLay(marker);

// 4. 信息窗口
var infoWindow = new T.InfoWindow();
infoWindow.setContent(htmlContent);
marker.openInfoWindow(infoWindow);

// 5. 点击事件
marker.addEventListener("click", function(e) {
  // 处理点击事件
});

// 6. 右键菜单
var menu = new T.ContextMenu({width: 100});
menu.addItem(new T.MenuItem('放大', function() { map.zoomIn() }));
map.addContextMenu(menu);
```

#### 设备类型图标

```javascript
// 根据设备类型显示不同图标
if (ptzType === 1) {
  // 球机
  icon = new T.Icon({
    iconUrl: "球机.png",
    iconSize: new T.Point(30, 27)
  });
} else if (ptzType === 2 || ptzType === 5) {
  // 半球或遥控半球
  icon = new T.Icon({
    iconUrl: "半球.png",
    iconSize: new T.Point(30, 27)
  });
} else if (ptzType === 3 || ptzType === 4) {
  // 固定枪机或遥控枪机
  icon = new T.Icon({
    iconUrl: "枪机.png",
    iconSize: new T.Point(30, 27)
  });
} else {
  // 默认标记
  icon = new T.Icon({
    iconUrl: "http://api.tianditu.gov.cn/img/map/markerA.png",
    iconSize: new T.Point(20, 27)
  });
}
```

### 1.5 后端API接口

```java
// 设备位置更新
@PutMapping("/qs/device")
public AjaxResult updateDevice(@RequestBody QsDevice device) {
    // 更新设备经纬度
    return toAjax(deviceService.updateDevice(device));
}

// 获取区域设备列表（包含位置信息）
@GetMapping("/qs/region/device/list")
public AjaxResult queryRegionForDevice() {
    List<QsDevice> list = deviceService.queryRegionForDevice();
    return AjaxResult.success(list);
}
```

## 2. lalmax-nvr 集成方案

### 2.1 技术选型

考虑到 lalmax-nvr 使用 Svelte 5 + Tailwind CSS，推荐以下方案：

| 方案 | 地图引擎 | 优点 | 缺点 |
|------|---------|------|------|
| 方案A | Leaflet + OpenStreetMap | 开源免费、社区活跃 | 需要自建瓦片服务 |
| 方案B | 天地图API | 国内数据全、免费 | 需要申请API Key |
| 方案C | 高德/百度地图 | 国内体验好、功能丰富 | 商用需授权 |

**推荐方案：Leaflet + 天地图瓦片**

- 使用 Leaflet 作为地图引擎（开源、轻量）
- 使用天地图瓦片服务（免费、国内数据全）
- 无需API Key限制

### 2.2 项目结构设计

```
lalmax-nvr/
├── web/
│   ├── src/
│   │   ├── routes/
│   │   │   └── Map.svelte              # 地图页面
│   │   │
│   │   ├── components/
│   │   │   └── map/
│   │   │       ├── MapView.svelte       # 地图视图组件
│   │   │       ├── MapMarker.svelte     # 设备标记组件
│   │   │       ├── MapInfoWindow.svelte # 信息窗口组件
│   │   │       └── MapControls.svelte   # 地图控件组件
│   │   │
│   │   └── lib/
│   │       └── map/
│   │           ├── map.ts              # 地图核心逻辑
│   │           ├── marker.ts           # 标记管理
│   │           └── utils.ts            # 工具函数
│   │
│   └── package.json                    # 添加leaflet依赖
│
└── internal/
    └── api/
        └── map.go                      # 地图相关API
```

### 2.3 数据模型扩展

#### 数据库Schema扩展

```sql
-- 扩展cameras表，添加位置字段
ALTER TABLE cameras ADD COLUMN longitude REAL;
ALTER TABLE cameras ADD COLUMN latitude REAL;
ALTER TABLE cameras ADD COLUMN address TEXT;
ALTER TABLE cameras ADD COLUMN ptz_type INTEGER DEFAULT 0;
ALTER TABLE cameras ADD COLUMN manufacturer TEXT;

-- 创建设备位置历史表（可选）
CREATE TABLE device_location_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    longitude REAL NOT NULL,
    latitude REAL NOT NULL,
    recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES cameras(id)
);
```

#### Go模型扩展

```go
// internal/model/camera.go
type Camera struct {
    // ... 现有字段
    
    // 地图相关字段
    Longitude    *float64 `json:"longitude,omitempty"`
    Latitude     *float64 `json:"latitude,omitempty"`
    Address      string   `json:"address,omitempty"`
    PTZType      int      `json:"ptzType"`      // 0:无 1:球机 2:半球 3:枪机
    Manufacturer string   `json:"manufacturer,omitempty"`
}
```

### 2.4 前端实现

#### 安装依赖

```bash
cd web
npm install leaflet @types/leaflet
```

#### 地图页面组件

```svelte
<!-- src/routes/Map.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { cameras, fetchCameras } from '$lib/stores/cameras';
  import MapView from '$lib/components/map/MapView.svelte';
  import MapControls from '$lib/components/map/MapControls.svelte';
  import MapInfoWindow from '$lib/components/map/MapInfoWindow.svelte';
  
  let mapContainer;
  let map;
  let selectedCamera = null;
  let markers = [];
  
  // 地图配置
  const defaultCenter = [39.9042, 116.4074]; // 北京
  const defaultZoom = 12;
  
  onMount(async () => {
    // 加载摄像头数据
    await fetchCameras();
    
    // 初始化地图
    initMap();
    
    // 添加设备标记
    addCameraMarkers();
  });
  
  onDestroy(() => {
    if (map) {
      map.remove();
    }
  });
  
  function initMap() {
    // 动态导入leaflet（避免SSR问题）
    import('leaflet').then((L) => {
      map = L.map(mapContainer).setView(defaultCenter, defaultZoom);
      
      // 添加天地图瓦片图层
      L.tileLayer('https://t{s}.tianditu.gov.cn/DataServer?T=vec_w&X={x}&Y={y}&L={z}&tk=YOUR_TOKEN', {
        subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
        attribution: '&copy; 天地图'
      }).addTo(map);
      
      // 添加注记图层
      L.tileLayer('https://t{s}.tianditu.gov.cn/DataServer?T=cva_w&X={x}&Y={y}&L={z}&tk=YOUR_TOKEN', {
        subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
        attribution: '&copy; 天地图'
      }).addTo(map);
    });
  }
  
  function addCameraMarkers() {
    if (!map || !$cameras) return;
    
    import('leaflet').then((L) => {
      // 清除现有标记
      markers.forEach(m => m.remove());
      markers = [];
      
      $cameras.forEach(camera => {
        if (camera.latitude && camera.longitude) {
          // 创建自定义图标
          const icon = createCameraIcon(camera.ptzType);
          
          // 创建标记
          const marker = L.marker([camera.latitude, camera.longitude], { icon })
            .addTo(map);
          
          // 绑定点击事件
          marker.on('click', () => {
            selectedCamera = camera;
            map.flyTo([camera.latitude, camera.longitude], 15);
          });
          
          // 绑定弹出窗口
          marker.bindPopup(createPopupContent(camera));
          
          markers.push(marker);
        }
      });
    });
  }
  
  function createCameraIcon(ptzType) {
    const iconMap = {
      1: '/icons/ball-camera.png',    // 球机
      2: '/icons/dome-camera.png',    // 半球
      3: '/icons/bullet-camera.png',  // 枪机
      5: '/icons/dome-camera.png',    // 遥控半球
    };
    
    const iconUrl = iconMap[ptzType] || '/icons/default-camera.png';
    
    return L.icon({
      iconUrl,
      iconSize: [30, 30],
      iconAnchor: [15, 30],
      popupAnchor: [0, -30]
    });
  }
  
  function createPopupContent(camera) {
    const statusColor = camera.status === 'ON' ? '#52c41a' : '#ff4d4f';
    const statusText = camera.status === 'ON' ? '在线' : '离线';
    
    return `
      <div class="camera-popup">
        <div class="popup-header">
          <span class="status-dot" style="background: ${statusColor}"></span>
          <strong>${camera.name}</strong>
        </div>
        <div class="popup-info">
          <p><span>IP:</span> ${camera.url || '未知'}</p>
          <p><span>厂商:</span> ${camera.manufacturer || '未知'}</p>
          <p><span>地址:</span> ${camera.address || '未知'}</p>
          <p><span>状态:</span> <span style="color: ${statusColor}">${statusText}</span></p>
        </div>
        <div class="popup-actions">
          <button onclick="playCamera('${camera.id}')" class="btn-play">
            ▶ 播放
          </button>
          <button onclick="editPosition('${camera.id}')" class="btn-position">
            📍 位置
          </button>
        </div>
      </div>
    `;
  }
  
  // 监听摄像头数据变化
  $: if ($cameras && map) {
    addCameraMarkers();
  }
</script>

<div class="map-page">
  <div class="map-sidebar">
    <MapControls 
      cameras={$cameras}
      on:selectCamera={(e) => {
        selectedCamera = e.detail;
        map.flyTo([e.detail.latitude, e.detail.longitude], 15);
      }}
    />
  </div>
  
  <div class="map-container" bind:this={mapContainer}></div>
  
  {#if selectedCamera}
    <MapInfoWindow 
      camera={selectedCamera}
      on:close={() => selectedCamera = null}
      on:play={(e) => handlePlay(e.detail)}
      on:editPosition={(e) => handleEditPosition(e.detail)}
    />
  {/if}
</div>

<style>
  .map-page {
    display: flex;
    height: calc(100vh - 64px);
  }
  
  .map-sidebar {
    width: 300px;
    background: white;
    border-right: 1px solid #e5e7eb;
    overflow-y: auto;
  }
  
  .map-container {
    flex: 1;
    height: 100%;
  }
  
  :global(.camera-popup) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    font-size: 13px;
    min-width: 200px;
  }
  
  :global(.popup-header) {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 8px;
    border-bottom: 1px solid #eee;
    margin-bottom: 8px;
  }
  
  :global(.status-dot) {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }
  
  :global(.popup-info p) {
    margin: 4px 0;
    display: flex;
    justify-content: space-between;
  }
  
  :global(.popup-info span) {
    color: #888;
  }
  
  :global(.popup-actions) {
    display: flex;
    gap: 8px;
    margin-top: 12px;
    justify-content: flex-end;
  }
  
  :global(.btn-play) {
    padding: 4px 12px;
    background: #409EFF;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  
  :global(.btn-position) {
    padding: 4px 12px;
    background: #f0f0f0;
    color: #333;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
</style>
```

#### 地图视图组件

```svelte
<!-- src/lib/components/map/MapView.svelte -->
<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { browser } from '$app/environment';
  
  export let center = [39.9042, 116.4074];
  export let zoom = 12;
  export let cameras = [];
  
  const dispatch = createEventDispatcher();
  
  let mapContainer;
  let map;
  let L;
  let markers = [];
  
  onMount(async () => {
    if (!browser) return;
    
    // 动态导入leaflet
    const leaflet = await import('leaflet');
    L = leaflet.default;
    
    // 导入leaflet CSS
    await import('leaflet/dist/leaflet.css');
    
    initMap();
  });
  
  onDestroy(() => {
    if (map) {
      map.remove();
      map = null;
    }
  });
  
  function initMap() {
    map = L.map(mapContainer).setView(center, zoom);
    
    // 天地图矢量底图
    L.tileLayer('https://t{s}.tianditu.gov.cn/DataServer?T=vec_w&X={x}&Y={y}&L={z}&tk={token}', {
      subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
      maxZoom: 18
    }).addTo(map);
    
    // 天地图注记图层
    L.tileLayer('https://t{s}.tianditu.gov.cn/DataServer?T=cva_w&X={x}&Y={y}&L={z}&tk={token}', {
      subdomains: ['0', '1', '2', '3', '4', '5', '6', '7'],
      maxZoom: 18
    }).addTo(map);
    
    // 添加控件
    L.control.zoom({ position: 'topright' }).addTo(map);
    L.control.scale({ position: 'bottomright' }).addTo(map);
    
    // 地图点击事件
    map.on('click', (e) => {
      dispatch('mapClick', {
        lat: e.latlng.lat,
        lng: e.latlng.lng
      });
    });
  }
  
  // 更新标记
  export function updateMarkers(cameraList) {
    if (!map || !L) return;
    
    // 清除现有标记
    markers.forEach(m => m.remove());
    markers = [];
    
    cameraList.forEach(camera => {
      if (camera.latitude && camera.longitude) {
        addMarker(camera);
      }
    });
  }
  
  function addMarker(camera) {
    const icon = getCameraIcon(camera.ptzType);
    
    const marker = L.marker([camera.latitude, camera.longitude], { icon })
      .addTo(map);
    
    marker.cameraId = camera.id;
    
    marker.on('click', () => {
      dispatch('cameraClick', camera);
    });
    
    markers.push(marker);
  }
  
  function getCameraIcon(ptzType) {
    const iconUrls = {
      1: '/icons/ball-camera.png',
      2: '/icons/dome-camera.png',
      3: '/icons/bullet-camera.png',
      5: '/icons/dome-camera.png'
    };
    
    return L.icon({
      iconUrl: iconUrls[ptzType] || '/icons/default-camera.png',
      iconSize: [30, 30],
      iconAnchor: [15, 30],
      popupAnchor: [0, -30]
    });
  }
  
  // 飞到指定位置
  export function flyTo(lat, lng, zoom = 15) {
    if (map) {
      map.flyTo([lat, lng], zoom);
    }
  }
  
  // 显示位置选择模式
  export function enableLocationPicker() {
    if (!map) return;
    
    map.getContainer().style.cursor = 'crosshair';
    
    const clickHandler = (e) => {
      map.getContainer().style.cursor = '';
      map.off('click', clickHandler);
      
      dispatch('locationSelected', {
        lat: e.latlng.lat,
        lng: e.latlng.lng
      });
    };
    
    map.on('click', clickHandler);
  }
  
  // 监听cameras变化
  $: if (map && cameras.length > 0) {
    updateMarkers(cameras);
  }
</script>

<div bind:this={mapContainer} class="map-container"></div>

<style>
  .map-container {
    width: 100%;
    height: 100%;
    min-height: 400px;
  }
  
  :global(.leaflet-popup-content) {
    margin: 12px 16px;
  }
</style>
```

#### 位置编辑组件

```svelte
<!-- src/lib/components/map/LocationPicker.svelte -->
<script>
  import { createEventDispatcher } from 'svelte';
  import MapView from './MapView.svelte';
  
  export let latitude = null;
  export let longitude = null;
  export let show = false;
  
  const dispatch = createEventDispatcher();
  
  let mapComponent;
  let selectedLat = latitude;
  let selectedLng = longitude;
  
  function handleLocationSelected(event) {
    selectedLat = event.detail.lat;
    selectedLng = event.detail.lng;
  }
  
  function handleConfirm() {
    if (selectedLat && selectedLng) {
      dispatch('confirm', {
        latitude: selectedLat,
        longitude: selectedLng
      });
      show = false;
    }
  }
  
  function handleCancel() {
    show = false;
    dispatch('cancel');
  }
  
  function handleOpen() {
    if (mapComponent && latitude && longitude) {
      mapComponent.flyTo(latitude, longitude, 15);
    }
  }
</script>

{#if show}
  <div class="modal-overlay" on:click|self={handleCancel}>
    <div class="modal-content">
      <div class="modal-header">
        <h3>选择设备位置</h3>
        <button class="close-btn" on:click={handleCancel}>&times;</button>
      </div>
      
      <div class="modal-body">
        <div class="map-wrapper">
          <MapView 
            bind:this={mapComponent}
            center={latitude && longitude ? [latitude, longitude] : [39.9042, 116.4074]}
            zoom={latitude ? 15 : 12}
            on:locationSelected={handleLocationSelected}
          />
        </div>
        
        <div class="location-info">
          <div class="input-group">
            <label>经度:</label>
            <input type="number" bind:value={selectedLng} step="0.000001" />
          </div>
          <div class="input-group">
            <label>纬度:</label>
            <input type="number" bind:value={selectedLat} step="0.000001" />
          </div>
          <button 
            class="pick-btn"
            on:click={() => mapComponent.enableLocationPicker()}
          >
            📍 在地图上选择
          </button>
        </div>
      </div>
      
      <div class="modal-footer">
        <button class="cancel-btn" on:click={handleCancel}>取消</button>
        <button class="confirm-btn" on:click={handleConfirm}>确定</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  
  .modal-content {
    background: white;
    border-radius: 8px;
    width: 800px;
    max-width: 90vw;
    max-height: 90vh;
    display: flex;
    flex-direction: column;
  }
  
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px;
    border-bottom: 1px solid #e5e7eb;
  }
  
  .modal-header h3 {
    margin: 0;
    font-size: 18px;
  }
  
  .close-btn {
    background: none;
    border: none;
    font-size: 24px;
    cursor: pointer;
    color: #666;
  }
  
  .modal-body {
    flex: 1;
    padding: 16px;
    overflow: auto;
  }
  
  .map-wrapper {
    height: 400px;
    border: 1px solid #e5e7eb;
    border-radius: 4px;
    margin-bottom: 16px;
  }
  
  .location-info {
    display: flex;
    gap: 16px;
    align-items: center;
  }
  
  .input-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  
  .input-group label {
    font-weight: 500;
    color: #374151;
  }
  
  .input-group input {
    width: 150px;
    padding: 8px 12px;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    font-size: 14px;
  }
  
  .pick-btn {
    padding: 8px 16px;
    background: #f3f4f6;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    cursor: pointer;
    transition: background 0.2s;
  }
  
  .pick-btn:hover {
    background: #e5e7eb;
  }
  
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px;
    border-top: 1px solid #e5e7eb;
  }
  
  .cancel-btn {
    padding: 8px 24px;
    background: white;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    cursor: pointer;
  }
  
  .confirm-btn {
    padding: 8px 24px;
    background: #3b82f6;
    color: white;
    border: none;
    border-radius: 4px;
    cursor: pointer;
  }
  
  .confirm-btn:hover {
    background: #2563eb;
  }
</style>
```

### 2.5 后端API实现

#### API路由定义

```go
// internal/api/map.go
package api

import (
    "encoding/json"
    "net/http"
    
    "github.com/go-chi/chi/v5"
)

type MapHandler struct {
    db *storage.DB
}

func NewMapHandler(db *storage.DB) *MapHandler {
    return &MapHandler{db: db}
}

func (h *MapHandler) Routes() chi.Router {
    r := chi.NewRouter()
    
    r.Get("/cameras", h.GetCamerasWithLocation)
    r.Put("/cameras/{id}/location", h.UpdateCameraLocation)
    r.Get("/cameras/nearby", h.GetNearbyCameras)
    
    return r
}

// GetCamerasWithLocation 获取带位置信息的摄像头列表
func (h *MapHandler) GetCamerasWithLocation(w http.ResponseWriter, r *http.Request) {
    cameras, err := h.db.GetCamerasWithLocation(r.Context())
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    writeJSON(w, cameras)
}

// UpdateCameraLocation 更新摄像头位置
func (h *MapHandler) UpdateCameraLocation(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    
    var req struct {
        Latitude  float64 `json:"latitude"`
        Longitude float64 `json:"longitude"`
        Address   string  `json:"address"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    err := h.db.UpdateCameraLocation(r.Context(), id, req.Latitude, req.Longitude, req.Address)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    writeJSON(w, map[string]string{"status": "ok"})
}

// GetNearbyCameras 获取附近的摄像头
func (h *MapHandler) GetNearbyCameras(w http.ResponseWriter, r *http.Request) {
    lat := parseFloat(r.URL.Query().Get("lat"))
    lng := parseFloat(r.URL.Query().Get("lng"))
    radius := parseFloatDefault(r.URL.Query().Get("radius"), 1000) // 默认1km
    
    cameras, err := h.db.GetNearbyCameras(r.Context(), lat, lng, radius)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    writeJSON(w, cameras)
}
```

#### 数据库操作

```go
// internal/storage/map.go
package storage

import (
    "context"
    "math"
)

// CameraWithLocation 带位置信息的摄像头
type CameraWithLocation struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    URL          string   `json:"url"`
    Protocol     string   `json:"protocol"`
    Encoding     string   `json:"encoding"`
    Enabled      bool     `json:"enabled"`
    Status       string   `json:"status"`
    
    // 位置信息
    Latitude     *float64 `json:"latitude,omitempty"`
    Longitude    *float64 `json:"longitude,omitempty"`
    Address      string   `json:"address,omitempty"`
    PTZType      int      `json:"ptzType"`
    Manufacturer string   `json:"manufacturer,omitempty"`
}

// GetCamerasWithLocation 获取带位置信息的摄像头
func (db *DB) GetCamerasWithLocation(ctx context.Context) ([]CameraWithLocation, error) {
    query := `
        SELECT 
            c.id, c.name, c.url, c.protocol, c.encoding, c.enabled,
            c.latitude, c.longitude, c.address, c.ptz_type, c.manufacturer,
            CASE 
                WHEN c.enabled = 1 THEN 'ON'
                ELSE 'OFF'
            END as status
        FROM cameras c
        WHERE c.archived = 0
        ORDER BY c.name
    `
    
    rows, err := db.QueryContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var cameras []CameraWithLocation
    for rows.Next() {
        var cam CameraWithLocation
        err := rows.Scan(
            &cam.ID, &cam.Name, &cam.URL, &cam.Protocol, &cam.Encoding, &cam.Enabled,
            &cam.Latitude, &cam.Longitude, &cam.Address, &cam.PTZType, &cam.Manufacturer,
            &cam.Status,
        )
        if err != nil {
            return nil, err
        }
        cameras = append(cameras, cam)
    }
    
    return cameras, nil
}

// UpdateCameraLocation 更新摄像头位置
func (db *DB) UpdateCameraLocation(ctx context.Context, id string, lat, lng float64, address string) error {
    query := `
        UPDATE cameras 
        SET latitude = ?, longitude = ?, address = ?
        WHERE id = ?
    `
    
    _, err := db.ExecContext(ctx, query, lat, lng, address, id)
    return err
}

// GetNearbyCameras 获取附近的摄像头
func (db *DB) GetNearbyCameras(ctx context.Context, lat, lng, radiusMeters float64) ([]CameraWithLocation, error) {
    // 使用Haversine公式计算距离
    query := `
        SELECT 
            c.id, c.name, c.url, c.protocol, c.encoding, c.enabled,
            c.latitude, c.longitude, c.address, c.ptz_type, c.manufacturer,
            CASE 
                WHEN c.enabled = 1 THEN 'ON'
                ELSE 'OFF'
            END as status,
            (6371000 * acos(
                cos(radians(?)) * cos(radians(c.latitude)) *
                cos(radians(c.longitude) - radians(?)) +
                sin(radians(?)) * sin(radians(c.latitude))
            )) AS distance
        FROM cameras c
        WHERE c.archived = 0
            AND c.latitude IS NOT NULL 
            AND c.longitude IS NOT NULL
        HAVING distance <= ?
        ORDER BY distance
    `
    
    rows, err := db.QueryContext(ctx, query, lat, lng, lat, radiusMeters)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var cameras []CameraWithLocation
    for rows.Next() {
        var cam CameraWithLocation
        var distance float64
        err := rows.Scan(
            &cam.ID, &cam.Name, &cam.URL, &cam.Protocol, &cam.Encoding, &cam.Enabled,
            &cam.Latitude, &cam.Longitude, &cam.Address, &cam.PTZType, &cam.Manufacturer,
            &cam.Status, &distance,
        )
        if err != nil {
            return nil, err
        }
        cameras = append(cameras, cam)
    }
    
    return cameras, nil
}

// HaversineDistance 计算两点之间的距离（米）
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
    const R = 6371000 // 地球半径（米）
    
    dLat := toRadians(lat2 - lat1)
    dLng := toRadians(lng2 - lng1)
    
    a := math.Sin(dLat/2)*math.Sin(dLat/2) +
        math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*
            math.Sin(dLng/2)*math.Sin(dLng/2)
    
    c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
    
    return R * c
}

func toRadians(deg float64) float64 {
    return deg * math.Pi / 180
}
```

### 2.6 配置扩展

```yaml
# config.yaml 扩展
map:
  enabled: true
  
  # 地图中心点（默认）
  default_center:
    latitude: 39.9042
    longitude: 116.4074
    zoom: 12
  
  # 天地图配置
  tianditu:
    token: "your_token_here"  # 申请地址: https://console.tianditu.gov.cn/
  
  # 或者使用OpenStreetMap（无需token）
  tile_provider: "osm"  # osm / tianditu / gaode / baidu
```

## 3. 功能对比

### 3.1 参考项目功能

| 功能 | 说明 | 实现难度 |
|------|------|---------|
| ✅ 地图展示 | 天地图API | 中 |
| ✅ 设备标记 | 根据设备类型显示不同图标 | 低 |
| ✅ 信息窗口 | 显示设备详情 | 低 |
| ✅ 位置编辑 | 修改设备经纬度 | 中 |
| ✅ 视频播放 | 在地图上播放视频 | 高 |
| ✅ 云台控制 | PTZ控制 | 高 |
| ✅ 区域管理 | 行政区划分组 | 中 |
| ✅ 热力图 | 设备在线状态可视化 | 中 |

### 3.2 lalmax-nvr 集成计划

| 阶段 | 功能 | 预计工作量 |
|------|------|-----------|
| Phase 1 | 基础地图展示 | 2天 |
| Phase 1 | 设备标记和信息窗口 | 2天 |
| Phase 1 | 位置编辑 | 1天 |
| Phase 2 | 视频播放集成 | 3天 |
| Phase 2 | 云台控制集成 | 2天 |
| Phase 3 | 区域管理 | 3天 |
| Phase 3 | 热力图/聚合 | 2天 |

## 4. 注意事项

### 4.1 坐标系问题

- 天地图使用 **WGS84** 坐标系（EPSG:4326）
- 国内地图（高德、百度）使用 **GCJ-02** 或 **BD-09** 坐标系
- 海康设备GPS数据通常是 **WGS84** 坐标
- 需要注意坐标转换

```javascript
// 坐标转换示例
function wgs84ToGcj02(lat, lng) {
  // WGS84转GCJ02
  // ... 转换算法
}

function gcj02ToWgs84(lat, lng) {
  // GCJ02转WGS84
  // ... 转换算法
}
```

### 4.2 性能优化

```javascript
// 1. 标记聚合
import L from 'leaflet';
import 'leaflet.markercluster';

const markers = L.markerClusterGroup();
cameras.forEach(camera => {
  const marker = L.marker([camera.lat, camera.lng]);
  markers.addLayer(marker);
});
map.addLayer(markers);

// 2. 虚拟滚动（大量设备时）
// 只渲染可视区域内的标记

// 3. 防抖处理
function debounce(fn, delay) {
  let timer;
  return function(...args) {
    clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), delay);
  };
}
```

### 4.3 移动端适配

```css
/* 响应式布局 */
@media (max-width: 768px) {
  .map-page {
    flex-direction: column;
  }
  
  .map-sidebar {
    width: 100%;
    height: 200px;
  }
}
```

## 5. 替代方案

如果不想使用天地图，还有以下选择：

### 5.1 OpenStreetMap（完全免费）

```javascript
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
  attribution: '&copy; OpenStreetMap contributors'
}).addTo(map);
```

### 5.2 高德地图（需要API Key）

```javascript
L.tileLayer('https://webrd01.is.autonavi.com/appmaptile?lang=zh_cn&size=1&scale=1&style=8&x={x}&y={y}&z={z}', {
  attribution: '&copy; 高德地图'
}).addTo(map);
```

### 5.3 百度地图（需要API Key）

```javascript
// 需要使用百度地图专用插件
L.tileLayer.baidu({layer: 'vec'}).addTo(map);
```

## 6. 总结

### 6.1 可行性评估

| 评估项 | 结论 |
|--------|------|
| 技术可行性 | ✅ 完全可行，Leaflet + 天地图方案成熟 |
| 工作量评估 | 约2-3周（完整功能） |
| 依赖项 | leaflet (npm) |
| API Key | 天地图免费申请 |

### 6.2 推荐实施步骤

1. **申请天地图API Key**（1天）
   - 访问 https://console.tianditu.gov.cn/
   - 注册并创建应用

2. **数据库Schema扩展**（1天）
   - 添加位置相关字段

3. **后端API开发**（2天）
   - 位置CRUD接口
   - 附近设备查询

4. **前端基础地图**（3天）
   - 地图组件
   - 标记组件
   - 信息窗口

5. **位置编辑功能**（2天）
   - 位置选择器
   - 位置保存

6. **视频播放集成**（3天）
   - 地图上播放视频
   - 与LiveView集成

7. **测试和优化**（2天）

### 6.3 快速开始

```bash
# 1. 安装依赖
cd web
npm install leaflet

# 2. 申请天地图Token
# 访问: https://console.tianditu.gov.cn/

# 3. 创建地图页面
# 参考上述代码实现

# 4. 扩展数据库
# 添加latitude, longitude字段

# 5. 实现API
# 参考上述Go代码
```

---

**文档版本**: v1.0  
**创建日期**: 2024  
**适用版本**: lalmax-nvr v0.37+
