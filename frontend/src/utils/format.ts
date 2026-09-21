
export function formatDate(value: string): string {
  return value ? new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '-';
}
export function nextStatus(current: string, statuses: readonly string[]): string | null {
  const index = statuses.indexOf(current);
  return index >= 0 && index < statuses.length - 1 ? statuses[index + 1] : null;
}

const transitions: Record<string, Record<string, readonly string[]>> = {
	bridgeAsset: { active: ['restricted', 'closed'], restricted: ['closed', 'retired', 'active'], closed: ['retired', 'restricted'], retired: ['closed'] },
	inspectionRound: { planned: ['running', 'review'], running: ['review', 'completed', 'planned'], review: ['completed', 'running'], completed: ['review'] },
	defectFinding: { new: ['verified', 'monitoring'], verified: ['monitoring', 'mitigated', 'new'], monitoring: ['mitigated', 'closed', 'verified'], mitigated: ['closed', 'monitoring'], closed: ['mitigated'] },
	priorityDecision: { draft: ['observe', 'restrict', 'urgent'], observe: [], restrict: [], urgent: [] },
};

export function allowedTargets(entityKey: string, current: string): readonly string[] {
	return transitions[entityKey]?.[current] || [];
}
export function statusTone(status: string): 'success' | 'warning' | 'danger' | 'neutral' {
  if (/approved|accepted|released|completed|signed|closed|pass|ready|online|cleared|succeeded/.test(status)) return 'success';
  if (/failed|rejected|critical|scrap|discard|revoked|urgent/.test(status)) return 'danger';
  if (/hold|warning|review|pending|restricted|limited|quarantine/.test(status)) return 'warning';
  return 'neutral';
}
