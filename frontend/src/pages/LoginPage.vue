<script setup lang="ts">
import { ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuth } from '../hooks/useAuth';

const username = ref('admin');
const password = ref('Admin123!');
const route = useRoute();
const router = useRouter();
const { authenticate, loading, error } = useAuth();

async function submit() {
	if (await authenticate(username.value.trim(), password.value)) {
		const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/bridges';
		await router.replace(redirect);
	}
}
</script>

<template>
	<main class="login-page">
		<section class="login-panel">
			<p class="eyebrow">安全基础设施决策系统</p>
			<h1>铁路桥梁缺陷处置优先级</h1>
			<p>使用已授权的岗位账号进入处置工作台。</p>
			<el-alert v-if="error" :title="error" type="error" show-icon/>
			<el-form label-position="top" @submit.prevent="submit">
				<el-form-item label="账号"><el-input v-model="username" autocomplete="username"/></el-form-item>
				<el-form-item label="密码"><el-input v-model="password" type="password" autocomplete="current-password" show-password/></el-form-item>
				<el-button native-type="submit" type="primary" :loading="loading">登录工作台</el-button>
			</el-form>
			<small>演示角色：viewer / operator / reviewer / admin，统一密码 Admin123!</small>
		</section>
	</main>
</template>
