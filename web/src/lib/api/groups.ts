import { apiRequest } from './client';

// ==================== Types ====================

export interface DeviceGroup {
  id: number;
  name: string;
  parent_id: number;
  level: number;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface DeviceGroupTreeNode {
  id: number;
  name: string;
  parent_id: number;
  level: number;
  sort_order: number;
  children?: DeviceGroupTreeNode[];
  stats?: {
    total: number;
    online: number;
  };
}

export interface DeviceGroupChannelDetail {
  id: number;
  group_id: number;
  device_id: string;
  channel_id: string;
  created_at: string;
  device_name: string;
  channel_name: string;
  is_online: boolean;
}

export interface CreateGroupRequest {
  name: string;
  parent_id?: number;
  sort_order?: number;
}

export interface UpdateGroupRequest {
  name?: string;
  parent_id?: number;
  sort_order?: number;
}

export interface AddGroupChannelRequest {
  device_id: string;
  channel_id: string;
}

export interface RemoveGroupChannelRequest {
  device_id: string;
  channel_id: string;
}

// ==================== API Functions ====================

/**
 * List all device groups.
 */
export async function listDeviceGroups(): Promise<DeviceGroup[]> {
  const res = await apiRequest<{ groups: DeviceGroup[] }>('/groups');
  return res.groups || [];
}

/**
 * Get a single device group by ID.
 */
export async function getDeviceGroup(id: number): Promise<{
  group: DeviceGroup;
  channel_total: number;
  channel_online: number;
}> {
  return apiRequest<{
    group: DeviceGroup;
    channel_total: number;
    channel_online: number;
  }>(`/groups/${id}`);
}

/**
 * Get the group tree structure.
 */
export async function getDeviceGroupTree(): Promise<DeviceGroupTreeNode[]> {
  const res = await apiRequest<{ tree: DeviceGroupTreeNode[] }>('/groups/tree');
  return res.tree || [];
}

/**
 * Create a new device group.
 */
export async function createDeviceGroup(req: CreateGroupRequest): Promise<{
  group: DeviceGroup;
  status: string;
}> {
  return apiRequest<{
    group: DeviceGroup;
    status: string;
  }>('/groups', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

/**
 * Update an existing device group.
 */
export async function updateDeviceGroup(
  id: number,
  req: UpdateGroupRequest
): Promise<{
  group: DeviceGroup;
  status: string;
}> {
  return apiRequest<{
    group: DeviceGroup;
    status: string;
  }>(`/groups/${id}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  });
}

/**
 * Delete a device group.
 */
export async function deleteDeviceGroup(id: number): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/groups/${id}`, {
    method: 'DELETE',
  });
}

/**
 * List channels in a group.
 */
export async function listGroupChannels(
  groupId: number
): Promise<DeviceGroupChannelDetail[]> {
  const res = await apiRequest<{ channels: DeviceGroupChannelDetail[] }>(
    `/groups/${groupId}/channels`
  );
  return res.channels || [];
}

/**
 * Add a channel to a group.
 */
export async function addGroupChannel(
  groupId: number,
  deviceId: string,
  channelId: string
): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/groups/${groupId}/channels`, {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId }),
  });
}

/**
 * Remove a channel from a group.
 */
export async function removeGroupChannel(
  groupId: number,
  deviceId: string,
  channelId: string
): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/groups/${groupId}/channels`, {
    method: 'DELETE',
    body: JSON.stringify({ device_id: deviceId, channel_id: channelId }),
  });
}
