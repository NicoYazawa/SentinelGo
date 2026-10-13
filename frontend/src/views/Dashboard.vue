<!--
  Dashboard.vue — 管理端首页仪表盘。

  内容：
    - 欢迎区 + 手动刷新
    - 核心 KPI 卡片（8 项：用户/角色/权限/菜单/今日新增/今日操作/登录/失败登录）
    - 安全与审计看板
  数据来源：GET /api/v1/dashboard/stats（后端聚合，避免多请求）
-->
<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import {
  User, UserFilled, Avatar, Key, Menu as MenuIcon, Document,
  CircleCheck, Warning, Refresh, ArrowRight,
} from '@element-plus/icons-vue'
import { ElCollapseTransition } from 'element-plus'
import { getDashboardStats, type DashboardStats } from '@/api/dashboard'
import { formatDateTime as fmtTime } from '@/utils/datetime'

const { t } = useI18n()
const router = useRouter()
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)
// 「失败操作」总类的展开状态
const expandedFailedOps = ref(false)

let timer: ReturnType<typeof setInterval> | null = null

// KPI 卡片配置（按当前数据动态着色）
const kpiCards = computed(() => {
  const s = stats.value
  if (!s) return []
  return [
    { label: t('dashboard.kpiUsers'), value: s.total_users, icon: User, color: '#409eff', to: '/system/users' },
    { label: t('dashboard.kpiTodayUsers'), value: s.today_new_users, icon: UserFilled, color: '#67c23a', to: '/system/users' },
    { label: t('dashboard.kpiRoles'), value: s.total_roles, icon: Avatar, color: '#9254de', to: '/system/roles' },
    { label: t('dashboard.kpiPerms'), value: s.total_permissions, icon: Key, color: '#e6a23c', to: '/system/permissions' },
    { label: t('dashboard.kpiMenus'), value: s.total_menus, icon: MenuIcon, color: '#13c2c2', to: '/system/menus' },
    { label: t('dashboard.kpiTodayOps'), value: s.today_operations, icon: Document, color: '#409eff', to: '/system/audit' },
    { label: t('dashboard.kpiTodayLogins'), value: s.today_logins, icon: CircleCheck, color: '#67c23a', to: '/system/audit' },
    {
      label: t('dashboard.kpiFailedLogins'),
      value: s.today_failed_logins,
      icon: Warning,
      color: s.today_failed_logins > 0 ? '#f56c6c' : '#909399',
      to: '/system/audit',
      alert: s.today_failed_logins > 0,
    },
  ]
})

// HTTP 方法 -> Tag 颜色
function methodType(method: string): 'success' | 'danger' | 'warning' | 'info' | '' {
  switch (method) {
    case 'GET': return 'success'
    case 'POST': return ''
    case 'PUT': return 'warning'
    case 'DELETE': return 'danger'
    default: return 'info'
  }
}

async function loadStats() {
  loading.value = true
  try {
    const res = await getDashboardStats()
    stats.value = res.data
  } catch {
    ElMessage.error(t('dashboard.loadFailed'))
  } finally {
    loading.value = false
  }
}

function goTo(path: string) {
  router.push(path)
}

