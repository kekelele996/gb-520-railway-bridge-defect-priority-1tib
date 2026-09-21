
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listDefectFinding(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/defects?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createDefectFinding(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/defects', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionDefectFinding(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/defects/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
