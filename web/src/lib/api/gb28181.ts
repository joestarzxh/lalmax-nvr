import { apiRequest } from './client';

export interface GB28181Channel {
  channel_id: string;
  is_playing?: boolean;
  stream_id?: string;
}

export interface GB28181Device {
  device_id: string;
  is_online: boolean;
  address: string;
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
