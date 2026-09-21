
<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';
import { useAuth } from './hooks/useAuth';
const router = useRouter();
const { session, logout, canAtLeast } = useAuth();
const navigation = computed(() => [
	{ to: '/bridges', label: '桥梁资产', visible: true },
	{ to: '/inspections', label: '检查批次', visible: true },
	{ to: '/defects', label: '缺陷发现', visible: true },
	{ to: '/priorities', label: '优先级决定', visible: true },
	{ to: '/audit', label: '审计记录', visible: canAtLeast('reviewer') },
].filter((item) => item.visible));
function signOut() { logout(); void router.push('/login'); }
</script>
<template>
	<router-view v-if="!session"/>
	<div v-else class="app-shell">
		<aside>
			<div class="brand"><span>CONTROL DESK</span><strong>铁路桥梁缺陷处置优先级</strong></div>
			<nav><router-link v-for="item in navigation" :key="item.to" :to="item.to">{{ item.label }}</router-link></nav>
			<div class="user-panel"><span>{{ session.displayName }}</span><small>{{ session.role }}</small><button @click="signOut">退出登录</button></div>
		</aside>
		<section class="content"><header class="topbar"><span>运行态势</span><span class="live-dot">服务已连接</span></header><router-view/></section>
	</div>
</template>
