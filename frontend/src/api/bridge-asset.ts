
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listBridgeAsset(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/bridges?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createBridgeAsset(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/bridges', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionBridgeAsset(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/bridges/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
