<!--
  Sidebar.vue — 可折叠侧边栏组件。

  功能：
    - 从后端 API 获取菜单树并渲染为 el-menu
    - 支持多级菜单（目录 M + 子菜单 C 的嵌套结构）
    - 菜单类型为 'M' 且有子节点 → el-sub-menu（可展开）
    - 菜单类型为 'C' 或无子节点 → el-menu-item（直接跳转）
    - API 请求失败时降级使用硬编码的默认菜单结构
    - 图标映射：通过 iconMap 将菜单 icon 字段映射到 Element Plus 图标组件
-->
<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getMenus, type MenuItem } from '@/api/menu'
import { HomeFilled, Setting, User, Avatar, Menu, Key, Monitor, Document } from '@element-plus/icons-vue'

const { t } = useI18n()

// 系统菜单名 i18n：按 path 映射到菜单文案键，未命中则回退到后端返回的原始名称。
const menuKeyByPath: Record<string, string> = {
  '/dashboard': 'menu.dashboard',
  '/system': 'menu.system',
  '/system/users': 'menu.users',
  '/system/roles': 'menu.roles',
  '/system/menus': 'menu.menus',
  '/system/permissions': 'menu.permissions',
  '/system/monitor': 'menu.monitor',
  '/system/audit': 'menu.audit',
}
function menuTitle(path: string, name: string): string {
  const key = menuKeyByPath[path]
  if (!key) return name
  const translated = t(key)
  return translated === key ? name : translated
}

const router = useRouter()
const route = useRoute()
defineProps<{ collapse: boolean }>()

/** 从 API 获取的菜单树数据 */
const menus = ref<MenuItem[]>([])
/** 当前激活的菜单项（高亮） */
const activeMenu = ref(route.path)
const openGroups = computed(() => menus.value
  .filter(menu => menu.children?.some(child => child.path === route.path))
  .map(menu => 'sub-' + menu.id))

// 路由变化时同步更新激活菜单
watch(() => route.path, (path) => { activeMenu.value = path })

/** 菜单 icon 字段 → Element Plus 图标组件的映射表 */
const iconMap: Record<string, any> = {
  HomeFilled, Setting, User, UserFilled: User, Avatar, Menu, Key, Monitor, Document,
}

/** 根据 icon 名称获取对应的 Element Plus 图标组件 */
function getIcon(name?: string) {
  return name ? iconMap[name] || null : null
}

onMounted(async () => {
  try {
    const res = await getMenus()
    menus.value = res.data || []
  } catch {
    // API 不可用时降级使用默认菜单结构
    menus.value = [
      {
        id: 1, parent_id: 0, name: '首页', path: '/dashboard', icon: 'HomeFilled',
        sort: 1, type: 'C', status: 1, hidden: false,
      },
      {
        id: 2, parent_id: 0, name: '系统管理', path: '/system', icon: 'Setting',
        sort: 2, type: 'M', status: 1, hidden: false,
        children: [
          { id: 3, parent_id: 2, name: '用户管理', path: '/system/users', icon: 'User', sort: 1, type: 'C', status: 1, hidden: false },
          { id: 4, parent_id: 2, name: '角色管理', path: '/system/roles', icon: 'Avatar', sort: 2, type: 'C', status: 1, hidden: false },
          { id: 5, parent_id: 2, name: '菜单管理', path: '/system/menus', icon: 'Menu', sort: 3, type: 'C', status: 1, hidden: false },
          { id: 6, parent_id: 2, name: '权限管理', path: '/system/permissions', icon: 'Key', sort: 4, type: 'C', status: 1, hidden: false },
          { id: 7, parent_id: 2, name: '系统监控', path: '/system/monitor', icon: 'Monitor', sort: 5, type: 'C', status: 1, hidden: false },
        ],
      },
    ]
  }
})

/** 菜单项点击 → 路由跳转 */
function handleSelect(index: string) {
  router.push(index)
}
</script>

<template>
  <el-menu
    class="sidebar-menu"
    :default-active="activeMenu"
    :default-openeds="openGroups"
    :collapse="collapse"
    :collapse-transition="false"
    background-color="#17283e"
    text-color="#a9b9cc"
    active-text-color="#d4e7ff"
    router
    @select="handleSelect"
  >
    <template v-for="menu in menus" :key="menu.id">
      <!-- 目录类型且有子菜单 → 可展开的子菜单组 -->
      <el-sub-menu v-if="menu.type === 'M' && menu.children?.length" :index="'sub-' + menu.id" :aria-label="menuTitle(menu.path, menu.name)">
        <template #title>
          <el-icon v-if="getIcon(menu.icon)"><component :is="getIcon(menu.icon)" /></el-icon>
          <span>{{ menuTitle(menu.path, menu.name) }}</span>
        </template>
        <el-menu-item v-for="child in menu.children" :key="child.id" :index="child.path" :aria-label="menuTitle(child.path, child.name)">
          <el-icon v-if="getIcon(child.icon)"><component :is="getIcon(child.icon)" /></el-icon>
          <span>{{ menuTitle(child.path, child.name) }}</span>
        </el-menu-item>
      </el-sub-menu>
      <!-- 菜单类型或无子菜单 → 直接跳转的单菜单项 -->
      <el-menu-item v-else :index="menu.path" :aria-label="menuTitle(menu.path, menu.name)">
        <el-icon v-if="getIcon(menu.icon)"><component :is="getIcon(menu.icon)" /></el-icon>
        <span>{{ menuTitle(menu.path, menu.name) }}</span>
      </el-menu-item>
    </template>
  </el-menu>
</template>

<style scoped>
.el-menu {
  border-right: none;
  flex: 1;
  min-height: 0;
  padding: 16px 10px;
  overflow-y: auto;
  overflow-x: hidden;
}
.el-menu--collapse { width: 76px; padding: 16px 10px; }
.sidebar-menu :deep(.el-menu) { background: transparent; }
.sidebar-menu :deep(.el-menu-item), .sidebar-menu :deep(.el-sub-menu__title) {
  height: 44px;
  line-height: 44px;
  border-radius: 7px;
  margin-bottom: 5px;
  font-size: 13px;
}
.sidebar-menu :deep(.el-sub-menu .el-menu-item) { min-width: 0; }
.sidebar-menu :deep(.el-icon) { font-size: 18px; color: #8fa7c1; }
.sidebar-menu :deep(.el-menu-item:hover), .sidebar-menu :deep(.el-sub-menu__title:hover) { background: #213852; color: #e2eaf5; }
.sidebar-menu :deep(.el-menu-item.is-active) {
  position: relative;
  background: #284b70;
  color: #e2efff;
  font-weight: 600;
}
.sidebar-menu :deep(.el-menu-item.is-active)::before {
  content: '';
  position: absolute;
  left: 0;
  top: 13px;
  width: 3px;
  height: 18px;
  border-radius: 2px;
  background: #91bcec;
}
.sidebar-menu :deep(.el-menu-item.is-active .el-icon) { color: #b8d9ff; }
.sidebar-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title) { color: #cbdbee; }
.sidebar-menu :deep(.el-sub-menu.is-active > .el-sub-menu__title .el-icon) { color: #abc5e2; }
</style>
