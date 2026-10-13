<template>
  <div class="login-wrapper">
    <div class="login-card">
      <h1 class="title">SentinelGo</h1>
      <p class="subtitle">{{ t('auth.userLogin') }}</p>

      <el-tabs v-model="activeTab" class="login-tabs">
        <!-- 密码登录 -->
        <el-tab-pane :label="t('auth.passwordTab')" name="password">
          <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-position="top">
            <el-form-item :label="t('auth.username')" prop="username">
              <el-input v-model="pwdForm.username" :placeholder="t('auth.usernamePlaceholder')" size="large" />
            </el-form-item>
            <el-form-item :label="t('auth.password')" prop="password">
              <el-input v-model="pwdForm.password" type="password" :placeholder="t('auth.passwordPlaceholder')" size="large"
                @keyup.enter="handlePwdLogin" show-password />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" :loading="pwdLoading" @click="handlePwdLogin" style="width: 100%">
                {{ t('auth.signInBtn') }}
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 短信登录 -->
        <el-tab-pane :label="t('auth.smsTab')" name="sms">
          <el-form ref="smsFormRef" :model="smsForm" :rules="smsRules" label-position="top">
            <el-form-item :label="t('auth.phone')" prop="phone">
              <el-input v-model="smsForm.phone" :placeholder="t('auth.phonePlaceholder')" size="large" />
            </el-form-item>
            <el-form-item :label="t('auth.code')" prop="code">
              <div class="sms-row">
                <el-input v-model="smsForm.code" :placeholder="t('auth.codePlaceholder')" size="large" style="flex: 1" />
                <el-button size="large" :disabled="countdown > 0" @click="handleSendSMS" style="margin-left: 12px; min-width: 120px">
                  {{ countdown > 0 ? `${countdown}s` : t('auth.sendCode') }}
                </el-button>
              </div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="large" :loading="smsLoading" @click="handleSMSLogin" style="width: 100%">
                {{ t('auth.signInBtn') }}
              </el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- 第三方登录 -->
        <el-tab-pane :label="t('auth.oauthTab')" name="oauth">
          <div class="oauth-section">
            <el-divider />
            <el-button size="large" @click="handleGitHubLogin" style="width: 100%">
              {{ t('auth.githubLogin') }}
            </el-button>
            <p class="oauth-tip">{{ t('auth.githubTip') }}</p>
          </div>
        </el-tab-pane>
      </el-tabs>

      <div class="bottom-link">
        <span>{{ t('auth.noAccount') }}</span>
        <router-link to="/register">{{ t('auth.goRegister') }}</router-link>
        <span style="margin: 0 8px">|</span>
        <router-link to="/forgot-password">{{ t('auth.forgotPassword') }}</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { loginByPassword, sendSMS, loginBySMS } from '@/api/auth'
import { saveAuth, setPasswordChangeRequired } from '@/utils/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const activeTab = ref('password')
const pwdLoading = ref(false)
const smsLoading = ref(false)
const countdown = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({ username: '', password: '' })
const pwdRules: FormRules = {
  username: [{ required: true, message: t('auth.usernameRule'), trigger: 'blur' }],
  password: [{ required: true, message: t('auth.passwordRule'), trigger: 'blur' }],
}

const smsFormRef = ref<FormInstance>()
const smsForm = reactive({ phone: '', code: '' })
const smsRules: FormRules = {
  phone: [
    { required: true, message: t('auth.phonePlaceholder'), trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: t('auth.phonePlaceholder'), trigger: 'blur' },
  ],
  code: [{ required: true, message: t('auth.codePlaceholder'), trigger: 'blur' }],
}

async function handlePwdLogin() {
  const valid = await pwdFormRef.value?.validate().catch(() => false)
  if (!valid) return

  pwdLoading.value = true
  try {
    const res = await loginByPassword(pwdForm.username, pwdForm.password)
    if (res.code === 0) {
      const d = res.data
      saveAuth(d.access_token, d.refresh_token, d.username)
      setPasswordChangeRequired(d.password_change_required === true)
      if (d.password_change_required) { await router.replace('/change-password'); return }
      ElMessage.success(t('auth.loginSuccess'))
      goHome()
    }
  } catch {
    // error handled by interceptor
  } finally {
    pwdLoading.value = false
  }
}

async function handleSendSMS() {
  const valid = await smsFormRef.value?.validateField('phone').catch(() => false)
  if (!valid) return

  try {
    const res = await sendSMS(smsForm.phone)
    ElMessage.success(t('auth.codeSent'))
    if (res.data?.dev_code) {
      smsForm.code = res.data.dev_code
      ElMessage({ message: `【DEV】${res.data.dev_code}`, type: 'warning', showClose: true, duration: 6000 })
    }
    countdown.value = 60
    countdownTimer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0) {
        if (countdownTimer) clearInterval(countdownTimer)
      }
    }, 1000)
  } catch {
    // handled by interceptor
  }
}

async function handleSMSLogin() {
  const valid = await smsFormRef.value?.validate().catch(() => false)
  if (!valid) return

  smsLoading.value = true
  try {
    const res = await loginBySMS(smsForm.phone, smsForm.code)
    if (res.code === 0) {
      const d = res.data
      saveAuth(d.access_token, d.refresh_token, d.username)
      setPasswordChangeRequired(d.password_change_required === true)
      if (d.password_change_required) { await router.replace('/change-password'); return }
      ElMessage.success(t('auth.loginSuccess'))
      goHome()
    }
  } catch {
    // handled by interceptor
  } finally {
    smsLoading.value = false
  }
}

function handleGitHubLogin() {
  window.location.href = '/api/v1/auth/oauth/github'
}

function goHome() {
  const redirect = (route.query.redirect as string) || '/home'
  router.push(redirect)
}
</script>

<style scoped>
.login-wrapper {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  width: 420px;
  padding: 40px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
}

.title {
  text-align: center;
  font-size: 28px;
  font-weight: 700;
  color: #303133;
  margin: 0;
}

.subtitle {
  text-align: center;
  color: #909399;
  margin: 8px 0 24px;
}

.login-tabs {
  margin-bottom: 8px;
}

.sms-row {
  display: flex;
  align-items: center;
  width: 100%;
}

.oauth-section {
  padding: 24px 0 8px;
  text-align: center;
}

.oauth-tip {
  color: #c0c4cc;
  font-size: 13px;
  margin-top: 12px;
}

.bottom-link {
  text-align: center;
  margin-top: 16px;
  color: #909399;
  font-size: 14px;
}

.bottom-link a {
  color: #409eff;
  margin-left: 4px;
  text-decoration: none;
}
</style>
