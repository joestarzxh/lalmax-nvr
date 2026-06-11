import { apiRequest } from './client';

export interface RecordingPlanTimeRange {
  id?: number;
  plan_id?: number;
  day_of_week: number; // 0=Sunday, 1=Monday, ..., 6=Saturday
  start_time: string;  // "HH:MM"
  end_time: string;    // "HH:MM"
}

export interface RecordingPlanChannel {
  id?: number;
  plan_id?: number;
  camera_id: string;
}

export interface RecordingPlan {
  id: number;
  name: string;
  enabled: boolean;
  time_ranges: RecordingPlanTimeRange[];
  channels: RecordingPlanChannel[];
  created_at: string;
  updated_at: string;
}

export interface CreateRecordingPlanRequest {
  name: string;
  enabled: boolean;
  time_ranges: RecordingPlanTimeRange[];
  channels?: RecordingPlanChannel[];
}

export async function listRecordingPlans(): Promise<{ plans: RecordingPlan[] }> {
  return apiRequest('/recording-plans');
}

export async function getRecordingPlan(id: number): Promise<{ plan: RecordingPlan }> {
  return apiRequest(`/recording-plans/${id}`);
}

export async function createRecordingPlan(data: CreateRecordingPlanRequest): Promise<{ plan: RecordingPlan }> {
  return apiRequest('/recording-plans', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updateRecordingPlan(id: number, data: Partial<CreateRecordingPlanRequest>): Promise<{ plan: RecordingPlan }> {
  return apiRequest(`/recording-plans/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export async function deleteRecordingPlan(id: number): Promise<void> {
  return apiRequest(`/recording-plans/${id}`, { method: 'DELETE' });
}

export async function setPlanChannels(planId: number, cameraIds: string[]): Promise<void> {
  return apiRequest(`/recording-plans/${planId}/channels`, {
    method: 'PUT',
    body: JSON.stringify({ camera_ids: cameraIds }),
  });
}

export async function addPlanChannel(planId: number, cameraId: string): Promise<void> {
  return apiRequest(`/recording-plans/${planId}/channels`, {
    method: 'POST',
    body: JSON.stringify({ camera_id: cameraId }),
  });
}

export async function removePlanChannel(planId: number, cameraId: string): Promise<void> {
  return apiRequest(`/recording-plans/${planId}/channels/${cameraId}`, {
    method: 'DELETE',
  });
}
