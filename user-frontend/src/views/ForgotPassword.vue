<template>
  <div class="login-wrapper">
    <div class="login-card">
      <h1 class="title">{{ t('forgot.title') }}</h1>
      <p class="subtitle">{{ t('forgot.subtitle') }}</p>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item :label="t('auth.phone')" prop="phone">
          <el-input v-model="form.phone" :placeholder="t('forgot.phonePlaceholder')" size="large" />
        </el-form-item>
        <el-form-item :label="t('auth.code')" prop="code">
          <div class="sms-row">
            <el-input v-model="form.code" :placeholder="t('auth.codePlaceholder')" size="large" style="flex: 1" />
            <el-button size="large" :disabled="countdown > 0" @click="handleSendCode" style="margin-left: 12px; min-width: 120px">
              {{ countdown > 0 ? `${countdown}s` : t('auth.sendCode') }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item :label="t('forgot.newPassword')" prop="newPassword">
          <el-input v-model="form.newPassword" type="password" :placeholder="t('forgot.newPassword')" size="large" show-password />
        </el-form-item>
        <el-form-item :label="t('forgot.confirmPwd')" prop="confirmPwd">
          <el-input v-model="form.confirmPwd" type="password" :placeholder="t('forgot.confirmPwdPlaceholder')" size="large" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" :loading="loading" @click="handleReset" style="width: 100%">
            {{ t('forgot.resetBtn') }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="bottom-link">
        <router-link to="/login">{{ t('forgot.backLogin') }}</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { sendSMS, resetPassword } from '@/api/auth'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const formRef = ref<FormInstance>()
const form = reactive({
  phone: '',
  code: '',
  newPassword: '',
  confirmPwd: '',
})

const validateConfirm = (_rule: any, value: string, callback: any) => {
  if (value !== form.newPassword) {
    callback(new Error(t('forgot.mismatch')))
  } else {
    callback()
  }
}

const rules: FormRules = {
  phone: [
    { required: true, message: t('auth.phonePlaceholder'), trigger: 'blur' },
    { pattern: /^1[3-9]\d{9}$/, message: t('forgot.phoneInvalid'), trigger: 'blur' },
  ],
  code: [{ required: true, message: t('auth.codePlaceholder'), trigger: 'blur' }],
  newPassword: [
    { required: true, message: t('forgot.newPwdRule'), trigger: 'blur' },
    { min: 6, message: t('forgot.newPwdLen'), trigger: 'blur' },
  ],
  confirmPwd: [
    { required: true, message: t('forgot.confirmPwd'), trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' },
  ],
}

async function handleSendCode() {
  const valid = await formRef.value?.validateField('phone').catch(() => false)
  if (!valid) return

  try {
    const res = await sendSMS(form.phone)
    ElMessage.success(t('auth.codeSent'))
    if (res.data?.dev_code) {
      form.code = res.data.dev_code
      ElMessage({ message: `【DEV】${res.data.dev_code}`, type: 'warning', showClose: true, duration: 6000 })
    }
    countdown.value = 60
    timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0 && timer) clearInterval(timer)
    }, 1000)
  } catch {
    // handled
  }
}

async function handleReset() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await resetPassword(form.phone, form.code, form.newPassword)
    if (res.code === 0) {
      ElMessage.success(t('forgot.resetSuccess'))
      router.push('/login')
    }
  } catch {
    // handled
  } finally {
    loading.value = false
  }
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

.sms-row {
  display: flex;
  align-items: center;
  width: 100%;
}

.bottom-link {
  text-align: center;
  margin-top: 16px;
  font-size: 14px;
}

.bottom-link a {
  color: #409eff;
  text-decoration: none;
}
</style>
