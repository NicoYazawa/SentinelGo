// 前端组件/视图测试共享：挂载全局（ElementPlus + 内存路由 + i18n）与 el-dialog 桩。
import { createRouter, createMemoryHistory } from 'vue-router'
import ElementPlus from 'element-plus'
import { i18n } from '../locales'

export const testRouter = createRouter({
  history: createMemoryHistory(),
  routes: [
    { path: '/', redirect: '/dashboard' },
    { path: '/dashboard', component: { template: '<div/>' } },
    { path: '/login', component: { template: '<div/>' } },
    { path: '/change-password', component: { template: '<div/>' } },
    { path: '/system/users', component: { template: '<div/>' } },
    { path: '/system/roles', component: { template: '<div/>' } },
    { path: '/system/menus', component: { template: '<div/>' } },
    { path: '/system/permissions', component: { template: '<div/>' } },
    { path: '/system/monitor', component: { template: '<div/>' } },
    { path: '/system/audit', component: { template: '<div/>' } },
  ],
})

// el-dialog 默认使用 Teleport 到 body，测试内不便定位；用内联桩替代以暴露插槽内容。
const ElDialogStub = {
  name: 'ElDialog',
  props: ['modelValue', 'title'],
  emits: ['update:modelValue'],
  template: `<div v-if="modelValue" class="el-dialog-stub" role="dialog" :aria-label="title"><div class="el-dialog__body"><slot/></div><div class="el-dialog__footer"><slot name="footer"/></div></div>`,
}

// el-drawer 同样 Teleport；内联桩暴露默认插槽内容。
const ElDrawerStub = {
  name: 'ElDrawer',
  props: ['modelValue', 'title'],
  emits: ['update:modelValue'],
  template: `<div v-if="modelValue" class="el-drawer-stub"><slot/></div>`,
}

export const mountGlobals = {
  global: {
    plugins: [ElementPlus, testRouter, i18n],
    stubs: { ElDialog: ElDialogStub, ElDrawer: ElDrawerStub },
  },
}
