<!--
  UserManagement.vue — 用户管理页面。

  功能：
    - 用户列表分页查询（关键词搜索 + 重置）
    - 新增/编辑用户（弹窗表单，编辑时用户名只读）
    - 删除用户（确认对话框）
    - 状态开关（启用/禁用）
    - 修改密码（独立弹窗，最少6位）
    - 分配角色（Checkbox Group 多选）
-->
<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { Plus, Edit, Delete, Search, Refresh, UserFilled, Key } from '@element-plus/icons-vue'
import {
  getUsers, createUser, updateUser, deleteUser, updateUserStatus, updateUserPassword,
  getUserRoles, assignUserRoles, type User,
} from '@/api/user'
import { getRoles, type Role } from '@/api/role'

const { t } = useI18n()

const loading = ref(false)
const userList = ref<User[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(10)

/** 搜索表单 */
const searchForm = reactive({ keyword: '' })

// ---- 新增/编辑弹窗 ----
const dialogVisible = ref(false)
const dialogTitle = ref(t('user.addDialog'))
const formRef = ref<FormInstance>()
const form = reactive({
  id: undefined as number | undefined,
  username: '', nickname: '', email: '', phone: '', password: '', status: 1,
})
const submitting = ref(false)

/** 表单校验规则 */
const rules: FormRules = {
  username: [{ required: true, message: t('user.usernameRule'), trigger: 'blur' }],
  password: [{ required: true, message: t('user.passwordRule'), trigger: 'blur' }, { min: 6, message: t('user.passwordMin'), trigger: 'blur' }],
}

// ---- 修改密码弹窗 ----
const passwordDialogVisible = ref(false)
const passwordFormRef = ref<FormInstance>()
const passwordForm = reactive({ id: 0, password: '' })
const passwordRules: FormRules = {
  password: [{ required: true, message: t('user.newPwdRule'), trigger: 'blur' }, { min: 6, message: t('user.newPwdMin'), trigger: 'blur' }],
}

// ---- 分配角色弹窗 ----
const roleDialogVisible = ref(false)
const allRoles = ref<Role[]>([])
const selectedRoles = ref<number[]>([])
const roleUserId = ref(0)

/** 加载用户列表 */
async function loadList() {
  loading.value = true
  try {
    const res = await getUsers({ keyword: searchForm.keyword, page: currentPage.value, page_size: pageSize.value })
    userList.value = res.data.list || []
    total.value = res.data.total || 0
  } finally { loading.value = false }
}

function handleSearch() { currentPage.value = 1; loadList() }
function handleReset() { searchForm.keyword = ''; handleSearch() }

/** 打开新增弹窗 */
function handleAdd() {
  dialogTitle.value = t('user.addDialog')
  form.id = undefined
  form.username = ''; form.nickname = ''; form.email = ''; form.phone = ''; form.password = ''; form.status = 1
  dialogVisible.value = true
}

/** 打开编辑弹窗（预填数据，密码留空） */
function handleEdit(row: User) {
  dialogTitle.value = t('user.editDialog')
  Object.assign(form, row)
  form.password = ''
  dialogVisible.value = true
}

/** 提交新增/编辑表单 */
async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const data: any = { ...form }
      if (!data.password) delete data.password  // 编辑时不传空密码
      if (form.id) {
        await updateUser(form.id, data)
        ElMessage.success(t('user.updateSuccess'))
      } else {
        await createUser(data)
        ElMessage.success(t('user.createSuccess'))
      }
      dialogVisible.value = false
      loadList()
    } finally { submitting.value = false }
  })
}

/** 删除用户（确认对话框） */
async function handleDelete(row: User) {
  try {
    await ElMessageBox.confirm(t('user.confirmDelete', { name: row.username }), t('common.tip'), { type: 'warning' })
    await deleteUser(row.id)
    ElMessage.success(t('user.deleteSuccess'))
    loadList()
  } catch { /* cancelled */ }
}

/** 切换用户状态（失败时回滚开关） */
async function handleStatusChange(row: User) {
  try {
    await updateUserStatus(row.id, row.status)
    ElMessage.success(t('user.statusUpdated'))
  } catch { row.status = row.status === 1 ? 0 : 1 }
}

/** 打开修改密码弹窗 */
function handleChangePassword(row: User) {
  passwordForm.id = row.id
  passwordForm.password = ''
  passwordDialogVisible.value = true
}

/** 提交修改密码 */
async function handlePasswordSubmit() {
  if (!passwordFormRef.value) return
  await passwordFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      await updateUserPassword(passwordForm.id, passwordForm.password)
      ElMessage.success(t('user.pwdUpdated'))
      passwordDialogVisible.value = false
    } finally { submitting.value = false }
  })
}

