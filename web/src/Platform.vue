<script setup>
import { computed, onMounted, ref } from 'vue'
import { appliedGeneration, overlayStatus } from './path-status.js'
import { Activity, ArrowLeft, ArrowRight, Building2, Database, LayoutDashboard, LogOut, Moon, Network, RefreshCw, Search, ShieldCheck, Sun } from '@lucide/vue'

const token = ref(sessionStorage.getItem('platform_session') || '')
const theme = ref(localStorage.getItem('theme') || 'light')
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)
const view = ref('overview')
const filter = ref('')
const tenants = ref([])
const health = ref(null)
const detail = ref(null)
const checked = ref('')
const nav = [['overview', '平台总览', LayoutDashboard], ['tenants', '企业管理', Building2], ['health', '服务健康', Activity]]
const visible = computed(() => tenants.value.filter(t => [t.email, t.cidr].some(value => value.toLowerCase().includes(filter.value.trim().toLowerCase()))))
const totals = computed(() => tenants.value.reduce((sum, t) => ({ devices: sum.devices + t.devices, online: sum.online + t.online }), { devices: 0, online: 0 }))
const fmt = value => value ? new Date(value).toLocaleString() : '—'
const device = id => detail.value?.devices?.find(d => d.id === id)?.hostname || id

function toggleTheme() {
  theme.value = theme.value === 'light' ? 'dark' : 'light'
  localStorage.setItem('theme', theme.value)
}
function clearSession() {
  token.value = ''
  sessionStorage.removeItem('platform_session')
  tenants.value = []
  detail.value = null
  health.value = null
  checked.value = ''
  view.value = 'overview'
}
async function api(path, options = {}) {
  const response = await fetch(`/api/platform/${path}`, {
    ...options, headers: { 'Content-Type': 'application/json', ...(token.value ? { Authorization: `Bearer ${token.value}` } : {}) },
  })
  if (response.status === 401 && path !== 'login') clearSession()
  if (!response.ok) {
    const message = response.status === 429 ? '请求过于频繁，请稍后重试。'
      : path === 'login' && response.status === 503 ? '平台登录未启用。'
      : response.status === 401 ? '平台凭证无效或会话已过期。' : '暂时无法读取平台数据。'
    throw new Error(message)
  }
  return response.status === 204 ? null : response.json()
}
async function run(fn) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try { await fn() } catch (e) { error.value = e.message } finally { busy.value = false }
}
async function load() {
  const results = await Promise.allSettled([api('tenants'), api('health')])
  if (!token.value) throw new Error('平台会话已过期，请重新登录。')
  tenants.value = results[0].status === 'fulfilled' ? results[0].value : []
  health.value = results[1].status === 'fulfilled' ? results[1].value : null
  if (results[0].status === 'rejected') throw results[0].reason
  checked.value = new Date().toLocaleTimeString()
  if (detail.value) detail.value = await api(`tenants/${encodeURIComponent(detail.value.user.id)}`)
}
function login() {
  return run(async () => {
    const session = await api('login', { method: 'POST', body: JSON.stringify({ email: email.value, password: password.value }) })
    password.value = ''
    token.value = session.token
    sessionStorage.setItem('platform_session', token.value)
    await load()
  })
}
function logout() {
  return run(async () => { await api('logout', { method: 'POST' }); clearSession() })
}
function select(next) { view.value = next; detail.value = null; error.value = '' }
function open(id) {
  return run(async () => { detail.value = await api(`tenants/${encodeURIComponent(id)}`); view.value = 'tenants' })
}
onMounted(() => { if (token.value) run(load) })
</script>

