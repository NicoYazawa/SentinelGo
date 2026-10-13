/**
 * Vue Router 路由配置。
 *
 * 路由守卫逻辑（beforeEach）：
 *  1. 设置页面标题（取自 meta.title 或默认 "SentinelGo"）
 *  2. 需要认证但未登录 → 重定向到 /login 并携带 redirect 参数
 *  3. 已登录访问 /login → 重定向到首页 /
 *  4. 其余放行
 *
 * 所有页面组件使用懒加载（() => import(...)），按需分割 chunk。
 */
import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn, mustChangePassword } from '@/utils/auth'

const router = createRouter({
  // HTML5 History 模式（需要服务端配置 fallback 到 index.html）
  history: createWebHistory(),
  routes: [
    {
      path: '/change-password',
      component: () => import('@/views/ChangePassword.vue'),
      meta: { requiresAuth: true, title: '设置正式密码' },
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      redirect: '/dashboard',
      meta: { requiresAuth: true },
    },
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue'),
      meta: { requiresAuth: true, title: '首页' },
    },
    // 系统管理（重定向到用户管理）
    {
      path: '/system',
      redirect: '/system/users',
      meta: { requiresAuth: true },
    },
    {
      path: '/system/users',
      name: 'UserManagement',
      component: () => import('@/views/UserManagement.vue'),
      meta: { requiresAuth: true, title: '用户管理' },
    },
    {
      path: '/system/roles',
      name: 'RoleManagement',
      component: () => import('@/views/RoleManagement.vue'),
      meta: { requiresAuth: true, title: '角色管理' },
    },
    {
      path: '/system/menus',
      name: 'MenuManagement',
      component: () => import('@/views/MenuManagement.vue'),
      meta: { requiresAuth: true, title: '菜单管理' },
    },
    {
      path: '/system/permissions',
      name: 'PermissionManagement',
      component: () => import('@/views/PermissionManagement.vue'),
      meta: { requiresAuth: true, title: '权限管理' },
    },
    {
      path: '/system/monitor',
      name: 'MonitorDashboard',
      component: () => import('@/views/MonitorDashboard.vue'),
      meta: { requiresAuth: true, title: '系统监控' },
    },
    {
      path: '/system/audit',
      name: 'OperationLog',
      component: () => import('@/views/OperationLog.vue'),
      meta: { requiresAuth: true, title: '操作日志' },
    },
    // 404 兜底：未匹配的路由重定向到首页
    {
      path: '/:pathMatch(.*)*',
      redirect: '/dashboard',
    },
  ],
})

/**
 * 全局前置守卫逻辑。
 * 每次路由切换前执行：设置标题 → 认证检查 → 登录页互斥。
 * 抽出为独立函数以便单测直接驱动（无需完整路由导航）。
 */
export function routeGuard(
  to: { meta: Record<string, unknown>; path: string; fullPath: string },
  _from: unknown,
  next: (target?: string) => void,
): void {
  // 动态设置页面标题
  document.title = (to.meta.title as string) || 'SentinelGo'

  // 需要认证但未登录：跳转登录页并携带 redirect
  if (to.meta.requiresAuth !== false && !isLoggedIn()) {
    next(`/login?redirect=${encodeURIComponent(to.fullPath)}`)
  } else if (isLoggedIn() && mustChangePassword() && to.path !== '/change-password') {
    next('/change-password')
  } else if (to.path === '/login' && isLoggedIn()) {
    // 已登录用户访问登录页：重定向到首页
    next('/')
  } else {
    next()
  }
}

router.beforeEach(routeGuard)

export default router
