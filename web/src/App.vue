<script setup>
import { onMounted, ref } from 'vue'
import { appliedGeneration, overlayStatus } from './path-status.js'
import { ArrowLeftRight, Cable, GitFork, LayoutDashboard, Monitor, Moon, Network, Radio, RefreshCw, Settings, Sun } from '@lucide/vue'

const apiBase = import.meta.env.VITE_API_BASE_URL || ''
const locale = ref(localStorage.getItem('locale') || 'zh')
const theme = ref(localStorage.getItem('theme') || 'dark')
const adminToken = ref(localStorage.getItem('admin_token') || '')
const user = ref(null)
const account = ref(null)
const authMode = ref('login')
const email = ref('')
const password = ref('')
const emailCode = ref('')
const emailCodeSending = ref(false)
const emailCodeCooldown = ref(0)
const devices = ref([])
const selectedDevice = ref(null)
const error = ref('')
const showPlanModal = ref(false)
const relayForm = ref({ name: '', public_key: '', endpoint: '', virtual_ip: '' })
const relayToken = ref('')
const relayInstallCommand = 'curl -fsSL https://controller.englishlisten.cn/downloads/v1.2.2/install-relay.sh | sudo sh'
const activeView = ref('overview')
const showAdminToken = ref(false)

const enterpriseNavigation = [
  { id: 'overview', label: '网络总览', icon: LayoutDashboard },
  { id: 'devices', label: '设备与站点', icon: Monitor },
  { id: 'routes', label: '子网路由', icon: GitFork },
  { id: 'paths', label: '连接路径', icon: ArrowLeftRight },
  { id: 'relays', label: 'Relay', icon: Radio },
  { id: 'settings', label: '企业设置', icon: Settings },
]

const pageMeta = {
  overview: ['网络总览', '企业网络运行状态与关键链路'],
  devices: ['设备与站点', '管理当前网络中的设备和主站点'],
  routes: ['子网路由', '检查网关状态、审批 LAN 路由并查看服务探测'],
  paths: ['连接路径', '查看直连与 Relay 候选、当前路径和切换状态'],
  relays: ['Relay', '管理企业自建 Relay 和账户路径模式'],
  settings: ['企业设置', '查看地址池、套餐和设备入网凭证'],
}

function navigate(view) {
  activeView.value = view
  error.value = ''
  selectedDevice.value = null
}

async function refreshDashboard() {
  error.value = ''
  await loadDashboard()
}

const messages = {
  zh: {
    title: 'SD-WAN 控制器',
    emailOnly: '仅支持邮箱注册和登录',
    login: '登录',
    register: '注册',
    logout: '退出',
    email: '邮箱',
    password: '密码',
    passwordHint: '至少 8 位',
    authFailed: '认证失败',
    accountTitle: '账号网络',
    accountDesc: '第一版取消区域概念。每个账号默认获得一个 100.64.0.0/10 下的独立 /24 地址池，设备直接加入账号网络。',
    enrollmentToken: '设备入网 Token',
    tokenNote: '当前版本 Admin Token 同时作为设备首次入网 Token。设备注册成功后会获得独立 Device Token，后续轮询和网络视图不依赖 Admin Token。',
    addressPool: '地址池',
    plan: '当前套餐',
    devices: '设备',
    netmap: '网络视图',
    capabilities: '能力',
    subnetFeature: '快启子网服务',
    relayFeature: '自行搭建 Relay',
    enabled: '已开通',
    disabled: '未开通',
    upgrade: '查看升级',
    planTitle: '服务等级',
    planDesc: '支付接口暂时不接入，当前只展示套餐能力。',
    monthly: '/月',
    close: '关闭',
    deviceList: '设备节点',
    noDevices: '暂无设备',
    deviceDetail: '设备详情',
    hostname: '主机名',
    virtualIP: '虚拟 IP',
    os: '系统',
    arch: '架构',
    status: '状态',
    clientVersion: '客户端版本',
    lastSeen: '最后在线',
    createdAt: '创建时间',
    endpoints: 'Endpoint',
    publicKey: 'WireGuard 公钥',
    role: '角色',
    mainSite: '主站点',
    client: '客户端',
    noMainSite: '尚未设置主站点',
    setMainSite: '设为主站点',
    setMainSiteFailed: '设置主站点失败',
    actions: '操作',
    deleteDevice: '删除',
    confirmDeleteDevice: '确认删除这个设备节点吗？删除后该设备的 Device Token 会失效，需要重新注册。',
    deleteFailed: '删除设备失败',
    switchLanguage: 'English',
  },
  en: {
    title: 'SD-WAN Controller',
    emailOnly: 'Email-only registration and login',
    login: 'Login',
    register: 'Register',
    logout: 'Logout',
    email: 'Email',
    password: 'Password',
    passwordHint: 'At least 8 characters',
    authFailed: 'Authentication failed',
    accountTitle: 'Account Network',
    accountDesc: 'The first release removes regions. Each account receives an isolated /24 pool under 100.64.0.0/10, and devices join the account network directly.',
    enrollmentToken: 'Device Enrollment Token',
    tokenNote: 'In this version, the Admin Token also works as the first-time device enrollment token. Registered devices receive an independent Device Token.',
    addressPool: 'Address Pool',
    plan: 'Plan',
    devices: 'Devices',
    netmap: 'Netmap',
    capabilities: 'Capabilities',
    subnetFeature: 'Quick Subnet Service',
    relayFeature: 'Self-hosted Relay',
    enabled: 'Enabled',
    disabled: 'Disabled',
    upgrade: 'View Plans',
    planTitle: 'Service Levels',
    planDesc: 'Payment is not connected yet. Plans are display-only for now.',
    monthly: '/month',
    close: 'Close',
    deviceList: 'Device Nodes',
    noDevices: 'No devices yet',
    deviceDetail: 'Device Detail',
    hostname: 'Hostname',
    virtualIP: 'Virtual IP',
    os: 'OS',
    arch: 'Arch',
    status: 'Status',
    clientVersion: 'Client Version',
    lastSeen: 'Last Seen',
    createdAt: 'Created At',
    endpoints: 'Endpoint',
    publicKey: 'WireGuard Public Key',
    role: 'Role',
    mainSite: 'Main Site',
    client: 'Client',
    noMainSite: 'No main site selected',
    setMainSite: 'Set Main Site',
    setMainSiteFailed: 'Failed to set main site',
    actions: 'Actions',
    deleteDevice: 'Delete',
    confirmDeleteDevice: 'Delete this device node? Its Device Token will stop working and the device must register again.',
    deleteFailed: 'Failed to delete device',
    switchLanguage: '中文',
  },
}

