import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { useUiStore } from './stores/ui'
import './style.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
// Restore the persisted theme (default dark) before mount so the first paint —
// including the locked screens, which never mount AppShell — honors it.
useUiStore().initTheme()
app.use(router).mount('#app')
