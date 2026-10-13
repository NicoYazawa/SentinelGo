<!--
  RoleManagement.vue — 角色管理页面。

  功能：
    - 角色列表查询（关键词搜索）
    - 新增/编辑角色（编辑时 code 只读）
    - 删除角色（级联删除关联数据和 Casbin 策略）
    - 状态开关（启用/禁用）
    - 分配菜单（el-tree 多选，展示完整菜单树）
    - 分配权限（表格列出 API 端点 + 开关授权）
-->
<script setup lang="ts">
import { ref, reactive, onMounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { Plus, Edit, Delete, Search, Refresh, Menu, Lock } from '@element-plus/icons-vue'
import {
  getRoles, createRole, updateRole, deleteRole, updateRoleStatus,
  getRoleMenus, assignRoleMenus, assignRolePermissions, type Role,
} from '@/api/role'
import { getMenuTree, type MenuItem } from '@/api/menu'

const { t } = useI18n()

const loading = ref(false)
const roleList = ref<Role[]>([])
const searchForm = reactive({ keyword: '' })

// ---- 新增/编辑弹窗 ----
const dialogVisible = ref(false)
const dialogTitle = ref(t('role.addDialog'))
const formRef = ref<FormInstance>()
const form = reactive({ id: undefined as number | undefined, name: '', code: '', desc: '', status: 1 })
const submitting = ref(false)

const rules: FormRules = {
  name: [{ required: true, message: t('role.nameRule'), trigger: 'blur' }],
  code: [{ required: true, message: t('role.codeRule'), trigger: 'blur' }],
}

// ---- 分配菜单弹窗 ----
const menuDialogVisible = ref(false)
const menuTreeRef = ref()   // el-tree 组件引用
const menuTree = ref<MenuItem[]>([])
const checkedMenuIds = ref<number[]>([])
const menuRoleId = ref(0)

// ---- 分配权限弹窗 ----
const permDialogVisible = ref(false)
const permRoleId = ref(0)
const permList = ref<{ path: string; method: string; checked: boolean }[]>([])

/** 加载角色列表 */
async function loadList() {
  loading.value = true
  try {
    const res = await getRoles({ keyword: searchForm.keyword })
    roleList.value = res.data || []
  } finally { loading.value = false }
}

function handleSearch() { loadList() }
function handleReset() { searchForm.keyword = ''; handleSearch() }

function handleAdd() {
  dialogTitle.value = t('role.addDialog')
  form.id = undefined; form.name = ''; form.code = ''; form.desc = ''; form.status = 1
  dialogVisible.value = true
}

function handleEdit(row: Role) {
  dialogTitle.value = t('role.editDialog')
  Object.assign(form, row)
  dialogVisible.value = true
}

/** 提交新增/编辑 */
async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitting.value = true
    try {
      const data: any = { name: form.name, code: form.code, desc: form.desc }
      if (form.id) {
        await updateRole(form.id, data)
        ElMessage.success(t('user.updateSuccess'))
      } else {
        await createRole(data)
        ElMessage.success(t('user.createSuccess'))
      }
      dialogVisible.value = false
      loadList()
    } finally { submitting.value = false }
  })
}

async function handleDelete(row: Role) {
  try {
    await ElMessageBox.confirm(t('role.confirmDelete', { name: row.name }), t('common.tip'), { type: 'warning' })
    await deleteRole(row.id)
    ElMessage.success(t('user.deleteSuccess'))
    loadList()
  } catch { /* cancelled */ }
}

/** 切换角色状态 */
async function handleStatusChange(row: Role) {
  try {
    await updateRoleStatus(row.id, row.status)
    ElMessage.success(t('user.statusUpdated'))
  } catch { row.status = row.status === 1 ? 0 : 1 }
}

/** 打开分配菜单弹窗（并行加载菜单树 + 角色已有菜单） */
async function handleAssignMenus(row: Role) {
  menuRoleId.value = row.id
  const [treeRes, menusRes] = await Promise.all([getMenuTree(), getRoleMenus(row.id)])
  menuTree.value = treeRes.data || []
  checkedMenuIds.value = (menusRes.data || []).map((m: any) => m.id)
  menuDialogVisible.value = true
  // nextTick 后设置 el-tree 已选节点
  nextTick(() => {
    if (menuTreeRef.value) {
      menuTreeRef.value.setCheckedKeys([])
      menuTreeRef.value.setCheckedKeys(checkedMenuIds.value)
    }
  })
}

/** 提交菜单分配 */
async function handleMenuSubmit() {
  if (!menuTreeRef.value) return
  submitting.value = true
  try {
    // el-tree 的 getCheckedKeys 返回所有勾选节点的 key 列表
    await assignRoleMenus(menuRoleId.value, menuTreeRef.value.getCheckedKeys())
    ElMessage.success(t('role.menuAssigned'))
    menuDialogVisible.value = false
  } finally { submitting.value = false }
}

