import { apiRequest } from './client';

export interface GB28181Channel {
  channel_id: string;
  is_playing?: boolean;
  stream_id?: string;
}

export interface GB28181Device {
  device_id: string;
  name?: string;
  manufacturer?: string;
  model?: string;
  firmware?: string;
  is_online: boolean;
  address: string;
  last_keepalive_at?: string;
  last_register_at?: string;
  channels: GB28181Channel[];
}

export interface GB28181DevicesResponse {
  devices: GB28181Device[];
}

export interface GB28181PlayRequest {
  device_id: string;
  channel_id: string;
  stream_id?: string;
}

export interface GB28181PlayResponse {
  ssrc: string;
  stream_id: string;
}

// Platform types
export interface GB28181Platform {
  id: number;
  name: string;
  enable: boolean;
  server_gb_id: string;
  server_ip: string;
  server_port: number;
  device_gb_id: string;
  device_ip: string;
  device_port: number;
  transport: string;
  status?: boolean;
}

export interface GB28181PlatformsResponse {
  platforms: GB28181Platform[];
}

export interface AddPlatformRequest {
  name: string;
  enable: boolean;
  server_gb_id: string;
  server_gb_domain?: string;
  server_ip: string;
  server_port: number;
  device_gb_id?: string;
  device_gb_domain?: string;
  device_ip?: string;
  device_port?: number;
  username?: string;
  password?: string;
  transport?: string;
  character_set?: string;
  expires?: number;
  keep_timeout?: number;
  max_timeout_count?: number;
}

// Alarm types
export interface GB28181Alarm {
  id: number;
  device_id: string;
  channel_id: string;
  alarm_type: string;
  alarm_time: string;
  priority: number;
  method: string;
  description: string;
  created_at: string;
}

export interface GB28181AlarmsResponse {
  alarms: GB28181Alarm[];
  total: number;
}

// Download types
export interface GB28181Download {
  id: number;
  device_id: string;
  channel_id: string;
  file_path: string;
  start_time: string;
  end_time: string;
  file_size: number;
  status: string;
  created_at: string;
}

export interface GB28181DownloadsResponse {
  downloads: GB28181Download[];
  total: number;
}

// Device API
export async function listGB28181Devices(signal?: AbortSignal): Promise<GB28181DevicesResponse> {
  return apiRequest<GB28181DevicesResponse>('/gb28181/devices', { signal });
}

export async function playGB28181Stream(req: GB28181PlayRequest, signal?: AbortSignal): Promise<GB28181PlayResponse> {
  return apiRequest<GB28181PlayResponse>('/gb28181/play', {
    method: 'POST',
    body: JSON.stringify(req),
    signal,
  });
}

export async function stopGB28181Stream(deviceId: string, channelId: string, signal?: AbortSignal): Promise<{ status: string }> {
  return apiRequest<{ status: string }>('/gb28181/stop', {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId }),
    signal,
  });
}

// Platform API
export async function listGB28181Platforms(signal?: AbortSignal): Promise<GB28181PlatformsResponse> {
  return apiRequest<GB28181PlatformsResponse>('/gb28181/platforms', { signal });
}

export async function addGB28181Platform(req: AddPlatformRequest, signal?: AbortSignal): Promise<{ id: number; status: string }> {
  return apiRequest<{ id: number; status: string }>('/gb28181/platforms', {
    method: 'POST',
    body: JSON.stringify(req),
    signal,
  });
}

export async function deleteGB28181Platform(id: number, signal?: AbortSignal): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/gb28181/platforms?id=${id}`, {
    method: 'DELETE',
    signal,
  });
}

// Broadcast API
export async function startBroadcast(deviceId: string, channelId: string, signal?: AbortSignal): Promise<{ session_id: string; port: number; ssrc: string }> {
  return apiRequest<{ session_id: string; port: number; ssrc: string }>('/gb28181/broadcast/start', {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId }),
    signal,
  });
}

export async function stopBroadcast(deviceId: string, channelId: string, signal?: AbortSignal): Promise<{ status: string }> {
  return apiRequest<{ status: string }>('/gb28181/broadcast/stop', {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId }),
    signal,
  });
}

// Alarm API
export async function listGB28181Alarms(deviceId?: string, limit = 50, offset = 0, signal?: AbortSignal): Promise<GB28181AlarmsResponse> {
  const params = new URLSearchParams();
  if (deviceId) params.set('device_id', deviceId);
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  return apiRequest<GB28181AlarmsResponse>(`/gb28181/alarms?${params}`, { signal });
}

// Download API
export async function startDownload(deviceId: string, channelId: string, startTime: string, endTime: string, signal?: AbortSignal): Promise<{ download_id: number; file_path: string; status: string }> {
  return apiRequest<{ download_id: number; file_path: string; status: string }>('/gb28181/download/start', {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId, start_time: startTime, end_time: endTime }),
    signal,
  });
}

export async function stopDownload(deviceId: string, channelId: string, downloadId: number, signal?: AbortSignal): Promise<{ status: string }> {
  return apiRequest<{ status: string }>('/gb28181/download/stop', {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId, download_id: downloadId }),
    signal,
  });
}

export async function listGB28181Downloads(deviceId?: string, channelId?: string, limit = 50, offset = 0, signal?: AbortSignal): Promise<GB28181DownloadsResponse> {
  const params = new URLSearchParams();
  if (deviceId) params.set('device_id', deviceId);
  if (channelId) params.set('channel_id', channelId);
  params.set('limit', String(limit));
  params.set('offset', String(offset));
  return apiRequest<GB28181DownloadsResponse>(`/gb28181/downloads?${params}`, { signal });
}

// --- Device Recording Query ---

export interface DeviceRecordItem {
  name: string;
  path: string;
  start_time: string;
  end_time: string;
  type?: string;
  size?: number;
}

export interface DeviceRecordResponse {
  device_id: string;
  channel_id: string;
  total: number;
  records: DeviceRecordItem[];
}

export interface QueryDeviceRecordRequest {
  device_id: string;
  channel_id: string;
  start_time: string; // RFC3339 or "2006-01-02T15:04:05"
  end_time: string;
}

export async function queryDeviceRecords(data: QueryDeviceRecordRequest): Promise<DeviceRecordResponse> {
  return apiRequest('/gb28181/record_info', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export interface PlaybackRequest {
  device_id: string;
  channel_id: string;
  stream_id?: string;
  start_time: string;
  end_time: string;
}

export interface PlaybackResponse {
  stream_id: string;
  url: string;
  message?: string;
}

export async function startDevicePlayback(data: PlaybackRequest): Promise<PlaybackResponse> {
  return apiRequest('/gb28181/playback', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// Playback speed control
export interface PlaySpeedRequest {
  device_id: string;
  channel_id: string;
  speed: 0.5 | 1 | 2 | 4;
}

// Playback seek control
export interface PlaySeekRequest {
  device_id: string;
  channel_id: string;
  seek_time: number; // seconds from start
}

export async function setPlaybackSpeed(data: PlaySpeedRequest): Promise<{ status: string }> {
  return apiRequest('/gb28181/playback/speed', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function seekPlayback(data: PlaySeekRequest): Promise<{ status: string }> {
  return apiRequest('/gb28181/playback/seek', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}
