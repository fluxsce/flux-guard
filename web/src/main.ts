import { createPinia } from 'pinia'
import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { installAiBridge } from './ai'
import './ai/modules'
import App from './App.vue'
import { i18n } from './locales'
import { installGuard, routes } from './router'
import 'niuma-ui/styles.css'
import 'niuma-ui/brand-icons.css'
import './styles/index.css'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
})

installGuard(router)

const app = createApp(App)
app.use(createPinia())
installAiBridge()
app.use(i18n).use(router).mount('#app')