onMounted(() => {
  loadStats()
  // 每 30 秒自动刷新
  timer = setInterval(loadStats, 30000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="dashboard">
    <!-- 欢迎区 -->
    <div class="welcome">
      <div>
        <h2>{{ t('dashboard.welcome') }} 👋</h2>
        <p class="sub">{{ t('dashboard.sub') }}</p>
      </div>
      <el-button :icon="Refresh" :loading="loading" @click="loadStats">{{ t('dashboard.refresh') }}</el-button>
    </div>

    <!-- 核心 KPI 卡片 -->
    <el-row :gutter="16" v-loading="loading">
      <el-col v-for="card in kpiCards" :key="card.label" :xs="12" :sm="12" :md="6" :lg="6">
        <el-card
          class="kpi-card"
          shadow="hover"
          :body-style="{ cursor: 'pointer' }"
          @click="goTo(card.to)"
        >
          <div class="kpi-inner">
            <div class="kpi-icon" :style="{ backgroundColor: card.color + '1a', color: card.color }">
              <el-icon :size="26"><component :is="card.icon" /></el-icon>
            </div>
            <div class="kpi-text">
              <div class="kpi-value" :style="{ color: card.color }">
                {{ card.value }}
                <el-tag v-if="card.alert" type="danger" size="small" effect="dark" class="kpi-alert">{{ t('dashboard.alert') }}</el-tag>
              </div>
              <div class="kpi-label">{{ card.label }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 安全与审计看板 -->
    <el-row :gutter="16" class="board-row">
      <!-- 安全概览 -->
      <el-col :xs="24" :md="8">
        <el-card class="board-card" shadow="never">
          <template #header><span class="card-title">🔐 {{ t('dashboard.securityOverview') }}</span></template>
          <div class="sec-item">
            <span>{{ t('dashboard.loginSuccess') }}</span>
            <strong class="ok">{{ stats?.today_logins ?? 0 }}</strong>
          </div>

          <!-- 失败操作：总类（含登录失败等子类），点击展开查看子类明细 -->
          <div class="sec-item sec-toggle" @click="expandedFailedOps = !expandedFailedOps">
            <span class="toggle-label">
              {{ t('dashboard.failedOps') }}
              <el-icon class="caret" :class="{ open: expandedFailedOps }"><ArrowRight /></el-icon>
            </span>
            <strong :class="(stats?.today_failed_ops ?? 0) > 0 ? 'danger' : ''">
              {{ stats?.today_failed_ops ?? 0 }}
            </strong>
          </div>

          <el-collapse-transition>
            <div v-if="expandedFailedOps" class="sec-detail">
              <div class="sec-sub">
                <span>├ {{ t('dashboard.failedLogin') }}</span>
                <strong :class="(stats?.today_failed_logins ?? 0) > 0 ? 'danger' : ''">
                  {{ stats?.today_failed_logins ?? 0 }}
                </strong>
              </div>
              <div class="sec-sub">
                <span>└ {{ t('dashboard.otherFailed') }}</span>
                <strong :class="((stats?.today_failed_ops ?? 0) - (stats?.today_failed_logins ?? 0)) > 0 ? 'danger' : ''">
                  {{ (stats?.today_failed_ops ?? 0) - (stats?.today_failed_logins ?? 0) }}
                </strong>
              </div>
              <div class="sec-detail-list">
                <div v-if="!(stats?.recent_failed_ops?.length)" class="detail-empty">{{ t('dashboard.noDetail') }}</div>
                <div v-for="op in (stats?.recent_failed_ops || [])" :key="op.id" class="op-row">
                  <span class="op-mod">{{ op.module }} / {{ op.operation }}</span>
                  <span class="op-ip">{{ op.ip }}</span>
                  <span class="op-time">{{ fmtTime(op.created_at) }}</span>
                </div>
              </div>
            </div>
          </el-collapse-transition>

          <el-alert
            v-if="(stats?.today_failed_ops ?? 0) > 0"
            type="warning"
            :closable="false"
            :title="t('dashboard.anomalyAlert')"
            class="sec-alert"
          />
        </el-card>
      </el-col>

      <!-- 最近异常操作 -->
      <el-col :xs="24" :md="16">
        <el-card class="board-card" shadow="never">
          <template #header>
            <span class="card-title">⚠️ {{ t('dashboard.recentAnomaly') }}</span>
            <el-button link type="primary" class="more-btn" @click="goTo('/system/audit')">{{ t('dashboard.viewAll') }}</el-button>
          </template>
          <el-table :data="stats?.recent_failed_ops || []" size="small" max-height="300" :empty-text="t('dashboard.noAnomaly')">
            <el-table-column prop="username" :label="t('dashboard.operator')" width="100" />
            <el-table-column prop="module" :label="t('dashboard.module')" width="80" />
            <el-table-column prop="operation" :label="t('dashboard.operation')" min-width="110" />
            <el-table-column prop="ip" :label="t('dashboard.ip')" width="130" />
            <el-table-column :label="t('dashboard.time')" width="160">
              <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
            </el-table-column>
            <el-table-column prop="error_msg" :label="t('dashboard.errorMsg')" min-width="160" show-overflow-tooltip />
          </el-table>
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近操作动态 -->
    <el-card class="board-card" shadow="never">
      <template #header>
        <span class="card-title">📋 {{ t('dashboard.recentOps') }}</span>
        <el-button link type="primary" class="more-btn" @click="goTo('/system/audit')">{{ t('dashboard.viewAll') }}</el-button>
      </template>
      <el-table :data="stats?.recent_operations || []" size="small" max-height="360" :empty-text="t('dashboard.noOps')">
        <el-table-column prop="username" :label="t('dashboard.operator')" width="110" />
        <el-table-column prop="module" :label="t('dashboard.module')" width="80" />
        <el-table-column prop="operation" :label="t('dashboard.operation')" min-width="120" />
        <el-table-column :label="t('dashboard.method')" width="80">
          <template #default="{ row }"><el-tag :type="methodType(row.method)" size="small">{{ row.method }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="ip" :label="t('dashboard.ip')" width="130" />
        <el-table-column :label="t('dashboard.result')" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? t('dashboard.success') : t('dashboard.failed') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('dashboard.time')" min-width="170">
          <template #default="{ row }">{{ fmtTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.dashboard { display: flex; flex-direction: column; gap: 16px; }

.welcome {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: linear-gradient(135deg, #409eff 0%, #66b1ff 100%);
  color: #fff;
  padding: 20px 24px;
  border-radius: 8px;
}
.welcome h2 { margin: 0; font-weight: 600; }
.welcome .sub { margin: 4px 0 0; opacity: 0.85; font-size: 13px; }

.kpi-card { border: none; }
.kpi-inner { display: flex; align-items: center; gap: 14px; }
.kpi-icon {
  width: 52px; height: 52px;
  border-radius: 10px;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0;
}
.kpi-value { font-size: 26px; font-weight: 700; line-height: 1.1; display: flex; align-items: center; gap: 6px; }
.kpi-alert { margin-left: 4px; transform: scale(0.85); }
.kpi-label { font-size: 13px; color: #909399; margin-top: 4px; }

.board-row { margin-top: 0; }
.board-card { border: none; }
.card-title { font-weight: 600; color: #303133; }
.more-btn { float: right; padding: 0; }

.sec-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 4px;
  border-bottom: 1px solid #f0f2f5;
  font-size: 14px;
  color: #606266;
}
.sec-item strong { font-size: 20px; color: #303133; }
.sec-item strong.ok { color: #67c23a; }
.sec-item strong.danger { color: #f56c6c; }
.sec-toggle { cursor: pointer; user-select: none; }
.sec-toggle:hover { background: #f5f7fa; }
.toggle-label { display: inline-flex; align-items: center; gap: 4px; }
.caret { transition: transform 0.2s ease; color: #909399; }
.caret.open { transform: rotate(90deg); }

.sec-detail {
  padding: 4px 4px 8px;
  border-left: 2px solid #f0f2f5;
  margin-left: 8px;
}
.sec-sub {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 8px;
  font-size: 13px;
  color: #606266;
}
.sec-sub strong { font-size: 16px; }
.sec-detail-list {
  margin-top: 4px;
  max-height: 200px;
  overflow-y: auto;
  background: #fafafa;
  border-radius: 6px;
  padding: 4px 0;
}
.op-row {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 5px 10px;
  font-size: 12px;
  color: #606266;
  border-bottom: 1px dashed #eee;
}
.op-row:last-child { border-bottom: none; }
.op-mod { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.op-ip { color: #909399; }
.op-time { color: #909399; white-space: nowrap; }
.detail-empty { padding: 10px; text-align: center; color: #c0c4cc; font-size: 12px; }
.sec-alert { margin-top: 12px; }
</style>
