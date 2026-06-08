/**
 * API barrel — re-exports everything so existing `$lib/api` imports work unchanged.
 */

// Client — auth, base fetch wrappers
export {
  storeCredentials,
  getCredentials,
  clearCredentials,
  isAuthenticated,
  login,
  logout,
  healthCheck,
  getSystemStats,
  getLocalNetworkInterfaces,
  getAuthHeader,
  API_BASE,
  apiRequest,
  apiRequestBlob,
  ApiRequestError,
  setupApi,
} from './client';

export type {
  AuthCredentials,
  LoginResponse,
  ApiError,
  HealthCheck,
  HealthResponse,
  SystemStats,
  SetupResponse,
  NetworkInterface,
} from './client';

// Cameras — CRUD, ONVIF, PTZ, protocols
export {
  listCameras,
  createCamera,
  getCamera,
  updateCamera,
  deleteCamera,
  permanentlyDeleteCamera,
  getCameraRecordingStats,
  enableCamera,
  disableCamera,
  startCamera,
  stopCamera,
  pauseRecording,
  resumeRecording,
  getDashboardCameras,
  getSnapshotUrl,
  testConnection,
  getMergeConfig,
  updateMergeConfig,
  deleteCameraMergeConfig,
  ptzMove,
  ptzStop,
  getPTZStatus,
  buildPTZContinuousMove,
  discoverONVIFDevices,
  getONVIFDeviceDetail,
  probeONVIFDevice,
  listProtocols,
  normalizeProtocol,
  buildProtocolsMap,
  getProtocolCapabilities,
  checkVendor,
  DEFAULT_PROTOCOLS,
  // Imaging
  getImagingSettings,
  setImagingSettings,
  getImagingOptions,
  // PTZ Presets
  getPTZPresets,
  createPTZPreset,
  goToPTZPreset,
  deletePTZPreset,
  // Snapshot URI
  getSnapshotUri,
  // Device Capabilities
  getDeviceCapabilities,
  // Device Management
  rebootDevice,
  getNetworkInterfaces,
  setNetworkInterfaces,
  getDeviceUsers,
  createDeviceUsers,
  deleteDeviceUsers,
  // Timelapse
  getTimelapseConfig,
  updateTimelapseConfig,
} from './cameras';
export type {
  CameraTranscodingConfig,
  Camera,
  CreateCameraRequest,
  UpdateCameraRequest,
  DiscoveredDevice,
  DiscoveryError,
  DiscoveryResult,
  DeviceInfo,
  DeviceProfile,
  ONVIFDeviceDetail,
  PTZMoveRequest,
  PTZDirection,
  PTZStatus,
  ProtocolCapabilities,
  ProtocolInfo,
  MergeConfig,
  TestConnectionRequest,
  TestConnectionResult,
  VendorCheckResult,
  PushTarget,
  // Imaging
  ImagingSettings,
  ImagingOptionRange,
  ImagingOptions,
  // PTZ Presets
  PTZPreset,
  // Snapshot URI
  SnapshotUriResponse,
  // Device Capabilities
  DeviceCapabilitiesInfo,
  // Device Management
  NetworkIPv4,
  NetworkIPv6,
  NetworkNTP,
  NetworkInterface as ONVIFNetworkInterface,
  ONVIFDeviceUser,
  // Timelapse
  TimelapseConfig,
  CameraRecordingStats,
} from './cameras';
// Recordings — list, download, frames, stats, archives
export {
  listRecordings,
  getRecording,
  deleteRecording,
  batchDeleteRecordings,
  getRecordingDownloadUrl,
  downloadRecording,
  listFrames,
  loadFrameBlob,
  loadRecordingVideoBlob,
  getRecordingPlaybackUrl,
  getStats,
  getStatsTrends,
  listArchives,
  restoreArchiveGroup,
  listArchiveRecordings,
  deleteArchiveGroup,
  deleteArchiveRecording,
  setArchiveRetention,
} from './recordings';

