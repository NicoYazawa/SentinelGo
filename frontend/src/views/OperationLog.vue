<!--
  OperationLog.vue — 操作审计日志页面。

  功能：
    - 操作日志分页查询（操作人 / 模块 / 结果 / 时间范围筛选）
    - 查看单条日志详情（操作人、UA、链路 TraceID、错误信息等）
-->
<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Search, Refresh, View } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getOperationLogs, type OperationLog } from '@/api/audit'
import { formatDateTime as fmtDateTime } from '@/utils/datetime'

const { t } = useI18n()

const loading = ref(false)
const logList = ref<OperationLog[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)

const moduleOptions = ['用户', '角色', '菜单', '权限', '认证', '个人中心', '其他']

const searchForm = reactive({
  username: '',
  module: '',
  status: 0 as number,
  dateRange: [] as string[],
})

// ---- 详情抽屉 ----
const detailVisible = ref(false)
const currentLog = ref<OperationLog | null>(null)

async function loadList() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      username: searchForm.username,
      module: searchForm.module,
      status: searchForm.status,
      page: currentPage.value,
      page_size: pageSize.value,
    }
    if (searchForm.dateRange && searchForm.dateRange.length === 2) {
      params.start = searchForm.dateRange[0] + ' 00:00:00'
      params.end = searchForm.dateRange[1] + ' 23:59:59'
    }
    const res = await getOperationLogs(params)
    logList.value = res.data.list || []
    total.value = res.data.total || 0
  } catch {
    ElMessage.error(t('audit.queryFailed'))
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  currentPage.value = 1
  loadList()
}

function handleReset() {
  searchForm.username = ''
  searchForm.module = ''
  searchForm.status = 0
  searchForm.dateRange = []
  handleSearch()
}

function showDetail(row: OperationLog) {
  currentLog.value = row
  detailVisible.value = true
}

onMounted(loadList)
</script>

<template>
  <div class="page-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="search-box">
            <el-input v-model="searchForm.username" :placeholder="t('audit.operator')" class="search-input" clearable @keyup.enter="handleSearch" />
            <el-select v-model="searchForm.module" :placeholder="t('audit.module')" clearable class="search-select">
              <el-option v-for="m in moduleOptions" :key="m" :label="m" :value="m" />
            </el-select>
            <el-select v-model="searchForm.status" :placeholder="t('audit.result')" clearable class="search-select">
              <el-option :label="t('audit.all')" :value="0" />
              <el-option :label="t('audit.success')" :value="1" />
              <el-option :label="t('audit.failed')" :value="2" />
            </el-select>
            <el-date-picker
              v-model="searchForm.dateRange"
              type="daterange"
              :range-separator="t('audit.rangeSeparator')"
              :start-placeholder="t('audit.startPlaceholder')"
              :end-placeholder="t('audit.endPlaceholder')"
              value-format="YYYY-MM-DD"
            />
            <el-button type="primary" @click="handleSearch"><el-icon><Search /></el-icon>{{ t('crud.search') }}</el-button>
            <el-button @click="handleReset"><el-icon><Refresh /></el-icon>{{ t('crud.reset') }}</el-button>
          </div>
        </div>
      </template>

      <el-table :data="logList" v-loading="loading" border>
        <el-table-column type="index" :label="t('audit.colIndex')" width="60" align="center" />
        <el-table-column prop="username" :label="t('audit.operator')" min-width="100" />
        <el-table-column prop="module" :label="t('audit.module')" width="100" />
        <el-table-column prop="operation" :label="t('audit.operation')" min-width="120" />
        <el-table-column prop="method" :label="t('audit.method')" width="80" />
        <el-table-column prop="path" :label="t('audit.path')" min-width="180" show-overflow-tooltip />
        <el-table-column prop="ip" :label="t('audit.ip')" width="130" />
        <el-table-column :label="t('audit.result')" width="80">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? t('audit.success') : t('audit.failed') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('audit.time')" width="170">
          <template #default="{ row }">{{ fmtDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column :label="t('audit.action')" width="90" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="showDetail(row)"><el-icon><View /></el-icon>{{ t('audit.detail') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <el-pagination
          v-model:current-page="currentPage" v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]" :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList" @current-change="loadList"
        />
      </div>
    </el-card>

    <!-- 详情抽屉 -->
    <el-drawer v-model="detailVisible" :title="t('audit.detailTitle')" size="420px">
      <el-descriptions v-if="currentLog" :column="1" border>
        <el-descriptions-item :label="t('audit.logId')">{{ currentLog.id }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.operator')">{{ currentLog.username || t('audit.anonymous') }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.module')">{{ currentLog.module }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.operation')">{{ currentLog.operation }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.method')">{{ currentLog.method }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.path')">{{ currentLog.path }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.clientIp')">{{ currentLog.ip }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.result')">
          <el-tag :type="currentLog.status === 1 ? 'success' : 'danger'" size="small">
            {{ currentLog.status === 1 ? t('audit.success') : t('audit.failed') }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item :label="t('audit.errorMsg')">{{ currentLog.error_msg || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.userAgent')">{{ currentLog.user_agent || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.traceId')">{{ currentLog.trace_id || '-' }}</el-descriptions-item>
        <el-descriptions-item :label="t('audit.opTime')">{{ fmtDateTime(currentLog.created_at) }}</el-descriptions-item>
      </el-descriptions>
    </el-drawer>
  </div>
</template>

<style scoped>
.page-container { /* wrapper */ }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.search-box { display: flex; gap: 10px; flex-wrap: wrap; }
.search-input { width: 160px; }
.search-select { width: 120px; }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
</style>
