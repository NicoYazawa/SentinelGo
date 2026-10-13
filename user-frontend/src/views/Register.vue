<template>
  <div class="login-wrapper">
    <div class="login-card">
      <h1 class="title">{{ t('auth.registerTitle') }}</h1>
      <p class="subtitle">{{ t('auth.registerSubtitle') }}</p>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item :label="t('auth.username')" prop="username">
          <el-input v-model="form.username" :placeholder="t('auth.usernamePlaceholder')" size="large" />
        </el-form-item>
        <el-form-item :label="t('auth.password')" prop="password">
          <el-input v-model="form.password" type="password" :placeholder="t('auth.passwordPlaceholder')" size="large" show-password />
        </el-form-item>
        <el-form-item :label="t('auth.confirmPwd')" prop="confirmPwd">
          <el-input v-model="form.confirmPwd" type="password" :placeholder="t('auth.confirmPwd')" size="large" show-password />
        </el-form-item>
        <el-form-item :label="t('auth.nickname')">
          <el-input v-model="form.nickname" :placeholder="t('auth.nicknamePlaceholder')" size="large" />
        </el-form-item>
        <el-form-item :label="t('auth.email')">
          <el-input v-model="form.email" :placeholder="t('auth.emailPlaceholder')" size="large" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" size="large" :loading="loading" @click="handleRegister" style="width: 100%">
            {{ t('auth.registerBtn') }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="bottom-link">
        <span>{{ t('auth.alreadyAccount') }}</span>
        <router-link to="/login">{{ t('auth.loginNow') }}</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { register } from '@/api/auth'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)

const formRef = ref<FormInstance>()
const form = reactive({
  username: '',
  password: '',
  confirmPwd: '',
  nickname: '',
  email: '',
})

const validateConfirm = (_rule: any, value: string, callback: any) => {
  if (value !== form.password) {
    callback(new Error(t('auth.pwdMismatch')))
  } else {
    callback()
  }
}

const rules: FormRules = {
  username: [
    { required: true, message: t('auth.usernameRule'), trigger: 'blur' },
    { min: 3, max: 32, message: t('auth.usernameLenRule'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('auth.passwordRule'), trigger: 'blur' },
    { min: 6, max: 64, message: t('auth.passwordLenRule'), trigger: 'blur' },
  ],
  confirmPwd: [
    { required: true, message: t('auth.confirmPwdRule'), trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' },
  ],
}

async function handleRegister() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    const res = await register({
      username: form.username,
      password: form.password,
      nickname: form.nickname,
      email: form.email,
    })
    if (res.code === 0) {
      ElMessage.success(t('auth.registerSuccess'))
      router.push('/login')
    }
  } catch {
    // handled by interceptor
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