function t(key) {
  return messages[locale.value][key]
}

function toggleLocale() {
  locale.value = locale.value === 'zh' ? 'en' : 'zh'
  localStorage.setItem('locale', locale.value)
}

function setTheme(value) {
  theme.value = value
  localStorage.setItem('theme', value)
}

function authHeaders() {
  return { Authorization: `Bearer ${adminToken.value}` }
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function gatewayServiceProbes(route) {
  try {
    const probes = JSON.parse(route.gateway_service_status || '[]')
    return Array.isArray(probes) ? probes : []
  } catch {
    return []
  }
}

function routeHistory(route) {
  try {
    const history = JSON.parse(route.gateway_history || '[]')
    return Array.isArray(history) ? history.slice(-5).reverse() : []
  } catch {
    return []
  }
}

function gatewayAdvice(route) {
  if (!route.gateway_checked_at) return '等待主站点 Agent 上报网关检查结果。'
  if (route.gateway_error?.includes('different LAN interface')) return '出口网卡与到 LAN 目标的实际路由网卡不一致；检查 out_interface。'
  if (!route.gateway_enabled) return '检查 IPv4 forwarding、WireGuard 接口、出口网卡和转发/NAT 规则。'
  if (!route.gateway_lan_reachable) return '网关到 LAN 探测目标不可达；检查目标地址、LAN 网关和本机路由。'
  const failedProbe = gatewayServiceProbes(route).find((probe) => !probe.reachable)
  if (failedProbe) return `LAN 可达，但服务 ${failedProbe.target} 探测失败；检查服务监听端口与主机防火墙。`
  if (route.gateway_error) return route.gateway_error
  return '网关基础转发与已配置服务探测均正常。'
}

function relayHealthLabel(status) {
  return ({ healthy: '数据面正常', awaiting_data_plane: '等待真实握手', degraded: '同步/接口异常', offline: '心跳超时' })[status] || '尚未上报状态'
}

function peerPathQuality(path) {
  return (account.value?.path_quality || []).filter((quality) =>
    quality.peer_device_id === path.client_device_id || quality.peer_device_id === path.main_site_device_id,
  )
}

function pathVerificationLabel(path) {
  const labels = { verified: '已验证', verifying: '验证中', failed: '探测失败', path_mismatch: '两端路径不一致' }
  return labels[path.verification_state] || '等待 Agent 上报'
}

function formatPrice(cents) {
  if (!cents) return locale.value === 'zh' ? '免费' : 'Free'
  return `￥${(cents / 100).toFixed(1)}`
}

async function readPayload(response) {
  const text = await response.text()
  if (!text) return {}
  return JSON.parse(text)
}

async function loadMe() {
  if (!adminToken.value) return
  const response = await fetch(`${apiBase}/admin/auth/me`, {
    headers: authHeaders(),
  })
  if (!response.ok) {
    logout()
    return
  }
  const payload = await readPayload(response)
  user.value = payload.user || payload.admin_user
}

async function loadDashboard() {
  if (!adminToken.value) return
  const [accountResp, devicesResp] = await Promise.all([
    fetch(`${apiBase}/admin/account`, { headers: authHeaders() }),
    fetch(`${apiBase}/admin/devices`, { headers: authHeaders() }),
  ])
  if (accountResp.status === 401 || devicesResp.status === 401) {
    logout()
    return
  }
  account.value = await readPayload(accountResp)
  const devicePayload = await readPayload(devicesResp)
  devices.value = devicePayload.devices || []
}

async function submitAuth() {
  error.value = ''
  const path = authMode.value === 'login' ? '/admin/auth/login' : '/admin/auth/register'
  const body = { email: email.value, password: password.value }
  if (authMode.value === 'register') {
    body.email_code = emailCode.value
  }
  const response = await fetch(`${apiBase}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const payload = await readPayload(response)
  if (!response.ok) {
    error.value = payload.error || t('authFailed')
    return
  }
  adminToken.value = payload.token
  user.value = payload.user || payload.admin_user
  localStorage.setItem('admin_token', payload.token)
  password.value = ''
  emailCode.value = ''
  await loadDashboard()
}

async function sendEmailCode() {
  error.value = ''
  if (!email.value) {
    error.value = '请先输入邮箱'
    return
  }
  emailCodeSending.value = true
  const response = await fetch(`${apiBase}/admin/auth/email-code`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: email.value, purpose: 'register' }),
  })
  const payload = await readPayload(response)
  emailCodeSending.value = false
  if (!response.ok) {
    error.value = payload.error || '验证码发送失败，请稍后重试'
    return
  }
  emailCodeCooldown.value = payload.cooldown_seconds || 60
  const timer = window.setInterval(() => {
    emailCodeCooldown.value -= 1
    if (emailCodeCooldown.value <= 0) {
      window.clearInterval(timer)
      emailCodeCooldown.value = 0
    }
  }, 1000)
}

function logout() {
  adminToken.value = ''
  user.value = null
  account.value = null
  devices.value = []
  selectedDevice.value = null
  localStorage.removeItem('admin_token')
}

async function selectDevice(device) {
  const response = await fetch(`${apiBase}/admin/devices/${device.id}`, {
    headers: authHeaders(),
  })
  if (!response.ok) {
    error.value = t('authFailed')
    return
  }
  selectedDevice.value = await readPayload(response)
}

async function deleteDevice(device) {
  error.value = ''
  if (!window.confirm(t('confirmDeleteDevice'))) return
  const response = await fetch(`${apiBase}/admin/devices/${device.id}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || t('deleteFailed')
    return
  }
  if (selectedDevice.value?.device?.id === device.id) {
    selectedDevice.value = null
  }
  await loadDashboard()
}

function roleLabel(role) {
  return role === 'main_site' ? t('mainSite') : t('client')
}

function planRank(code) {
  if (code === 'relay') return 2
  if (code === 'subnet') return 1
  return 0
}

function canFreeUpgrade(plan) {
  if (!plan || plan.code === 'free') return false
  const active = account.value?.subscription
  const remaining = account.value?.free_upgrade?.months_remaining || 0
  if (!active) return remaining > 0
  if (active.source !== 'free_upgrade') return remaining > 0
  return planRank(plan.code) > planRank(active.plan_code)
}

function freeUpgradeButtonText(plan) {
  const active = account.value?.subscription
  if (active?.source === 'free_upgrade') {
    if (active.plan_code === plan.code) return '已开通'
    if (planRank(plan.code) > planRank(active.plan_code)) return '升级到该版本'
    return '已包含该能力'
  }
  const months = Math.min(12, account.value?.free_upgrade?.months_remaining || 0)
  return `免费升级 ${months} 个月`
}

async function setMainSite(device) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/devices/${device.id}/main-site`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || t('setMainSiteFailed')
    return
  }
  if (selectedDevice.value?.device?.id === device.id) {
    selectedDevice.value = await readPayload(response)
  }
  await loadDashboard()
}

async function freeUpgrade(plan) {
  error.value = ''
  if (!canFreeUpgrade(plan)) return
  const active = account.value?.subscription
  const months = active?.source === 'free_upgrade' ? 0 : Math.min(12, account.value?.free_upgrade?.months_remaining || 0)
  const response = await fetch(`${apiBase}/admin/subscription/free-upgrade`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ plan_code: plan.code, months }),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || 'Free upgrade failed'
    return
  }
  showPlanModal.value = false
  await loadMe()
  await loadDashboard()
}

async function cancelSubscription() {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/subscription/cancel`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || 'Cancel subscription failed'
    return
  }
  showPlanModal.value = false
  await loadMe()
  await loadDashboard()
}

