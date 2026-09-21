
import { computed, ref } from 'vue';
import { clearSession, loadSession, saveSession } from '../api/client';
import { login } from '../api/auth';
import type { UserRole, UserSession } from '../types/domain';

const roleRank: Record<UserRole, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };
const session = ref<UserSession | null>(loadSession());
const loading = ref(false);
const error = ref('');

export function roleAtLeast(role: UserRole | undefined, minimum: UserRole): boolean {
	return Boolean(role && roleRank[role] >= roleRank[minimum]);
}

export function useAuth() {
	const authenticate = async (username: string, password: string): Promise<boolean> => {
		loading.value = true;
		error.value = '';
		try {
			const next = await login(username, password);
			saveSession(next);
			session.value = next;
			return true;
		} catch (reason) {
			error.value = reason instanceof Error ? reason.message : String(reason);
			return false;
		} finally {
			loading.value = false;
		}
	};
	const logout = () => {
		clearSession();
		session.value = null;
	};
	return {
		session, loading, error, authenticate, logout,
		authenticated: computed(() => Boolean(session.value?.token)),
		canAtLeast: (minimum: UserRole) => roleAtLeast(session.value?.role, minimum),
	};
}