export type {
  Recording,
  FrameInfo,
  FramesResponse,
  RecordingListResponse,
  StorageStats,
  DailyStats,
  ArchiveGroup,
  ArchiveListResponse,
} from './recordings';

// Events — unified event center
export {
  listEvents,
  getEvent,
  acknowledgeEvent,
  deleteEvent,
} from './events';

export type {
  NvrEvent,
  EventsResponse,
  EventsParams,
  EventSource,
  EventSeverity,
  EventStatus,
} from './events';

// Settings — cleanup, webdav, merge, features
export {
  getSettings,
  updateSettings,
  getMergeSettings,
  updateMergeSettings,
  getMergeStatus,
  getMergePending,
  getFeatures,
  updateFeatures,
  getStreamingSettings,
  updateStreamingSettings,
  getGB28181Settings,
  updateGB28181Settings,
  reloadConfig,
  checkConfigChange,
  regenerateLalmaxConfig,
  getHLSSettings,
  updateHLSSettings,
} from './settings';

export type {
  CleanupConfig,
  WebDAVConfig,
  SettingsConfig,
  MergeStatus,
  MergePending,
  FeatureFlags,
  StreamingConfig,
  WebRTCConfig,
  FLVStreamingConfig,
  RTMPConfig,
  SRTStreamConfig,
  SRTConfig,
  GB28181Config,
  HLSConfig,
} from './settings';

// Streams — runtime stream inventory
export {
  listStreams,
  getStream,
  bindCamera,
  unbindCamera,
  promoteStream,
  deleteStream,
  kickPublisher,
} from './streams';

export type {
  StreamInfo,
  StreamPlayURL,
  StreamSessionStatus,
  StreamsResponse,
  BindCameraRequest,
  PromoteStreamRequest,
  StreamOperationResponse,
} from './streams';

// Xiaomi — cloud auth, devices, sync
export {
  xiaomiAuth,
  xiaomiDevices,
  xiaomiCaptcha,
  xiaomiVerify,
  xiaomiSync,
} from './xiaomi';

export type {
  XiaomiDevice,
  XiaomiDevicesResponse,
  XiaomiAuthResponse,
} from './xiaomi';

// Health — camera health status and events
export {
  getHealthStatus,
  getHealthEvents,
  getCameraHealth,
  getHealthCameras,
  getStabilityData,
} from './health';
export type {
  HealthStatus,
  HealthEventType,
  HealthEvent,
  CameraHealth,
  HealthStatusResponse,
  HealthEventsResponse,
  HealthEventsParams,
  CameraHealthDetail,
  HealthCamerasResponse,
  StabilityMetrics,
  StabilityDataResponse,
} from './health';

// Transcoding — hardware check, FFmpeg, task management
export {
  getTranscodingCheck,
  getFFmpegStatus,
  downloadFFmpeg,
  retryDownload,
  getTranscodingStatus,
  getTranscodingTasks,
  enqueueTranscodeTask,
  cancelTranscodeTask,
  getTranscodingCameras,
  getTranscodingSettings,
  updateTranscodingSettings,
} from './transcoding';

export type {
  HardwareCapabilities,
  SelfCheckResult,
  DownloadStatus,
  TranscodeTask,
  ManagerStatus,
  TranscodingSettings,
} from './transcoding';

// AI Detection — localStorage-backed settings
export {
  getAiSettings,
  saveAiSettings,
  detectAiBackend,
} from './ai';

export type {
  AiDetectionSettings,
} from './ai';

// GB28181 — SIP device management
export {
  listGB28181Devices,
  playGB28181Stream,
  stopGB28181Stream,
} from './gb28181';

export type {
  GB28181Device,
  GB28181Channel,
  GB28181DevicesResponse,
  GB28181PlayRequest,
  GB28181PlayResponse,
} from './gb28181';
