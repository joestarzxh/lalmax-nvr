/**
 * User management API
 */

import { apiRequest } from './client';

export interface User {
  id: number;
  username: string;
  role: 'super_admin' | 'user';
  display_name: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateUserRequest {
  username: string;
  password: string;
  role?: 'super_admin' | 'user';
  display_name?: string;
}

export interface UpdateUserRequest {
  password?: string;
  role?: 'super_admin' | 'user';
  display_name?: string;
  enabled?: boolean;
}

/**
 * List all users
 */
export async function listUsers(): Promise<User[]> {
  return apiRequest<User[]>('/users');
}

/**
 * Get a user by ID
 */
export async function getUser(id: number): Promise<User> {
  return apiRequest<User>(`/users/${id}`);
}

/**
 * Create a new user
 */
export async function createUser(req: CreateUserRequest): Promise<User> {
  return apiRequest<User>('/users', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

/**
 * Update a user
 */
export async function updateUser(id: number, req: UpdateUserRequest): Promise<User> {
  return apiRequest<User>(`/users/${id}`, {
    method: 'PUT',
    body: JSON.stringify(req),
  });
}

/**
 * Delete a user
 */
export async function deleteUser(id: number): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/users/${id}`, {
    method: 'DELETE',
  });
}
