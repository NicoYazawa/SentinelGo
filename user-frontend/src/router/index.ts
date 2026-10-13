import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn, mustChangePassword } from '@/utils/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/change-password',
      component: () => import('@/views/ChangePassword.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/register',
      name: 'Register',
      component: () => import('@/views/Register.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/auth/oauth/callback',
      name: 'OAuthCallback',
      component: () => import('@/views/OAuthCallback.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/forgot-password',
      name: 'ForgotPassword',
      component: () => import('@/views/ForgotPassword.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/profile',
      name: 'Profile',
      component: () => import('@/views/Profile.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/home',
      name: 'Home',
      component: () => import('@/views/Home.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/account',
      name: 'AccountBind',
      component: () => import('@/views/AccountBind.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/',
      redirect: '/home',
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/home',
    },
  ],
})

router.beforeEach((to, _from, next) => {
  document.title = 'SentinelGo'

  if (to.meta.requiresAuth !== false && !isLoggedIn()) {
    next(`/login?redirect=${encodeURIComponent(to.fullPath)}`)
  } else if (isLoggedIn() && mustChangePassword() && to.path !== '/change-password') {
    next('/change-password')
  } else if (to.path === '/login' && isLoggedIn()) {
    next('/home')
  } else {
    next()
  }
})

export default router