function deviceLabel(deviceID) {
  const device = devices.value.find((item) => item.id === deviceID)
  if (!device) return deviceID
  return `${device.hostname} / ${device.virtual_ip}`
}

function subnetGatewayLabel(route) {
  if (!route.gateway_checked_at) return '未上报'
  if (route.gateway_enabled && !route.gateway_error && route.gateway_lan_reachable) return '网关正常'
  if (route.gateway_enabled) return '部分异常'
  return '未启用'
}

function subnetGatewayDetail(route) {
  const parts = []
  if (route.gateway_out_interface) parts.push(`出口 ${route.gateway_out_interface}`)
  if (route.gateway_route_interface && route.gateway_route_interface !== route.gateway_out_interface) {
    parts.push(`实际 ${route.gateway_route_interface}`)
  }
  if (route.gateway_lan_target) {
    parts.push(`${route.gateway_lan_target} ${route.gateway_lan_reachable ? '可达' : '不可达'}`)
  }
  if (route.gateway_error) parts.push(route.gateway_error)
  return parts.join(' · ') || '等待主站点上报 subnet-gateway 状态'
}

async function approveSubnetRoute(route, approved) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/subnet-routes/${route.id}/approval`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ approved }),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || '子网路由审批失败'
    return
  }
  await loadDashboard()
}

async function disableSubnetRoute(route) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/subnet-routes/${route.id}/disable`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || '子网路由停用失败'
    return
  }
  await loadDashboard()
}

async function createRelay() {
  error.value = ''
  relayToken.value = ''
  const response = await fetch(`${apiBase}/admin/relays`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify(relayForm.value),
  })
  if (response.status === 401) {
    logout()
    return
  }
  const payload = await readPayload(response)
  if (!response.ok) {
    error.value = payload.error || 'Relay 创建失败'
    return
  }
  relayToken.value = payload.relay_token
  relayForm.value = { name: '', public_key: '', endpoint: '', virtual_ip: '' }
  await loadDashboard()
}

async function copyRelayInstallCommand() {
  try {
    await navigator.clipboard.writeText(relayInstallCommand)
  } catch {
    error.value = '无法访问剪贴板，请手动复制部署命令。'
  }
}

async function enableRelay(relay) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/relays/${relay.id}/enable`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || 'Relay 启用失败'
    return
  }
  await loadDashboard()
}

async function disableRelay(relay) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/relays/${relay.id}/disable`, {
    method: 'POST',
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || 'Relay 停用失败'
    return
  }
  await loadDashboard()
}

