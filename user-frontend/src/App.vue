<template>
  <el-config-provider :locale="elLocaleRef">
    <router-view />
    <LanguageSwitcher class="global-lang-switcher" />
  </el-config-provider>
</template>

<script setup lang="ts">
import { elLocaleRef } from './locales'
import LanguageSwitcher from './components/LanguageSwitcher.vue'
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

.global-lang-switcher {
  position: fixed;
  top: 14px;
  right: 16px;
  z-index: 2000;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}
</style>
