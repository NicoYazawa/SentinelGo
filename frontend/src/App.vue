<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { Fold, Expand, Refresh, ArrowDown, SwitchButton, User, Lock } from '@element-plus/icons-vue'
import Sidebar from './components/Sidebar.vue'
import ProfileDialog from './components/ProfileDialog.vue'
import LanguageSwitcher from './components/LanguageSwitcher.vue'
import { removeToken, getToken, getUsername } from './utils/auth'
import { setUserTimezone } from './utils/timezone'
import { elLocaleRef, setLocale, SUPPORTED_LANGS, type Lang } from './locales'
import request from './utils/request'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const isCollapse = ref(false)
const username = ref(getUsername() || '')
const avatar = ref('')
const profileVisible = ref(false)
const standalone = computed(() => ['/login', '/change-password'].includes(route.path))
const displayName = computed(() => username.value || getUsername() || t('common.user'))
const avatarInitial = computed(() => Array.from(displayName.value)[0]?.toUpperCase() || 'U')
const pageTitle = computed(() => String(route.meta.title || 'SentinelGo'))
const breadcrumbs = computed(() => {
  const items: { title: string; path?: string }[] = [{ title: t('nav.home'), path: '/dashboard' }]
  if (route.path === '/dashboard') return [{ title: t('nav.home') }]
  if (route.path.startsWith('/system/')) items.push({ title: t('nav.system'), path: '/system' })
  items.push({ title: pageTitle.value })
  return items
})

async function loadCurrentUser() {
  if (!getToken()) return
  try {
    const res = await request.get('/user/info')
    const user = res.data?.user
    username.value = user?.nickname || user?.username || getUsername() || ''
    avatar.value = user?.avatar || ''
    if (user?.timezone) setUserTimezone(user.timezone)
    // 登录后按用户在库中的语言偏好同步界面语言（其保存的偏好优先）
    if (user?.locale && (SUPPORTED_LANGS as string[]).includes(user.locale)) {
      setLocale(user.locale as Lang)
    }
  } catch { /* Request interceptor handles authentication errors. */ }
}

// Refresh identity when entering the layout after login or saving the profile.
watch(standalone, isLogin => {
  if (!isLogin) void loadCurrentUser()
  else { username.value = ''; avatar.value = '' }
}, { immediate: true })

function handleRefresh() { window.location.reload() }

async function handleLogout() {
  ElMessageBox.confirm(t('common.confirmLogout'), t('common.tip'), { type: 'warning' }).then(async () => {
    try { await request.post('/auth/logout') } catch { /* Preserve the existing logout flow. */ }
    removeToken()
    router.push('/login')
  }).catch(() => {})
}
</script>