/** 所有 HTTP 方法（当前权限分配仅演示 /api/v1/users 的四种方法） */
const allMethods = ['GET', 'POST', 'PUT', 'DELETE']

/** 打开分配权限弹窗 */
async function handleAssignPermissions(row: Role) {
  permRoleId.value = row.id
  permList.value = allMethods.map(m => ({ path: '/api/v1/users', method: m, checked: false }))
  permDialogVisible.value = true
}

/** 提交权限策略分配（仅提交选中的） */
async function handlePermSubmit() {
  const perms = permList.value.filter(p => p.checked).map(p => ({ path: p.path, method: p.method }))
  if (perms.length === 0) { ElMessage.warning(t('role.permAtLeastOne')); return }
  submitting.value = true
  try {
    await assignRolePermissions(permRoleId.value, perms)
    ElMessage.success(t('role.permAssigned'))
    permDialogVisible.value = false
  } finally { submitting.value = false }
}

onMounted(loadList)
</script>

<template>
  <div class="page-container">
    <el-card>
      <template #header>
        <div class="card-header">
          <div class="search-box">
            <el-input v-model="searchForm.keyword" :placeholder="t('role.searchPlaceholder')" class="search-input" clearable @keyup.enter="handleSearch">
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-button type="primary" @click="handleSearch"><el-icon><Search /></el-icon>{{ t('crud.search') }}</el-button>
            <el-button @click="handleReset"><el-icon><Refresh /></el-icon>{{ t('crud.reset') }}</el-button>
          </div>
          <el-button type="primary" @click="handleAdd"><el-icon><Plus /></el-icon>{{ t('crud.add') }}</el-button>
        </div>
      </template>

      <!-- 角色列表 -->
      <el-table :data="roleList" v-loading="loading" border>
        <el-table-column prop="name" :label="t('role.name')" min-width="120" />
        <el-table-column prop="code" :label="t('role.code')" min-width="120" />
        <el-table-column prop="desc" :label="t('role.desc')" min-width="180" show-overflow-tooltip />
        <el-table-column :label="t('user.status')" width="80">
          <template #default="{ row }">
            <el-switch v-model="row.status" :active-value="1" :inactive-value="0" @change="handleStatusChange(row)" />
          </template>
        </el-table-column>
        <el-table-column :label="t('user.action')" width="360" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="handleEdit(row)"><el-icon><Edit /></el-icon>{{ t('user.edit') }}</el-button>
            <el-button type="primary" link @click="handleAssignMenus(row)"><el-icon><Menu /></el-icon>{{ t('role.assignMenus') }}</el-button>
            <el-button type="primary" link @click="handleAssignPermissions(row)"><el-icon><Lock /></el-icon>{{ t('role.assignPerms') }}</el-button>
            <el-button type="danger" link @click="handleDelete(row)"><el-icon><Delete /></el-icon>{{ t('user.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑弹窗 -->
    <el-dialog :title="dialogTitle" v-model="dialogVisible" width="500px" :close-on-click-modal="false">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item :label="t('role.formName')" prop="name">
          <el-input v-model="form.name" :placeholder="t('role.formName')" />
        </el-form-item>
        <el-form-item :label="t('role.formCode')" prop="code">
          <el-input v-model="form.code" :placeholder="t('role.formCode')" :disabled="!!form.id" />
        </el-form-item>
        <el-form-item :label="t('role.formDesc')">
          <el-input v-model="form.desc" type="textarea" :rows="3" :placeholder="t('role.formDesc')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">{{ t('user.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 分配菜单弹窗（el-tree 多选） -->
    <el-dialog :title="t('role.menuDialog')" v-model="menuDialogVisible" width="500px" :close-on-click-modal="false">
      <el-tree ref="menuTreeRef" :data="menuTree" show-checkbox node-key="id"
        :props="{ children: 'children', label: 'name' }" />
      <template #footer>
        <el-button @click="menuDialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button type="primary" @click="handleMenuSubmit" :loading="submitting">{{ t('user.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 分配权限弹窗（表格 + 开关） -->
    <el-dialog :title="t('role.permDialog')" v-model="permDialogVisible" width="550px" :close-on-click-modal="false">
      <el-table :data="permList" border>
        <el-table-column prop="path" :label="t('permission.path')" width="200" />
        <el-table-column prop="method" :label="t('permission.method')" width="100">
          <template #default="{ row }">
            <el-tag :type="row.method === 'GET' ? 'success' : row.method === 'POST' ? 'primary' : row.method === 'PUT' ? 'warning' : 'danger'" size="small">{{ row.method }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('permission.description')" width="100">
          <template #default="{ row }"><el-switch v-model="row.checked" /></template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="permDialogVisible = false">{{ t('user.cancel') }}</el-button>
        <el-button type="primary" @click="handlePermSubmit" :loading="submitting">{{ t('user.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.card-header { display: flex; justify-content: space-between; align-items: center; }
.search-box { display: flex; gap: 10px; }
.search-input { width: 240px; }
.pagination-container { margin-top: 20px; display: flex; justify-content: flex-end; }
</style>