/** 打开分配角色弹窗（同时加载所有角色和用户已有角色） */
async function handleAssignRoles(row: User) {
  roleUserId.value = row.id
  const [rolesRes, userRolesRes] = await Promise.all([getRoles(), getUserRoles(row.id)])
  allRoles.value = rolesRes.data || []
  selectedRoles.value = (userRolesRes.data || []).map((r: Role) => r.id)
  roleDialogVisible.value = true
}

/** 提交角色分配 */
async function handleRoleSubmit() {
  submitting.value = true
  try {
    await assignUserRoles(roleUserId.value, selectedRoles.value)
    ElMessage.success(t('user.roleAssigned'))
    roleDialogVisible.value = false
  } finally { submitting.value = false }
}

onMounted(loadList)
</script>

<template>
  <div class="page-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <!-- 搜索栏 -->
          <div class="search-box">
            <el-input v-model="searchForm.keyword" :placeholder="t('user.searchPlaceholder')" class="search-input" clearable @keyup.enter="handleSearch">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-button type="primary" @click="handleSearch"><el-icon><Search /></el-icon>{{ t('crud.search') }}</el-button>
            <el-button @click="handleReset"><el-icon><Refresh /></el-icon>{{ t('crud.reset') }}</el-button>
          </div>
          <el-button type="primary" @click="handleAdd"><el-icon><Plus /></el-icon>{{ t('crud.add') }}</el-button>
        </div>
      </template>

      <!-- 用户列表 -->
      <el-table :data="userList" v-loading="loading" border>
        <el-table-column type="index" :label="t('user.colIndex')" width="60" align="center" />
        <el-table-column prop="username" :label="t('user.username')" min-width="100" />
        <el-table-column prop="nickname" :label="t('user.nickname')" min-width="100" />
        <el-table-column prop="email" :label="t('user.email')" min-width="150" show-overflow-tooltip />
        <el-table-column prop="phone" :label="t('user.phone')" min-width="120" />
        <el-table-column :label="t('user.status')" width="80">
          <template #default="{ row }">
            <el-switch v-model="row.status" :active-value="1" :inactive-value="0" @change="handleStatusChange(row)" />
          </template>
        </el-table-column>
        <el-table-column :label="t('user.action')" width="360" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="handleEdit(row)"><el-icon><Edit /></el-icon>{{ t('user.edit') }}</el-button>
            <el-button type="primary" link @click="handleAssignRoles(row)"><el-icon><UserFilled /></el-icon>{{ t('user.assignRoles') }}</el-button>
            <el-button type="primary" link @click="handleChangePassword(row)"><el-icon><Key /></el-icon>{{ t('user.changePwd') }}</el-button>
            <el-button type="danger" link @click="handleDelete(row)"><el-icon><Delete /></el-icon>{{ t('user.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页组件 -->
      <div class="pagination-container">
        <el-pagination
          v-model:current-page="currentPage" v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]" :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList" @current-change="loadList"
        />
      </div>
    </el-card>

    <!-- 新增/编辑弹窗 -->
    <el-dialog :title="dialogTitle" v-model="dialogVisible" width="500px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item :label="t('user.username')" prop="username">
          <el-input v-model="form.username" :placeholder="t('user.usernameRule')" :disabled="!!form.id" />
        </el-form-item>
        <el-form-item :label="t('user.formNickname')">
          <el-input v-model="form.nickname" :placeholder="t('user.formNickname')" />
        </el-form-item>
        <el-form-item :label="t('user.formEmail')">
          <el-input v-model="form.email" :placeholder="t('user.formEmail')" />
        </el-form-item>
        <el-form-item :label="t('user.formPhone')">
          <el-input v-model="form.phone" :placeholder="t('user.formPhone')" />
        </el-form-item>
        <el-form-item :label="t('user.formPassword')" prop="password">
          <el-input v-model="form.password" type="password" show-password :placeholder="form.id ? t('user.passwordKeep') : t('user.passwordPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">{{ t('user.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 修改密码弹窗 -->
    <el-dialog :title="t('user.changePwdDialog')" v-model="passwordDialogVisible" width="450px" :close-on-click-modal="false">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="80px">
        <el-form-item :label="t('user.newPassword')" prop="password">
          <el-input v-model="passwordForm.password" type="password" show-password :placeholder="t('user.newPwdRule')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button type="primary" @click="handlePasswordSubmit" :loading="submitting">{{ t('user.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 分配角色弹窗 -->
    <el-dialog :title="t('user.assignRoleDialog')" v-model="roleDialogVisible" width="500px" :close-on-click-modal="false">
      <el-checkbox-group v-model="selectedRoles">
        <el-checkbox v-for="role in allRoles" :key="role.id" :value="role.id" :label="role.id">
          {{ role.name }} ({{ role.code }})
        </el-checkbox>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="roleDialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button type="primary" @click="handleRoleSubmit" :loading="submitting">{{ t('user.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.page-container { /* wrapper for breadcrumb space */ }
.card-header { display: flex; justify-content: space-between; align-items: center; }
.search-box { display: flex; gap: 10px; }
.search-input { width: 240px; }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
</style>
