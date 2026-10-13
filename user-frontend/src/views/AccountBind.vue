<template>
  <div class="account-wrapper">
    <header class="account-header">
      <el-button text @click="$router.push('/home')">← {{ t('accountBind.backHome') }}</el-button>
      <h1>{{ t('accountBind.title') }}</h1>
    </header>

    <main class="account-main">
      <el-card class="channel-card">
        <template #header>
          <span>{{ t('accountBind.boundList') }}</span>
        </template>

        <el-table :data="channels" style="width: 100%">
          <el-table-column :label="t('accountBind.loginMethod')" width="150">
            <template #default="{ row }">
              <el-tag v-if="row.identity_type === 'password'" type="success">{{ t('accountBind.methodPassword') }}</el-tag>
              <el-tag v-else-if="row.identity_type === 'phone'" type="primary">{{ t('accountBind.methodPhone') }}</el-tag>
              <el-tag v-else-if="row.identity_type === 'github'" type="warning">{{ t('accountBind.methodGithub') }}</el-tag>
              <el-tag v-else>{{ row.identity_type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="identifier" :label="t('accountBind.identifier')" />
          <el-table-column :label="t('accountBind.action')" width="120">
            <template #default="{ row }">
              <el-button type="danger" text size="small" @click="handleUnbind(row.identity_type)"
                :disabled="row.identity_type === 'password'">
                {{ t('accountBind.unbind') }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-card class="bind-card">
        <template #header>
          <span>{{ t('accountBind.bindPhoneCard') }}</span>
        </template>
        <el-form :model="bindForm" label-width="80px" style="max-width: 480px">
          <el-form-item :label="t('auth.phone')">
            <el-input v-model="bindForm.phone" :placeholder="t('accountBind.phonePlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('auth.code')">
            <div class="sms-row">
              <el-input v-model="bindForm.code" :placeholder="t('accountBind.codePlaceholder')" style="flex: 1" />
              <el-button :disabled="bindCountdown > 0" @click="handleBindSendSMS" style="margin-left: 12px">
                {{ bindCountdown > 0 ? `${bindCountdown}s` : t('auth.sendCode') }}
              </el-button>
            </div>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="handleBindPhone">{{ t('accountBind.bindBtn') }}</el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { getChannels, bindPhone, unbindChannel, sendSMS } from '@/api/auth'
import type { ChannelInfo } from '@/api/auth'

const { t } = useI18n()

const channels = ref<ChannelInfo[]>([])
const bindForm = reactive({ phone: '', code: '' })
const bindCountdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

onMounted(async () => {
  try {
    const res = await getChannels()
    if (res.code === 0) {
      channels.value = res.data
    }
  } catch {
    // handled
  }
})

async function handleBindSendSMS() {
  if (!bindForm.phone) {
    ElMessage.warning(t('accountBind.enterPhone'))
    return
  }
  try {
    const res = await sendSMS(bindForm.phone, 'bind')
    ElMessage.success(t('auth.codeSent'))
    if (res.data?.dev_code) {
      bindForm.code = res.data.dev_code
      ElMessage({ message: `【DEV】${res.data.dev_code}`, type: 'warning', showClose: true, duration: 6000 })
    }
    bindCountdown.value = 60
    timer = setInterval(() => {
      bindCountdown.value--
      if (bindCountdown.value <= 0 && timer) clearInterval(timer)
    }, 1000)
  } catch {
    // handled
  }
}

async function handleBindPhone() {
  if (!bindForm.phone || !bindForm.code) {
    ElMessage.warning(t('accountBind.enterPhoneCode'))
    return
  }
  try {
    const res = await bindPhone(bindForm.phone, bindForm.code)
    if (res.code === 0) {
      ElMessage.success(t('accountBind.bindSuccess'))
      bindForm.phone = ''
      bindForm.code = ''
      // 刷新列表
      const r = await getChannels()
      if (r.code === 0) channels.value = r.data
    }
  } catch {
    // handled
  }
}

async function handleUnbind(type: string) {
  try {
    await ElMessageBox.confirm(t('accountBind.confirmUnbindMsg'), t('accountBind.confirmUnbindTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
    const res = await unbindChannel(type)
    if (res.code === 0) {
      ElMessage.success(t('accountBind.unbindSuccess'))
      channels.value = channels.value.filter((c) => c.identity_type !== type)
    }
  } catch {
    // cancelled or error
  }
}
</script>

<style scoped>
.account-wrapper {
  min-height: 100vh;
  background: #f5f7fa;
}

.account-header {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 20px 40px;
  background: #fff;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.account-header h1 {
  font-size: 18px;
  margin: 0;
  color: #303133;
}

.account-main {
  max-width: 800px;
  margin: 24px auto;
  padding: 0 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.sms-row {
  display: flex;
  align-items: center;
  width: 100%;
}
</style>
