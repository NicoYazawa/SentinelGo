<template>
  <div class="home-wrapper">
    <header class="home-header">
      <h1>SentinelGo</h1>
      <div class="header-right">
        <span class="greeting">{{ t('home.greeting') }}{{ username }}</span>
        <el-button @click="goProfile">{{ t('home.profileBtn') }}</el-button>
        <el-button @click="goAccount">{{ t('home.accountBtn') }}</el-button>
        <el-button type="danger" plain @click="handleLogout">{{ t('nav.logout') }}</el-button>
      </div>
    </header>

    <main class="home-main">
      <el-card class="info-card">
        <template #header>
          <span>{{ t('home.welcome') }}</span>
        </template>
        <p>{{ t('home.welcomeDesc1') }}</p>
        <p>{{ t('home.welcomeDesc2') }}</p>
      </el-card>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getUsername, removeToken } from '@/utils/auth'
import { logout } from '@/api/auth'

const { t } = useI18n()
const router = useRouter()
const username = ref(getUsername() || t('common.user'))

async function handleLogout() {
  try {
    await logout()
  } catch {
    // ignore
  }
  removeToken()
  ElMessage.success(t('common.loggedOut'))
  router.push('/login')
}

function goProfile() {
  router.push('/profile')
}

function goAccount() {
  router.push('/account')
}
</script>

<style scoped>
.home-wrapper {
  min-height: 100vh;
  background: #f5f7fa;
}

.home-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 40px;
  height: 64px;
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.home-header h1 {
  font-size: 20px;
  color: #303133;
  margin: 0;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.greeting {
  color: #606266;
  font-size: 14px;
}

.home-main {
  max-width: 800px;
  margin: 40px auto;
  padding: 0 20px;
}

.info-card p {
  color: #606266;
  line-height: 1.8;
  margin: 8px 0;
}
</style>
