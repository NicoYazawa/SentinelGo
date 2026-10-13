<!--
  ProfileDialog.vue — 个人中心弹窗（点击右上角用户名下拉中的「个人中心」打开）。

  功能：
    - 修改当前用户的昵称、邮箱、手机号、时区
    - 保存时调用 PATCH /api/v1/user/profile
    - 时区设置后通过 setUserTimezone 同步全局时区（影响全站时间展示）
-->
<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { getProfile, updateProfile } from '@/api/user'
import { getToken } from '@/utils/auth'
import { setUserTimezone } from '@/utils/timezone'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [boolean]; saved: [] }>()

const visible = ref(props.modelValue)
const loading = ref(false)
const form = reactive({ nickname: '', email: '', phone: '', timezone: 'Asia/Shanghai' })

const TIMEZONES = [
  'Asia/Shanghai', 'Asia/Tokyo', 'Asia/Singapore', 'Asia/Kolkata', 'Asia/Dubai',
  'Europe/London', 'Europe/Paris', 'Europe/Moscow', 'America/New_York', 'America/Los_Angeles',
  'America/Sao_Paulo', 'Australia/Sydney', 'UTC',
]

watch(() => props.modelValue, (v) => {
  visible.value = v
  if (v) loadProfile()
})
watch(visible, (v) => emit('update:modelValue', v))

async function loadProfile() {
  const token = getToken()
  if (!token) return
  loading.value = true
  try {
    const res = await getProfile()
    if (res.code === 0) {
      const u = res.data
      form.nickname = u.nickname || ''
      form.email = u.email || ''
      form.phone = u.phone || ''
      form.timezone = u.timezone || 'Asia/Shanghai'
      if (u.timezone) setUserTimezone(u.timezone)
    } else {
      ElMessage.error(t('profile.loadFailed'))
    }
  } catch {
    ElMessage.error(t('profile.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  loading.value = true
  try {
    const res = await updateProfile({
      nickname: form.nickname || undefined,
      email: form.email || undefined,
      phone: form.phone || undefined,
      timezone: form.timezone || undefined,
    })
    if (res.code === 0) {
      if (form.timezone) setUserTimezone(form.timezone)
      ElMessage.success(t('profile.saveSuccess'))
      emit('saved')
      visible.value = false
    } else {
      ElMessage.error(res.msg || t('profile.loadFailed'))
    }
  } catch {
    // 拦截器已对服务端错误进行提示，这里仅避免未捕获的 Promise 拒绝。
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" :title="t('profile.title')" width="460px" :close-on-click-modal="false">
    <el-form label-width="80px">
      <el-form-item :label="t('profile.nickname')">
        <el-input v-model="form.nickname" :placeholder="t('profile.nicknamePlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('profile.email')">
        <el-input v-model="form.email" :placeholder="t('profile.emailPlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('profile.phone')">
        <el-input v-model="form.phone" :placeholder="t('profile.phonePlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('profile.timezone')">
        <el-select v-model="form.timezone" :placeholder="t('profile.timezonePlaceholder')" style="width: 100%">
          <el-option v-for="tz in TIMEZONES" :key="tz" :label="tz" :value="tz" />
        </el-select>
      </el-form-item>
      <el-alert :title="t('profile.tip')" type="info" :closable="false" show-icon style="margin-top: 8px" />
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('user.cancel') }}</el-button>
      <el-button type="primary" :loading="loading" @click="save">{{ t('user.confirm') }}</el-button>
    </template>
  </el-dialog>
</template>
