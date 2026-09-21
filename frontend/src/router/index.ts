import { createRouter, createWebHistory } from 'vue-router';
import BridgeAssetPage from '../pages/BridgeAssetPage.vue';
import InspectionRoundPage from '../pages/InspectionRoundPage.vue';
import DefectFindingPage from '../pages/DefectFindingPage.vue';
import PriorityDecisionPage from '../pages/PriorityDecisionPage.vue';
import AuditPage from '../pages/AuditPage.vue';
import LoginPage from '../pages/LoginPage.vue';
import { loadSession } from '../api/client';
import { roleAtLeast } from '../hooks/useAuth';
import type { UserRole } from '../types/domain';
export const router = createRouter({ history: createWebHistory(), routes: [
	{ path: '/', redirect: '/bridges' },
	{ path: '/login', component: LoginPage, meta: { public: true } },
	{ path: '/bridges', component: BridgeAssetPage, meta: { minimumRole: 'viewer' } },
	{ path: '/inspections', component: InspectionRoundPage, meta: { minimumRole: 'viewer' } },
	{ path: '/defects', component: DefectFindingPage, meta: { minimumRole: 'viewer' } },
	{ path: '/priorities', component: PriorityDecisionPage, meta: { minimumRole: 'viewer' } },
	{ path: '/audit', component: AuditPage, meta: { minimumRole: 'reviewer' } },
] });

router.beforeEach((to) => {
	const session = loadSession();
	if (to.meta.public) return session ? '/bridges' : true;
	if (!session) return { path: '/login', query: { redirect: to.fullPath } };
	const minimum = to.meta.minimumRole as UserRole | undefined;
	if (minimum && !roleAtLeast(session.role, minimum)) return '/bridges';
	return true;
});
