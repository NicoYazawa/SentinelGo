<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import request from '@/utils/request'
import { removeToken, getUsername } from '@/utils/auth'

const { t } = useI18n()
const router = useRouter()
const saving = ref(false)
const form = reactive({ current_password: '', new_password: '', confirm_password: '' })
async function submit() {
  if (saving.value) return
  if (!form.current_password) { ElMessage.warning(t('changePwd.enterTemp')); return }
  if (Array.from(form.new_password).length < 15 || new TextEncoder().encode(form.new_password).length > 72) {
    ElMessage.warning(t('changePwd.pwdLenRule')); return
  }
  if (form.new_password !== form.confirm_password) { ElMessage.warning(t('changePwd.mismatch')); return }
  saving.value = true
  try {
    await request.post('/auth/password/initialize', form)
    form.current_password = ''; form.new_password = ''; form.confirm_password = ''
    removeToken()
    ElMessage.success(t('changePwd.setSuccess'))
    await router.replace('/login')
  } catch { /* The request interceptor displays server errors. */ }
  finally { saving.value = false }
}
async function logout() {
  try { await request.post('/auth/logout') } catch { }
  removeToken()
  await router.replace('/login')
}
</script>

<template>
  <main class="setup-page">
    <section class="setup-card">
      <p class="brand">SentinelGo</p>
      <h1>{{ t('changePwd.title') }}</h1>
      <p class="description">{{ getUsername() }}{{ t('changePwd.usingTempPwd') }}</p>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item :label="t('changePwd.tempPassword')">
          <el-input v-model="form.current_password" type="password" show-password autocomplete="current-password" size="large" />
        </el-form-item>
        <el-form-item :label="t('changePwd.newPassword')">
          <el-input v-model="form.new_password" type="password" show-password autocomplete="new-password" size="large" />
        </el-form-item>
        <p class="hint">{{ t('changePwd.hint') }}</p>
        <el-form-item :label="t('changePwd.confirmNew')">
          <el-input v-model="form.confirm_password" type="password" show-password autocomplete="new-password" size="large" />
        </el-form-item>
        <el-button type="primary" native-type="submit" size="large" :loading="saving" class="submit-button">{{ t('changePwd.saveBtn') }}</el-button>
      </el-form>
      <el-button text class="logout-button" :disabled="saving" @click="logout">{{ t('changePwd.logoutLater') }}</el-button>
    </section>
  </main>
</template>

<style scoped>
.setup-page { min-height: 100dvh; display: flex; align-items: center; justify-content: center; padding: 24px; background: #f3f6fa; color: #293b52; }
.setup-card { width: 440px; max-width: 100%; padding: 36px; background: #fff; border: 1px solid #e4eaf2; border-radius: 12px; box-shadow: 0 8px 32px #23354b0a; }
.brand { margin: 0 0 20px; color: #4274ad; font-weight: 650; font-size: 16px; }
h1 { margin: 0 0 12px; font-size: 24px; }
.description { margin: 0 0 28px; font-size: 14px; line-height: 1.7; color: #64748b; }
.hint { margin: -6px 0 18px; font-size: 12px; line-height: 1.6; color: #7b899b; }
.submit-button { width: 100%; }
.logout-button { display: flex; margin: 14px auto 0; }
</style>
