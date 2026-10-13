<!--
  MonitorDashboard.vue — 系统监控面板页面。

  内容：
    - 运行时指标（uptime / goroutines / 内存 / Go 版本 / CPU 核数 / HTTP 统计 / P99）
    - HTTP 请求统计表格（path × method × status 聚合）
  数据来源：GET /api/v1/monitor/stats
-->
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getMonitorStats, type MonitorStats, type HTTPMetric } from '@/api/monitor'

const { t } = useI18n()

const loading = ref(false)
const stats = ref<MonitorStats | null>(null)
const probing = ref(false)

async function loadStats() {
  loading.value = true
  try {
    const res = await getMonitorStats()
    stats.value = res.data
  } catch {
    ElMessage.error(t('dashboard.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function runProbe() {
  probing.value = true
  try {
    await loadStats()
  } finally {
    probing.value = false
  }
}

let timer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  loadStats()
  timer = setInterval(loadStats, 15000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

function fmtUptime(d: number): string {
  const days = Math.floor(d / 86400)
  const hours = Math.floor((d % 86400) / 3600)
  const minutes = Math.floor((d % 3600) / 60)
  const parts: string[] = []
  if (days > 0) parts.push(`${days}d`)
  if (hours > 0) parts.push(`${hours}h`)
  if (minutes > 0) parts.push(`${minutes}m`)
  return parts.join(' ') || '0m'
}
function fmtMem(mb: number): string {
  if (mb < 1024) return `${mb} MB`
  return `${(mb / 1024).toFixed(2)} GB`
}
function fmtP99(s: number): string {
  return `${(s * 1000).toFixed(1)} ms`
}
</script>

<template>
  <div class="monitor">
    <div class="monitor-head">
      <h2 class="page-title">{{ t('monitor.title') }}</h2>
      <el-button :icon="Refresh" :loading="probing" @click="runProbe">{{ t('dashboard.refresh') }}</el-button>
    </div>

    <!-- 运行时指标 -->
    <el-row :gutter="16" class="metric-row" v-loading="loading">
      <el-col :xs="12" :sm="8" :md="6" v-for="m in [
        { label: t('monitor.uptime'), value: fmtUptime(stats?.uptime_seconds ?? 0), color: '#409eff' },
        { label: t('monitor.goroutines'), value: stats?.num_goroutines ?? 0, color: '#67c23a' },
        { label: t('monitor.memory'), value: fmtMem(stats?.memory_alloc_mb ?? 0) + ' / ' + fmtMem(stats?.memory_total_mb ?? 0), color: '#e6a23c' },
        { label: t('monitor.httpErrors'), value: stats?.http_errors ?? 0, color: '#f56c6c' },
        { label: t('monitor.totalRequests'), value: stats?.http_total ?? 0, color: '#9254de' },
        { label: t('monitor.cpuCores'), value: stats?.num_cpu ?? 0, color: '#13c2c2' },
        { label: t('monitor.p99'), value: fmtP99(stats?.http_p99_seconds ?? 0), color: '#eb2f96' },
        { label: t('monitor.goVersion'), value: stats?.go_version ?? '-', color: '#722ed1' },
      ]" :key="m.label">
        <el-card shadow="hover" class="metric-card">
          <div class="metric-label">{{ m.label }}</div>
          <div class="metric-value" :style="{ color: m.color }">{{ m.value }}</div>
        </el-card>
      </el-col>
    </el-row>

    <!-- HTTP 请求统计 -->
    <el-card shadow="never" class="http-card">
      <template #header><span class="card-title">📊 {{ t('monitor.httpStats') }}</span></template>
      <el-table :data="(stats?.http_metrics || []) as HTTPMetric[]" size="small" :empty-text="t('audit.noData')">
        <el-table-column prop="path" :label="t('audit.path')" min-width="240" show-overflow-tooltip />
        <el-table-column prop="method" :label="t('audit.method')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.method === 'GET' ? 'success' : row.method === 'POST' ? 'primary' : row.method === 'PUT' ? 'warning' : 'danger'" size="small">{{ row.method }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" :label="t('monitor.statusCode')" width="100" />
        <el-table-column prop="count" :label="t('monitor.count')" width="100" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.monitor { display: flex; flex-direction: column; gap: 16px; }
.monitor-head { display: flex; justify-content: space-between; align-items: center; }
.page-title { margin: 0; font-size: 20px; font-weight: 600; color: #303133; }
.metric-row { margin-top: 0; }
.metric-card { border: none; text-align: center; }
.metric-label { font-size: 13px; color: #909399; margin-bottom: 8px; }
.metric-value { font-size: 22px; font-weight: 700; }
.http-card { border: none; }
.card-title { font-weight: 600; color: #303133; }
</style>
