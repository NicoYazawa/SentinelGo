<!--
  Login.vue — 登录页面（admin）。

  功能：
    - 用户名 + 密码表单登录
    - 登录成功后保存 Token 对（access + refresh）和用户名
    - 支持 redirect 参数：登录后跳转到之前访问的页面
    - 回车键快捷登录（@keyup.enter）
-->
<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import { saveAuth, setPasswordChangeRequired } from '@/utils/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

/** 登录按钮加载状态 */
const loading = ref(false)

/** 登录表单数据 */
const form = reactive({
  username: '',
  password: '',
})

/** 执行登录 */
async function handleLogin() {
  if (!form.username || !form.password) {
    ElMessage.warning(t('login.enterBoth'))
    return
  }

  loading.value = true
  try {
    const res = await axios.post('/api/login', {
      username: form.username,
      password: form.password,
    })

    if (res.data.code === 0) {
      const { access_token, refresh_token, username } = res.data.data
      // 保存认证信息到 localStorage
      saveAuth(access_token, refresh_token, username)
      setPasswordChangeRequired(res.data.data.password_change_required === true)
      if (res.data.data.password_change_required) {
        await router.replace('/change-password')
        return
      }
      ElMessage.success(t('login.success'))

      // 跳转到 redirect 参数指定的页面或默认首页
      const redirect = (route.query.redirect as string) || '/'
      router.push(redirect)
    } else {
      ElMessage.error(res.data.msg || t('login.failed'))
    }
  } catch {
    ElMessage.error(t('common.networkError'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-container">
    <div class="login-card">
      <h2 class="login-title">SentinelGo</h2>
      <p class="login-subtitle">{{ t('login.subtitle') }}</p>

      <el-form @submit.prevent="handleLogin">
        <el-form-item>
          <el-input
            v-model="form.username"
            :placeholder="t('login.usernamePlaceholder')"
            size="large"
            prefix-icon="User"
          />
        </el-form-item>

        <el-form-item>
          <el-input
            v-model="form.password"
            type="password"
            :placeholder="t('login.passwordPlaceholder')"
            size="large"
            prefix-icon="Lock"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="login-button"
            @click="handleLogin"
          >
            {{ t('login.button') }}
          </el-button>
        </el-form-item>
      </el-form>

      <p class="login-hint">{{ t('login.hint') }}</p>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100vh;
  background: linear-gradient(135deg, #1e3c72 0%, #2a5298 50%, #1e3c72 100%);
}

.login-card {
  width: 400px;
  padding: 40px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
}

.login-title {
  text-align: center;
  font-size: 28px;
  color: #303133;
  margin-bottom: 8px;
}

.login-subtitle {
  text-align: center;
  color: #909399;
  margin-bottom: 32px;
  font-size: 14px;
}

.login-hint {
  text-align: center;
  color: #c0c4cc;
  font-size: 12px;
  margin-top: 16px;
}

.login-button {
  width: 100%;
}
</style>
