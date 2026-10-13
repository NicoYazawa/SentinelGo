/**
 * SentinelGo 前端应用入口。
 *
 * 初始化顺序：
 *  1. Pinia — 轻量级状态管理（替代 Vuex）
 *  2. Vue Router — SPA 路由
 *  3. Element Plus — UI 组件库
 *
 * 全局注册后挂载到 #app 根节点。
 */
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './style.css'

import App from './App.vue'
import router from './router'
import { i18n } from './locales'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)
app.use(ElementPlus)

app.mount('#app')
