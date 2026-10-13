<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { SUPPORTED_LANGS, setLocale, currentLang, type Lang } from '../locales'

const { t } = useI18n()

function onChange(e: Event) {
  setLocale((e.target as HTMLSelectElement).value as Lang)
}
</script>

<template>
  <label class="lang-switcher" :title="t('nav.language')">
    <span class="lang-icon" aria-hidden="true">🌐</span>
    <select :value="currentLang" :aria-label="t('nav.language')" @change="onChange">
      <option v-for="l in SUPPORTED_LANGS" :key="l" :value="l">{{ l }}</option>
    </select>
  </label>
</template>

<style scoped>
.lang-switcher {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid #d8dee9;
  border-radius: 8px;
  background: #fff;
  font-size: 13px;
  color: #41536b;
  cursor: pointer;
}
.lang-switcher select {
  border: 0;
  background: transparent;
  font: inherit;
  color: inherit;
  cursor: pointer;
  outline: none;
}
.lang-icon { font-size: 14px; line-height: 1; }
.lang-switcher:focus-within { outline: 2px solid #409eff; outline-offset: 2px; }
</style>
