<template>
  <div class="profile-wrapper">
    <header class="profile-header">
      <el-button text @click="$router.push('/home')">← {{ t('profile.backHome') }}</el-button>
      <h1>{{ t('profile.title') }}</h1>
    </header>

    <main class="profile-main">
      <el-card class="avatar-card">
        <template #header><span>{{ t('profile.avatar') }}</span></template>
        <div class="avatar-section">
          <el-avatar :size="100" :src="avatarSrc" />
          <div class="avatar-upload">
            <el-upload
              :show-file-list="false"
              :before-upload="beforeUpload"
              :http-request="handleUpload"
              accept="image/*"
            >
              <el-button type="primary" :loading="uploading">{{ t('profile.changeAvatar') }}</el-button>
            </el-upload>
            <p class="upload-tip">{{ t('profile.uploadTip') }}</p>
          </div>
        </div>
      </el-card>

      <el-card class="info-card">
        <template #header><span>{{ t('profile.basicInfo') }}</span></template>
        <el-form ref="formRef" :model="form" label-width="80px" style="max-width: 480px">
          <el-form-item :label="t('auth.username')">
            <el-input v-model="profile.username" disabled />
          </el-form-item>
          <el-form-item :label="t('auth.nickname')">
            <el-input v-model="form.nickname" :placeholder="t('auth.nicknamePlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('auth.email')">
            <el-input v-model="form.email" :placeholder="t('auth.emailPlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('auth.phone')">
            <el-input v-model="form.phone" :placeholder="t('auth.phonePlaceholder')" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="saving" @click="handleSave">{{ t('profile.saveChanges') }}</el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getProfile, updateProfile, uploadAvatar } from '@/api/auth'

const { t } = useI18n()

const saving = ref(false)
const uploading = ref(false)

const profile = reactive({
  id: 0,
  username: '',
  avatar: '',
})

const avatarSrc = ref('')
const form = reactive({ nickname: '', email: '', phone: '' })

onMounted(async () => {
  try {
    const res = await getProfile()
    if (res.code === 0) {
      const u = res.data
      profile.id = u.id
      profile.username = u.username
      profile.avatar = u.avatar || ''
      avatarSrc.value = u.avatar || ''
      form.nickname = u.nickname || ''
      form.email = u.email || ''
      form.phone = u.phone || ''
    }
  } catch {
    // handled
  }
})

async function handleSave() {
  saving.value = true
  try {
    const res = await updateProfile(form.nickname, form.email, form.phone)
    if (res.code === 0) {
      ElMessage.success(t('profile.saveSuccess'))
    }
  } catch {
    // handled
  } finally {
    saving.value = false
  }
}

function beforeUpload(file: File) {
  const isImage = file.type.startsWith('image/')
  const isLt2M = file.size / 1024 / 1024 < 2
  if (!isImage) {
    ElMessage.error(t('profile.onlyImage'))
    return false
  }
  if (!isLt2M) {
    ElMessage.error(t('profile.imageTooLarge'))
    return false
  }
  return true
}

async function handleUpload(options: any) {
  uploading.value = true
  try {
    const res = await uploadAvatar(options.file)
    if (res.code === 0) {
      avatarSrc.value = res.data.avatar_url + '?t=' + Date.now()
      ElMessage.success(t('profile.avatarUpdated'))
    }
  } catch {
    // handled
  } finally {
    uploading.value = false
  }
}
</script>

<style scoped>
.profile-wrapper {
  min-height: 100vh;
  background: #f5f7fa;
}

.profile-header {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 20px 40px;
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.profile-header h1 {
  font-size: 18px;
  margin: 0;
  color: #303133;
}

.profile-main {
  max-width: 800px;
  margin: 24px auto;
  padding: 0 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.avatar-section {
  display: flex;
  align-items: center;
  gap: 24px;
}

.avatar-upload {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.upload-tip {
  color: #c0c4cc;
  font-size: 12px;
}
</style>
