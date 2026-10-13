<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { SUPPORTED_LANGS, setLocale, currentLang, type Lang } from '../locales'
import { getToken } from '../utils/auth'
import { updateProfile } from '../api/user'

const { t } = useI18n()

function onChange(e: Event) {
  const lang = (e.target as HTMLSelectElement).value as Lang
  setLocale(lang)
  // 已登录时把语言偏好落库，下次登录自动跟随
  if (getToken()) {
    updateProfile({ locale: lang }).catch(() => {})
  }
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
  border: 1px solid #e8edf3;
  border-radius: 8px;
  background: #fff;
  font-size: 13px;
  color: #41536b;
  cursor: pointer;
}
.lang-switcher:hover { background: #f4f7fb; }
.lang-switcher select {
  border: 0;
  background: transparent;
  font: inherit;
  color: inherit;
  cursor: pointer;
  outline: none;
}
.lang-icon { font-size: 14px; line-height: 1; }
.lang-switcher:focus-within { outline: 2px solid var(--sg-primary); outline-offset: 2px; }
</style>
