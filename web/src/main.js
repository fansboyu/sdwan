import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import Platform from './Platform.vue'

createApp(/^\/platform\/?$/.test(location.pathname) ? Platform : App).mount('#app')
