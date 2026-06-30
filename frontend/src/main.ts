import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useUiStore } from './stores/ui'
import './style.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
// Apply the theme before mount so the first paint defaults to dark (the wallet's
// native habitat). ui.init() later reconciles any persisted preference.
useUiStore().applyTheme()
app.use(router).mount('#app')
