<template>
  <div class="callback-wrapper">
    <el-result :icon="statusIcon" :title="statusTitle" :sub-title="statusMsg">
      <template #extra>
        <el-button type="primary" @click="goHome" v-if="success">{{ t('oauth.goHome') }}</el-button>
        <el-button @click="goLogin" v-else>{{ t('oauth.backLogin') }}</el-button>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { saveAuth, setPasswordChangeRequired } from '@/utils/auth'

const { t } = useI18n()
const router = useRouter()
const success = ref(false)
const statusIcon = ref<'success' | 'error'>('error')
const statusTitle = ref(t('oauth.loginFailed'))
const statusMsg = ref('')

function goHome() {
  router.push('/home')
}

function goLogin() {
  router.push('/login')
}

onMounted(() => {
  const hash = window.location.hash.substring(1)
  if (!hash) {
    statusMsg.value = t('oauth.missingParams')
    return
  }

  const params = new URLSearchParams(hash)
  const accessToken = params.get('access_token')
  const refreshToken = params.get('refresh_token')
  const username = params.get('username')
  const error = params.get('error')

  if (error) {
    statusMsg.value = decodeURIComponent(error)
    return
  }

  if (!accessToken || !refreshToken || !username) {
    statusMsg.value = t('oauth.incompleteParams')
    return
  }

  saveAuth(accessToken, refreshToken, username)
  setPasswordChangeRequired(params.get('password_change_required') === 'true')
  if (params.get('password_change_required') === 'true') { router.replace('/change-password'); return }
  success.value = true
  statusIcon.value = 'success'
  statusTitle.value = t('oauth.loginSuccess')
  statusMsg.value = t('oauth.welcome') + username

  setTimeout(() => {
    router.push('/home')
  }, 1500)
})
</script>

<style scoped>
.callback-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}
</style>