async function setPathMode(mode) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/path-mode`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ mode }),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || '连接模式切换失败'
    return
  }
  await loadMe()
  await loadDashboard()
}

onMounted(async () => {
  await loadMe()
  await loadDashboard()
})
</script>

<template>
  <main :class="user ? 'app-shell' : 'login-shell'" :data-theme="theme">
    <section class="login-page" v-if="!user">
      <div class="login-brand"><span class="brand-mark">S</span><span>SD-WAN 控制台</span></div>
      <div class="login-visual" aria-hidden="true"><span class="wave wave-one"></span><span class="wave wave-two"></span><span class="wave wave-three"></span></div>
      <section class="login-hero">
        <p class="login-ribbon">欢迎登录 SD-WAN 控制台</p>
        <h1>不受网络限制<br />安全访问本地资源</h1>
        <p>多地设备互联协同，轻松管理私有网络、主站点和安全接入。</p>
        <div class="login-dashes"><span></span><span></span></div>
      </section>
      <section class="login-card">
        <div class="login-logo">S</div>
        <h2>{{ authMode === 'login' ? '账号密码登录' : '注册控制台账号' }}</h2>
        <div class="login-tabs">
          <button :class="{ active: authMode === 'login' }" type="button" @click="authMode = 'login'">登录</button>
          <button :class="{ active: authMode === 'register' }" type="button" @click="authMode = 'register'">注册</button>
        </div>
        <form class="auth-form login-form" @submit.prevent="submitAuth">
          <label class="login-field"><span>@</span><input v-model="email" type="email" autocomplete="email" placeholder="请输入邮箱" required /></label>
          <div class="code-row" v-if="authMode === 'register'">
            <label class="login-field code-field"><span>#</span><input v-model="emailCode" inputmode="numeric" autocomplete="one-time-code" placeholder="请输入邮箱验证码" required /></label>
            <button class="code-button" type="button" :disabled="emailCodeSending || emailCodeCooldown > 0" @click="sendEmailCode">{{ emailCodeCooldown > 0 ? `${emailCodeCooldown}s` : (emailCodeSending ? '发送中' : '发送验证码') }}</button>
          </div>
          <label class="login-field"><span>⌁</span><input v-model="password" type="password" :autocomplete="authMode === 'login' ? 'current-password' : 'new-password'" placeholder="请输入密码" required /></label>
          <div class="login-options">
            <label><input type="checkbox" /><span>30天内自动登录</span></label>
            <button class="link-button" type="button" @click="authMode = authMode === 'login' ? 'register' : 'login'">{{ authMode === 'login' ? '创建账号' : '返回登录' }}</button>
          </div>
          <button class="login-submit" type="submit">{{ authMode === 'login' ? '登录' : '注册' }}</button>
        </form>
        <p class="login-agreement">登录即同意《用户协议》和《隐私政策》</p>
        <p class="error login-error" v-if="error">{{ error }}</p>
      </section>
    </section>

    <template v-else>
      <header class="console-topbar">
        <div class="console-brand"><span class="console-mark">S</span><span>SD-WAN</span></div>
        <div class="console-context"><span>当前企业网络</span><strong>{{ user.email }}</strong></div>
        <div class="console-actions">
          <span class="controller-health">企业管理端</span>
          <div class="theme-switch" role="group" aria-label="界面主题">
            <button type="button" :class="{ selected: theme === 'dark' }" :aria-pressed="theme === 'dark'" aria-label="暗黑风格" title="暗黑风格" @click="setTheme('dark')"><Moon :size="16" aria-hidden="true" /></button>
            <button type="button" :class="{ selected: theme === 'light' }" :aria-pressed="theme === 'light'" aria-label="明亮风格" title="明亮风格" @click="setTheme('light')"><Sun :size="16" aria-hidden="true" /></button>
          </div>
          <button class="console-icon-button" type="button" aria-label="刷新数据" title="刷新数据" @click="refreshDashboard"><RefreshCw :size="18" aria-hidden="true" /></button>
          <button class="console-text-button" type="button" @click="toggleLocale">{{ t('switchLanguage') }}</button>
          <button class="user-menu" type="button" @click="logout"><span class="user-avatar">{{ user.email?.slice(0, 1).toUpperCase() }}</span><span>退出</span></button>
        </div>
      </header>

      <div class="console-layout">
        <aside class="console-sidebar" aria-label="企业管理导航">
          <p class="sidebar-caption">企业工作台</p>
          <button v-for="item in enterpriseNavigation" :key="item.id" class="console-nav-item" :class="{ active: activeView === item.id }" type="button" @click="navigate(item.id)">
            <component :is="item.icon" class="nav-symbol" :size="19" aria-hidden="true" />{{ item.label }}
          </button>
          <div class="sidebar-network">
            <span class="network-dot"></span><span>企业网络</span>
            <code>{{ account?.user?.overlay_cidr || '加载中' }}</code>
          </div>
        </aside>

        <section class="console-content">
          <header class="page-heading">
            <div><p class="page-eyebrow">{{ user.email }}</p><h1>{{ pageMeta[activeView][0] }}</h1><p class="page-description">{{ pageMeta[activeView][1] }}</p></div>
            <div class="page-heading-actions">
              <span v-if="account" class="sync-label">Netmap v{{ account.user.netmap_version }}</span>
              <button v-if="activeView === 'settings'" class="action-button primary" type="button" @click="showPlanModal = true">查看套餐</button>
            </div>
          </header>

          <p class="console-error" role="alert" v-if="error">{{ error }}</p>


          <template v-if="account && activeView === 'overview'">
            <div class="metric-grid">
              <article class="metric-card"><span><Monitor :size="18" aria-hidden="true" /> 设备节点</span><strong>{{ account.device_count }} <small>/ {{ account.user.max_devices }}</small></strong><em>当前企业网络</em></article>
              <article class="metric-card"><span><Network :size="18" aria-hidden="true" /> Overlay 地址池</span><strong class="metric-code">{{ account.user.overlay_cidr }}</strong><em>企业专属网段</em></article>
              <article class="metric-card"><span><Cable :size="18" aria-hidden="true" /> 已批准子网</span><strong>{{ (account.subnet_routes || []).filter((route) => route.approved && route.status === 'active').length }}</strong><em>{{ (account.subnet_routes || []).length }} 条已发布</em></article>
              <article class="metric-card"><span><Radio :size="18" aria-hidden="true" /> Relay 状态</span><strong class="metric-state" :class="account.relay_healthy ? 'good' : 'muted'">{{ account.relay_healthy ? '运行正常' : '未就绪' }}</strong><em>{{ (account.relays || []).length }} 个已配置</em></article>
            </div>

            <section class="content-section topology-section">
              <div class="section-heading"><div><h2>企业网络拓扑</h2><p>当前版本支持一个主站点 Hub 与客户端；多分支站点模型尚未接入。</p></div><button class="text-action" type="button" @click="navigate('devices')">查看设备 <span>→</span></button></div>
              <div class="topology-row">
                <div class="topology-node main-node"><span class="node-icon">H</span><div><strong>主站点 Hub</strong><small>{{ account.main_site ? `${account.main_site.hostname} · ${account.main_site.virtual_ip}` : '尚未指定主站点' }}</small></div><span class="node-status" :class="account.main_site ? 'online' : 'pending'">{{ account.main_site ? '已设置' : '待设置' }}</span></div>
                <div class="topology-link"><span>Overlay</span></div>
                <div class="topology-node"><span class="node-icon clients">D</span><div><strong>客户端设备</strong><small>{{ Math.max(0, devices.length - (account.main_site ? 1 : 0)) }} 个设备节点</small></div><button class="text-action" type="button" @click="navigate('devices')">管理 →</button></div>
                <div class="topology-link relay-link"><span>{{ account.user.path_mode === 'relay' ? '业务路径' : '备用路径' }}</span></div>
                <div class="topology-node relay-node"><span class="node-icon relay">R</span><div><strong>自建 Relay</strong><small>{{ account.relay_healthy ? '健康 · 可供路径切换' : '未就绪或心跳超时' }}</small></div><span class="node-status" :class="account.relay_healthy ? 'online' : 'pending'">{{ account.relay_healthy ? '可用' : '待检查' }}</span></div>
              </div>
            </section>

            <section class="content-section">
              <div class="section-heading"><div><h2>连接路径</h2><p>按客户端显示 Controller 目标路径与 Agent 上报的运行状态。</p></div><button class="text-action" type="button" @click="navigate('paths')">查看诊断 <span>→</span></button></div>
              <div class="data-table path-summary-table">
                <div class="data-row table-head"><span>客户端</span><span>当前路径</span><span>目标路径</span><span>状态</span><span>更新时间</span></div>
                <div class="data-row" v-for="path in (account.peer_paths || []).slice(0, 4)" :key="path.client_device_id">
                  <span class="primary-cell">{{ deviceLabel(path.client_device_id) }}</span><span>{{ path.current_path }}</span><span>{{ path.desired_path }}</span><span><i class="status-dot" :class="path.state === 'stable' ? 'green' : 'amber'"></i>{{ path.state }}</span><span>{{ formatTime(path.updated_at) }}</span>
                </div>
                <div class="table-empty" v-if="!(account.peer_paths || []).length">暂无路径状态；设备完成首次同步后会显示在这里。</div>
              </div>
            </section>

            <section class="content-section">
              <div class="section-heading"><div><h2>子网路由概况</h2><p>审批状态与主站点 LAN 网关探测。</p></div><button class="text-action" type="button" @click="navigate('routes')">管理路由 <span>→</span></button></div>
              <div class="route-overview-list" v-if="(account.subnet_routes || []).length">
                <div class="route-overview-row" v-for="route in account.subnet_routes.slice(0, 3)" :key="route.id"><code>{{ route.cidr }}</code><span>{{ deviceLabel(route.device_id) }}</span><span class="route-status" :class="route.approved ? 'approved' : 'waiting'">{{ route.approved ? '已批准' : '待审批' }}</span><span>{{ subnetGatewayLabel(route) }}</span></div>
              </div>
              <div class="table-empty" v-else>暂未发布子网路由。</div>
            </section>
          </template>

          <template v-if="account && activeView === 'devices'">
            <section class="content-section">
              <div class="section-heading"><div><h2>设备节点</h2><p>主站点负责对外发布 LAN 路由，客户端接入企业 Overlay。</p></div><span class="section-meta">{{ devices.length }} / {{ account.user.max_devices }} 个设备</span></div>
              <div class="data-table">
                <div class="data-row device-table-head"><span>设备</span><span>虚拟 IP</span><span>角色</span><span>客户端版本</span><span>最近在线</span><span>操作</span></div>
                <div class="data-row device-table-row" v-for="device in devices" :key="device.id" @click="selectDevice(device)">
                  <span class="device-name"><span class="device-glyph">{{ device.os?.slice(0, 1).toUpperCase() || 'D' }}</span><span><strong>{{ device.hostname }}</strong><small>{{ device.os }} · {{ device.arch || '架构未知' }}</small></span></span>
                  <code>{{ device.virtual_ip }}</code><span><span class="role-label" :class="{ hub: device.site_role === 'main_site' }">{{ roleLabel(device.site_role) }}</span></span><span>{{ device.client_version || 'unknown' }}</span><span>{{ formatTime(device.last_seen_at) }}</span>
                  <span class="inline-actions"><button class="mini-action" type="button" :disabled="device.site_role === 'main_site'" @click.stop="setMainSite(device)">{{ device.site_role === 'main_site' ? '主站点' : '设为主站点' }}</button><button class="mini-action danger-text" type="button" @click.stop="deleteDevice(device)">删除</button></span>
                </div>
                <div class="table-empty" v-if="!devices.length">{{ t('noDevices') }}</div>
              </div>
            </section>
            <section class="content-section detail-section" v-if="selectedDevice">
              <div class="section-heading"><div><h2>{{ selectedDevice.device.hostname }}</h2><p>设备详情与 Endpoint</p></div><button class="text-action" type="button" @click="selectedDevice = null">关闭</button></div>
              <dl class="detail-grid"><dt>虚拟 IP</dt><dd>{{ selectedDevice.device.virtual_ip }}</dd><dt>状态</dt><dd>{{ selectedDevice.device.status }}</dd><dt>角色</dt><dd>{{ roleLabel(selectedDevice.device.site_role) }}</dd><dt>公钥</dt><dd><code>{{ selectedDevice.device.public_key }}</code></dd></dl>
              <div class="endpoint-list"><div v-for="endpoint in selectedDevice.endpoints || []" :key="endpoint.id" class="endpoint-row"><span>{{ endpoint.endpoint_type }}</span><code>{{ endpoint.address }}</code><small>{{ endpoint.source }} · {{ formatTime(endpoint.updated_at) }}</small></div><span v-if="!selectedDevice.endpoints?.length" class="muted-copy">暂无 Endpoint</span></div>
            </section>
          </template>

          <template v-if="account && activeView === 'routes'">
            <section class="content-section route-conflicts" v-if="(account.route_conflicts || []).length">
              <div class="section-heading"><div><h2>子网冲突记录</h2><p>同一企业内重叠的 LAN CIDR 会阻止发布；不同企业间的相同私网段不构成冲突。</p></div><span class="section-meta">{{ account.route_conflicts.filter((item) => !item.resolved_at).length }} 项待处理</span></div>
              <div class="route-conflict-row" v-for="item in account.route_conflicts" :key="item.id"><span class="route-status" :class="item.resolved_at ? 'approved' : 'waiting'">{{ item.resolved_at ? '已解决' : '待处理' }}</span><code>{{ item.cidr }}</code><span>{{ item.details }}</span><small>{{ deviceLabel(item.device_id) }} · {{ formatTime(item.created_at) }}</small></div>
            </section>
            <section class="content-section">
              <div class="section-heading"><div><h2>已发布的 LAN 路由</h2><p>批准路由才会加入客户端 Netmap；网关状态与服务探测用于辅助诊断。</p></div><span class="section-meta">{{ account.capabilities.enable_subnet ? '套餐已支持' : '当前套餐不支持' }}</span></div>
              <div class="data-table">
                <div class="data-row route-table-head"><span>网段</span><span>发布设备</span><span>审批</span><span>网关诊断</span><span>TCP 服务</span><span>操作</span></div>
                <div class="data-row route-table-row" v-for="route in account.subnet_routes || []" :key="route.id">
                  <code>{{ route.cidr }}</code><span>{{ deviceLabel(route.device_id) }}</span><span class="route-status" :class="route.approved ? 'approved' : 'waiting'">{{ route.approved ? '已批准' : '待审批' }}</span>
                  <span class="gateway-cell"><strong :class="subnetGatewayLabel(route) === '网关正常' ? 'text-good' : 'text-warn'">{{ subnetGatewayLabel(route) }}</strong><small>{{ subnetGatewayDetail(route) }}</small></span>
                  <span class="probe-cell"><small v-for="probe in gatewayServiceProbes(route)" :key="probe.target" :class="probe.reachable ? 'text-good' : 'text-warn'">{{ probe.target }} · {{ probe.reachable ? `${probe.latency_ms} ms` : (probe.error || '不可达') }}</small><span v-if="!gatewayServiceProbes(route).length" class="muted-copy">未配置</span></span>
                  <span class="inline-actions"><button class="mini-action" type="button" :disabled="route.approved && route.status === 'active'" @click="approveSubnetRoute(route, true)">批准</button><button class="mini-action" type="button" :disabled="!route.approved" @click="approveSubnetRoute(route, false)">撤销批准</button><button class="mini-action danger-text" type="button" :disabled="!route.advertised" @click="disableSubnetRoute(route)">停用</button></span>
                  <span class="gateway-advice">建议：{{ gatewayAdvice(route) }}</span>
                  <details class="route-history" v-if="routeHistory(route).length"><summary>运行状态历史（{{ routeHistory(route).length }}）</summary><div v-for="event in routeHistory(route)" :key="`${event.at}-${event.event}`"><strong>{{ event.event === 'route_created' ? '路由发布' : event.event === 'route_state_changed' ? '审批/发布状态变化' : '网关健康变化' }}</strong><span>{{ event.status }} · {{ event.gateway_enabled ? '网关已启用' : '网关未启用' }} · {{ event.lan_target || '未配置探测目标' }} {{ event.lan_reachable ? '可达' : '不可达' }}</span><small>{{ formatTime(event.at) }}<template v-if="event.gateway_error"> · {{ event.gateway_error }}</template></small></div></details>
                </div>
                <div class="table-empty" v-if="!(account.subnet_routes || []).length">暂无子网路由。请在主站点 Agent 配置 advertise_routes 后启动 Agent。</div>
              </div>
            </section>
          </template>

          <template v-if="account && activeView === 'paths'">
            <section class="content-section">
              <div class="section-heading"><div><h2>路径策略</h2><p>自动模式在直连连续异常后切换到 Relay，直连恢复后再切回。</p></div><span class="health-pill" :class="account.relay_healthy ? 'healthy' : 'unavailable'">Relay {{ relayHealthLabel(account.active_relay?.health_status) }}</span></div>
              <p class="relay-health-hint" v-if="account.active_relay && !account.relay_healthy">{{ account.active_relay.health_status === 'awaiting_data_plane' ? 'Relay 已完成控制面同步；Agent 正在验证真实 WireGuard 握手，验证通过后才允许切换策略。' : `暂不允许切换路径：${account.active_relay.last_error || '请检查 Relay 接口、同步状态与最近心跳。'}` }}</p>
              <div class="segmented-control" role="group" aria-label="连接路径模式"><button :class="{ selected: account.user.path_mode === 'direct' }" type="button" @click="setPathMode('direct')">直连优先</button><button :class="{ selected: account.user.path_mode === 'auto' }" type="button" :disabled="!account.relay_healthy" @click="setPathMode('auto')">自动 fallback</button><button :class="{ selected: account.user.path_mode === 'relay' }" type="button" :disabled="!account.relay_healthy" @click="setPathMode('relay')">强制 Relay</button></div>
              <div class="data-table">
                <div class="data-row path-table-head"><span>客户端 / 主站点</span><span>控制器策略</span><span>实际路径 / 验证</span><span>路径质量</span><span>最后握手</span><span>流量</span></div>
                <template v-for="path in account.peer_paths || []" :key="path.client_device_id">
                  <div class="data-row path-table-row"><span><strong>{{ deviceLabel(path.client_device_id) }}</strong><small>{{ deviceLabel(path.main_site_device_id) }}</small></span><span>{{ path.current_path }} → {{ path.desired_path }}<small>目标代次 {{ path.generation }}</small><small>分支：{{ appliedGeneration(path.client_applied_generation, path.generation) }}</small><small>Hub：{{ appliedGeneration(path.hub_applied_generation, path.generation) }}</small></span><span><strong>{{ path.verified_path || '未确认' }}</strong><small :class="overlayStatus(path) === 'Overlay 已验证' ? 'text-good' : path.verification_state === 'failed' || path.verification_state === 'path_mismatch' ? 'text-warn' : ''">{{ overlayStatus(path) }}</small><small v-if="path.verified_at">最近验证 {{ formatTime(path.verified_at) }}</small><small>分支到 LAN：未采集</small><small v-if="path.verification_error">{{ path.verification_error }}</small></span><span>{{ peerPathQuality(path).map((quality) => `${quality.path_type}${quality.current_path ? ' 当前' : ' 候选'}`).join(' / ') || '等待 Agent 上报' }}</span><span>{{ formatTime(peerPathQuality(path)[0]?.latest_handshake_at) }}</span><span>{{ peerPathQuality(path)[0]?.rx_bytes || 0 }} B 收 / {{ peerPathQuality(path)[0]?.tx_bytes || 0 }} B 发</span></div>
                </template>
                <div class="table-empty" v-if="!(account.peer_paths || []).length">当前没有需要展示的路径状态。</div>
              </div>
            </section>
          </template>

          <template v-if="account && activeView === 'relays'">
            <section class="content-section">
              <div class="section-heading"><div><h2>企业自建 Relay</h2><p>当前每个账户仅支持一个活动 Relay；登记其他节点暂不能组成主备池。</p></div><span class="health-pill" :class="account.relay_healthy ? 'healthy' : 'unavailable'">{{ account.relay_healthy ? '服务可用' : '未检测到健康 Relay' }}</span></div>
              <div class="relay-deploy-note"><strong>部署准备</strong><span>Relay 主机需先配置并启动 <code>sdwan-relay</code> WireGuard 接口，开放公网 UDP 端口，并启用 IPv4 转发。创建时填写该主机生成的 WireGuard 公钥和公网 Endpoint。</span><code>sudo sh -c 'umask 077; wg genkey &gt; /etc/wireguard/sdwan-relay.key; wg pubkey &lt; /etc/wireguard/sdwan-relay.key &gt; /etc/wireguard/sdwan-relay.pub'</code><small>随后读取公钥：<code>sudo cat /etc/wireguard/sdwan-relay.pub</code></small></div>
              <form class="relay-create-form" @submit.prevent="createRelay">
                <label><span>Relay 名称</span><input v-model="relayForm.name" placeholder="例如：华东中继" /></label>
                <label><span>WireGuard 公钥</span><input v-model="relayForm.public_key" placeholder="Relay public key" required /></label>
                <label><span>公网 Endpoint</span><input v-model="relayForm.endpoint" placeholder="relay.example.com:51873" required /></label>
                <label><span>虚拟 IP（可选）</span><input v-model="relayForm.virtual_ip" placeholder="默认 100.254.253.1" /></label>
                <button class="action-button primary" type="submit" :disabled="!account.capabilities.enable_self_relay">创建 Relay</button>
              </form>
              <div class="one-time-token" v-if="relayToken"><strong>Relay Token（仅显示一次）</strong><code>{{ relayToken }}</code><span>在已启动 WireGuard 接口的 Relay 主机运行安装命令，再按提示粘贴 Token。</span><div class="relay-install-command"><code>{{ relayInstallCommand }}</code><button class="mini-action" type="button" @click="copyRelayInstallCommand">复制命令</button></div></div>
              <div class="data-table">
                <div class="data-row relay-table-head"><span>Relay</span><span>虚拟 IP</span><span>Endpoint</span><span>运行状态</span><span>同步 / 版本</span><span>操作</span></div>
                <div class="data-row relay-table-row" v-for="relay in account.relays || []" :key="relay.id"><span><strong>{{ relay.name || '未命名 Relay' }}</strong><small>{{ relay.id }}</small></span><code>{{ relay.virtual_ip }}</code><code>{{ relay.endpoint }}</code><span class="route-status" :class="relay.health_status === 'healthy' ? 'approved' : 'waiting'">{{ relay.status }} · {{ relayHealthLabel(relay.health_status) }}</span><span>{{ relay.synced_peer_count }} peers · {{ relay.handshake_peer_count }} 有握手<small>最近握手 {{ formatTime(relay.last_handshake_at) }} · {{ relay.version || '版本未知' }}</small></span><span class="inline-actions"><button class="mini-action" type="button" :disabled="relay.status === 'active'" @click="enableRelay(relay)">启用</button><button class="mini-action danger-text" type="button" :disabled="relay.status !== 'active'" @click="disableRelay(relay)">停用</button></span></div>
                <div class="table-empty" v-if="!(account.relays || []).length">暂无 Relay。当前套餐可能需要开通自建 Relay 能力。</div>
              </div>
            </section>
          </template>

          <template v-if="account && activeView === 'settings'">
            <section class="content-section settings-grid">
              <div class="settings-block"><div class="section-heading"><div><h2>网络信息</h2><p>当前账号网络的基础配置。</p></div></div><dl class="detail-grid"><dt>账号</dt><dd>{{ user.email }}</dd><dt>Overlay 地址池</dt><dd><code>{{ account.user.overlay_cidr }}</code></dd><dt>设备上限</dt><dd>{{ account.user.max_devices }}</dd><dt>Netmap 版本</dt><dd>v{{ account.user.netmap_version }}</dd><dt>当前套餐</dt><dd>{{ account.user.plan_code }}</dd><dt>主站点</dt><dd>{{ account.main_site ? `${account.main_site.hostname} / ${account.main_site.virtual_ip}` : '尚未设置' }}</dd></dl></div>
              <div class="settings-block"><div class="section-heading"><div><h2>套餐能力</h2><p>当前版本能力状态与升级入口。</p></div><button class="text-action" type="button" @click="showPlanModal = true">管理套餐 →</button></div><div class="setting-line"><span>子网路由</span><strong>{{ account.capabilities.enable_subnet ? '已开通' : '未开通' }}</strong></div><div class="setting-line"><span>自建 Relay</span><strong>{{ account.capabilities.enable_self_relay ? '已开通' : '未开通' }}</strong></div><div class="setting-line"><span>免费升级额度</span><strong>{{ account.free_upgrade?.months_remaining || 0 }} 个月</strong></div></div>
              <div class="settings-block sensitive-block"><div class="section-heading"><div><h2>设备入网凭证</h2><p>当前版本 Admin Token 仍可用于首次设备注册，请勿公开分享。</p></div><button class="text-action" type="button" @click="showAdminToken = !showAdminToken">{{ showAdminToken ? '隐藏' : '显示' }}</button></div><code class="token-value">{{ showAdminToken ? adminToken : '••••••••••••••••••••••••' }}</code><p class="muted-copy">设备注册后会获得独立 Device Token。平台仍需进一步拆分短时入网凭证和设备审批。</p></div>
            </section>
          </template>
        </section>
      </div>

      <div class="modal-backdrop" v-if="showPlanModal" @click.self="showPlanModal = false">
        <section class="modal">
          <div class="section-title"><h2>{{ t('planTitle') }}</h2><button class="ghost" type="button" @click="showPlanModal = false">{{ t('close') }}</button></div>
          <p>{{ t('planDesc') }}</p>
          <div class="main-site" v-if="account?.subscription"><span>当前订阅</span><strong>{{ account.subscription.plan_code }} · 到期 {{ formatTime(account.subscription.expires_at) }}</strong><button class="danger small" type="button" @click="cancelSubscription">取消订阅</button></div>
          <div class="main-site"><span>免费升级额度</span><strong>{{ account?.free_upgrade?.months_remaining || 0 }} 个月剩余</strong></div>
          <div class="plans"><article class="plan" v-for="plan in account?.plans || []" :key="plan.code"><strong>{{ plan.name }}</strong><div class="price">{{ formatPrice(plan.price_cents) }} <span v-if="plan.price_cents">{{ t('monthly') }}</span></div><p>设备上限：{{ plan.max_devices }}</p><p>{{ t('subnetFeature') }}：{{ plan.enable_subnet ? t('enabled') : t('disabled') }}</p><p>{{ t('relayFeature') }}：{{ plan.enable_self_relay ? t('enabled') : t('disabled') }}</p><button v-if="plan.code !== 'free'" type="button" :disabled="!canFreeUpgrade(plan)" @click="freeUpgrade(plan)">{{ freeUpgradeButtonText(plan) }}</button></article></div>
        </section>
      </div>
    </template>
  </main>
</template>
