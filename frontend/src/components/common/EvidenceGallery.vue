
<script setup lang="ts">
import type { DomainRecord } from '../../types/domain';
import StatusBadge from './StatusBadge.vue';
defineProps<{ records: DomainRecord[] }>();
</script>
<template>
	<div v-if="records.length" class="evidence-strip">
		<article v-for="item in records.slice(0, 4)" :key="item.id">
			<header><strong>{{ item.code }}</strong><StatusBadge :status="item.status"/></header>
			<span>{{ item.name }}</span>
			<p>{{ item.evidence || '未附证据说明' }}</p>
			<small>{{ item.relatedCode || item.facility }}</small>
		</article>
	</div>
	<div v-else class="empty">暂无业务证据</div>
</template>