<template>
  <main class="app-shell platform-shell" :data-theme="theme">
    <header class="console-topbar">
      <div class="console-brand"><ShieldCheck :size="23" aria-hidden="true" /> SD-WAN 平台运营台</div>
      <div class="console-actions">
        <button class="console-icon-button" type="button" aria-label="切换主题" title="切换主题" @click="toggleTheme"><Moon v-if="theme === 'light'" :size="19" /><Sun v-else :size="19" /></button>
        <button v-if="token" class="console-icon-button" type="button" :disabled="busy" aria-label="刷新数据" title="刷新数据" @click="run(load)"><RefreshCw :size="19" /></button>
        <button v-if="token" class="console-icon-button" type="button" :disabled="busy" aria-label="退出平台" title="退出平台" @click="logout"><LogOut :size="19" /></button>
      </div>
    </header>
    <section v-if="!token" class="platform-login">
      <h1>平台运营登录</h1>
      <form class="platform-form" @submit.prevent="login">
        <label>平台邮箱<input v-model="email" type="email" autocomplete="username" required /></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required /></label>
        <p v-if="error" class="console-error" role="alert">{{ error }}</p>
        <button class="action-button primary" :disabled="busy" type="submit">登录</button>
        <a href="/">返回企业管理台</a>
      </form>
    </section>
    <div v-else class="console-layout">
      <aside class="console-sidebar" aria-label="平台导航">
        <p class="sidebar-caption">只读运营</p>
        <button v-for="[id, label, icon] in nav" :key="id" class="console-nav-item" :class="{ active: view === id }" type="button" :disabled="busy" @click="select(id)"><component :is="icon" :size="19" aria-hidden="true" />{{ label }}</button>
      </aside>
      <section class="console-content platform-content" :aria-busy="busy">
        <header class="page-heading">
          <div><button v-if="detail" class="text-action" type="button" :disabled="busy" @click="select('tenants')"><ArrowLeft :size="16" /> 返回企业列表</button><h1>{{ detail ? detail.user.email : nav.find(([id]) => id === view)?.[1] }}</h1></div>
          <span v-if="checked" class="section-meta">最近刷新 {{ checked }}</span>
        </header>
        <p v-if="error" class="console-error" role="alert">{{ error }}</p>
        <template v-if="view === 'overview'">
          <div v-if="checked && !error" class="metric-grid">
            <article class="metric-card"><span><Building2 :size="18" /> 企业账户</span><strong>{{ tenants.length }}</strong></article>
            <article class="metric-card"><span><Network :size="18" /> 已登记设备</span><strong>{{ totals.devices }}</strong></article>
            <article class="metric-card"><span><Activity :size="18" /> 近 60 秒在线</span><strong>{{ totals.online }}</strong></article>
          </div>
          <button class="action-button" type="button" :disabled="busy" @click="select('tenants')">查看企业 <ArrowRight :size="16" /></button>
        </template>
        <section v-if="view === 'health'" class="content-section platform-table-wrap">
          <table class="platform-table"><thead><tr><th>组件</th><th>状态</th></tr></thead><tbody>
            <tr><td><Activity :size="17" /> Controller</td><td>{{ health ? (health.postgres ? '就绪' : '未就绪 · API 可响应') : '未确认' }}<small v-if="health">{{ fmt(health.checked_at) }}</small></td></tr>
            <tr><td><Database :size="17" /> PostgreSQL</td><td>{{ health ? (health.postgres ? '可连接' : '连接异常') : '未确认' }}</td></tr>
            <tr><td><Network :size="17" /> Bootstrap</td><td>未采集</td></tr>
          </tbody></table>
        </section>
        <section v-if="view === 'tenants' && !detail" class="content-section">
          <label class="platform-search"><Search :size="18" /><input v-model="filter" aria-label="搜索企业" placeholder="搜索账户或网段" /></label>
          <div class="platform-table-wrap"><table class="platform-table"><thead><tr><th>账户</th><th>Overlay</th><th>设备 / 配额</th><th>近期在线</th><th>操作</th></tr></thead><tbody>
            <tr v-for="t in visible" :key="t.id"><td>{{ t.email }}</td><td><code>{{ t.cidr }}</code></td><td>{{ t.devices }} / {{ t.max_devices }}</td><td>{{ t.online }}</td><td><button class="mini-action" type="button" :disabled="busy" @click="open(t.id)">查看诊断</button></td></tr>
            <tr v-if="!busy && !visible.length && !error"><td colspan="5">没有匹配的企业</td></tr>
          </tbody></table></div>
        </section>
        <template v-if="detail">
          <section class="content-section platform-table-wrap">
            <h2>站点与设备</h2><table class="platform-table"><thead><tr><th>设备</th><th>角色</th><th>虚拟 IP</th><th>最近上报</th></tr></thead><tbody>
              <tr v-for="d in detail.devices" :key="d.id"><td>{{ d.hostname }}</td><td>{{ d.site_role === 'main_site' ? '主 Hub' : '客户端' }}</td><td><code>{{ d.virtual_ip }}</code></td><td>{{ fmt(d.last_seen_at) }}</td></tr>
              <tr v-if="!detail.devices?.length"><td colspan="4">暂无设备</td></tr>
            </tbody></table>
          </section>
          <section class="content-section platform-table-wrap">
            <h2>连接路径</h2><table class="platform-table"><thead><tr><th>分支 / Hub</th><th>目标路径</th><th>双端已应用</th><th>Overlay 验证</th><th>上次验证时间</th></tr></thead><tbody>
              <tr v-for="p in detail.paths" :key="p.client_device_id"><td>{{ device(p.client_device_id) }} / {{ device(p.main_site_device_id) }}</td><td>{{ p.desired_path }}<small>目标代次 {{ p.generation }}</small></td><td>分支：{{ appliedGeneration(p.client_applied_generation, p.generation) }}<small>Hub：{{ appliedGeneration(p.hub_applied_generation, p.generation) }}</small></td><td>{{ overlayStatus(p) }}<small v-if="p.verification_error">{{ p.verification_error }}</small></td><td>{{ fmt(p.verified_at) }}</td></tr>
              <tr v-if="!detail.paths?.length"><td colspan="5">暂无链路</td></tr>
            </tbody></table>
          </section>
          <section class="content-section platform-table-wrap">
            <h2>子网路由</h2><table class="platform-table"><thead><tr><th>网段</th><th>审批</th><th>网关自检</th><th>分支到 LAN</th></tr></thead><tbody>
              <tr v-for="r in detail.routes" :key="r.id"><td><code>{{ r.cidr }}</code></td><td>{{ r.approved ? '已批准' : '未批准' }}</td><td>{{ !r.gateway_checked_at ? '未上报' : r.gateway_error || (r.gateway_enabled ? '已启用' : '未启用') }}<small v-if="r.gateway_checked_at">{{ fmt(r.gateway_checked_at) }}</small></td><td>未采集</td></tr>
              <tr v-if="!detail.routes?.length"><td colspan="4">暂无子网</td></tr>
            </tbody></table>
          </section>
          <section class="content-section platform-table-wrap">
            <h2>企业自建 Relay</h2><table class="platform-table"><thead><tr><th>节点</th><th>状态</th><th>最近握手</th></tr></thead><tbody>
              <tr v-for="r in detail.relays" :key="r.id"><td>{{ r.name || r.id }}</td><td>{{ r.status !== 'active' ? '未启用' : ({healthy: '握手已确认', degraded: '同步或接口异常', offline: '心跳超时', awaiting_data_plane: '等待握手'})[r.health_status] || '未确认' }}</td><td>{{ fmt(r.last_handshake_at) }}</td></tr>
              <tr v-if="!detail.relays?.length"><td colspan="3">未配置企业 Relay</td></tr>
            </tbody></table>
          </section>
        </template>
      </section>
    </div>
  </main>
</template>