<template>
  <el-config-provider :locale="elLocaleRef">
  <div v-if="standalone" class="app-container">
    <router-view />
  </div>
  <el-container v-else class="layout-container">
    <el-aside :width="isCollapse ? '76px' : '232px'" class="aside" :class="{ 'aside-collapsed': isCollapse }">
      <router-link to="/dashboard" class="brand" :aria-label="'SentinelGo ' + t('nav.home')">
        <span class="brand-mark"><el-icon><Lock /></el-icon></span>
        <span v-if="!isCollapse" class="brand-copy">
          <strong>SentinelGo</strong>
          <span>{{ t('nav.console') }}</span>
        </span>
      </router-link>
      <Sidebar :collapse="isCollapse" />
    </el-aside>

    <el-container class="main-container">
      <el-header class="header">
        <div class="header-left">
          <button class="icon-button" type="button" :aria-label="isCollapse ? t('nav.expand') : t('nav.collapse')" :aria-expanded="!isCollapse" @click="isCollapse = !isCollapse">
            <el-icon><Fold v-if="!isCollapse" /><Expand v-else /></el-icon>
          </button>
          <div class="page-context">
            <nav :aria-label="t('nav.breadcrumb')">
              <el-breadcrumb separator="/">
                <el-breadcrumb-item v-for="(item, index) in breadcrumbs" :key="item.title" :to="item.path" :aria-current="index === breadcrumbs.length - 1 ? 'page' : undefined">
                  {{ item.title }}
                </el-breadcrumb-item>
              </el-breadcrumb>
            </nav>
            <h1 class="page-title">{{ pageTitle }}</h1>
          </div>
        </div>
        <div class="header-right">
          <LanguageSwitcher />
          <el-tooltip :content="t('nav.refresh')" placement="bottom">
            <button class="icon-button refresh-button" type="button" :aria-label="t('nav.refresh')" @click="handleRefresh"><el-icon><Refresh /></el-icon></button>
          </el-tooltip>
          <span class="header-divider" aria-hidden="true"></span>
          <el-dropdown trigger="click">
            <button class="user-info" type="button" :aria-label="`${displayName}，${t('nav.profile')}`">
              <el-avatar :size="34" :src="avatar" class="user-avatar">{{ avatarInitial }}</el-avatar>
              <span class="user-name">{{ displayName }}</span>
              <el-icon class="user-chevron"><ArrowDown /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="profileVisible = true"><el-icon><User /></el-icon>{{ t('nav.profile') }}</el-dropdown-item>
                <el-dropdown-item divided class="logout-item" @click="handleLogout"><el-icon><SwitchButton /></el-icon>{{ t('nav.logout') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-main class="main-content"><router-view /></el-main>
    </el-container>
  </el-container>
  <ProfileDialog v-model="profileVisible" @saved="loadCurrentUser" />
  </el-config-provider>
</template>

<style scoped>
.layout-container { height: 100dvh; width: 100%; overflow: hidden; }
.aside { display: flex; flex-direction: column; background: var(--sg-nav-bg); transition: width 180ms ease; overflow: hidden; border-right: 1px solid #20344c; }
.brand { display: flex; align-items: center; gap: 12px; height: 80px; padding: 0 22px; flex-shrink: 0; color: #f8fafc; text-decoration: none; border-bottom: 1px solid rgba(255,255,255,.07); }
.brand-mark { display: grid; place-items: center; width: 34px; height: 36px; flex-shrink: 0; border: 1px solid #4b6c90; border-radius: 10px; color: #b7d4f5; background: #243c58; font-size: 19px; }
.brand-copy { display: flex; flex-direction: column; gap: 4px; white-space: nowrap; }
.brand-copy strong { font-size: 17px; font-weight: 650; letter-spacing: -.3px; }
.brand-copy > span { font-size: 11px; letter-spacing: 1.5px; color: #90a5bd; }
.aside-collapsed .brand { justify-content: center; padding: 0; }
.main-container { min-width: 0; display: flex; flex-direction: column; height: 100%; }
.header { --el-header-height: 80px; display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 0 28px 0 22px; background: #fff; border-bottom: 1px solid var(--sg-border); flex-shrink: 0; }
.header-left { display: flex; align-items: center; gap: 20px; min-width: 0; }
.icon-button { display: inline-flex; align-items: center; justify-content: center; width: 36px; height: 36px; padding: 0; border: 1px solid transparent; border-radius: 8px; color: #64748b; background: transparent; cursor: pointer; font-size: 19px; transition: background 150ms ease, color 150ms ease; flex-shrink: 0; }
.icon-button:hover { color: var(--sg-primary); background: #f0f5fb; }
.icon-button:focus-visible, .user-info:focus-visible, .brand:focus-visible { outline: 2px solid var(--sg-primary); outline-offset: 3px; }
.page-context { display: flex; flex-direction: column; gap: 7px; min-width: 0; }
.page-context :deep(.el-breadcrumb) { font-size: 12px; line-height: 15px; white-space: nowrap; }
.page-context :deep(.el-breadcrumb__inner) { color: #7b899b; font-weight: 400; }
.page-context :deep(.el-breadcrumb__inner.is-link:hover) { color: var(--sg-primary); }
.page-context :deep(.el-breadcrumb__separator) { margin: 0 9px; color: #c5ced9; font-weight: 400; }
.page-context :deep(.el-breadcrumb__item:last-child .el-breadcrumb__inner) { color: #596a80; }
.page-title { font-size: 18px; font-weight: 650; line-height: 23px; color: #23354b; margin: 0; letter-spacing: .2px; }
.header-right { display: flex; align-items: center; gap: 14px; flex-shrink: 0; }
.header-divider { width: 1px; height: 24px; background: #e8edf3; }
.user-info { display: flex; align-items: center; gap: 10px; padding: 5px 8px; border: 0; border-radius: 8px; background: transparent; color: #41536b; font: inherit; font-size: 13px; cursor: pointer; }
.user-info:hover { background: #f4f7fb; }
.user-avatar { background: #eaf1fa; color: #416d9f; font-size: 14px; font-weight: 600; border: 1px solid #dce7f4; }
.user-name { max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.user-chevron { color: #94a3b8; font-size: 12px; }
.main-content { flex: 1; min-height: 0; padding: 24px; overflow-y: auto; background: var(--sg-page-bg); }
@media (max-width: 760px) {
  .header { padding: 0 12px; gap: 8px; }
  .header-left { gap: 10px; }
  .header-right { gap: 4px; }
  .refresh-button, .header-divider, .user-name { display: none; }
  .user-info { gap: 5px; padding: 4px; }
  .page-context :deep(.el-breadcrumb) { font-size: 11px; }
  .page-context :deep(.el-breadcrumb__separator) { margin: 0 5px; }
  .main-content { padding: 16px; }
}
@media (prefers-reduced-motion: reduce) { .aside, .icon-button { transition: none; } }
</style>
