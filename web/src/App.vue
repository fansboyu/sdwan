<script setup>
import { onMounted, ref } from 'vue'

const apiBase = import.meta.env.VITE_API_BASE_URL || ''
const locale = ref(localStorage.getItem('locale') || 'zh')
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

function authHeaders() {
  return { Authorization: `Bearer ${adminToken.value}` }
}

function formatTime(value) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
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

async function setRelayMode(enabled) {
  error.value = ''
  const response = await fetch(`${apiBase}/admin/relay-mode`, {
    method: 'POST',
    headers: { ...authHeaders(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  })
  if (response.status === 401) {
    logout()
    return
  }
  if (!response.ok) {
    const payload = await readPayload(response)
    error.value = payload.error || 'Relay 模式切换失败'
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
  <main :class="user ? 'shell' : 'login-shell'">
    <section class="topbar" v-if="user">
      <div>
        <p class="eyebrow">controller.englishlisten.cn</p>
        <h1>{{ t('title') }}</h1>
      </div>
      <div class="actions">
        <button class="ghost" type="button" @click="toggleLocale">{{ t('switchLanguage') }}</button>
        <button class="ghost" v-if="user" type="button" @click="logout">{{ t('logout') }}</button>
      </div>
    </section>

    <section class="login-page" v-if="!user">
      <div class="login-brand">
        <span class="brand-mark">S</span>
        <span>SD-WAN 控制台</span>
      </div>

      <div class="login-visual" aria-hidden="true">
        <span class="wave wave-one"></span>
        <span class="wave wave-two"></span>
        <span class="wave wave-three"></span>
      </div>

      <section class="login-hero">
        <p class="login-ribbon">欢迎登录 SD-WAN 控制台</p>
        <h1>不受网络限制<br />安全访问本地资源</h1>
        <p>多地设备互联协同，轻松管理私有网络、主站点和安全接入。</p>
        <div class="login-dashes">
          <span></span>
          <span></span>
        </div>
      </section>

      <section class="login-card">
        <div class="login-logo">S</div>
        <h2>{{ authMode === 'login' ? '账号密码登录' : '注册控制台账号' }}</h2>
        <div class="login-tabs">
          <button :class="{ active: authMode === 'login' }" type="button" @click="authMode = 'login'">
            登录
          </button>
          <button :class="{ active: authMode === 'register' }" type="button" @click="authMode = 'register'">
            注册
          </button>
        </div>
        <form class="auth-form login-form" @submit.prevent="submitAuth">
          <label class="login-field">
            <span>@</span>
            <input v-model="email" type="email" autocomplete="email" placeholder="请输入邮箱" required />
          </label>
          <div class="code-row" v-if="authMode === 'register'">
            <label class="login-field code-field">
              <span>#</span>
              <input v-model="emailCode" inputmode="numeric" autocomplete="one-time-code" placeholder="请输入邮箱验证码" required />
            </label>
            <button
              class="code-button"
              type="button"
              :disabled="emailCodeSending || emailCodeCooldown > 0"
              @click="sendEmailCode"
            >
              {{ emailCodeCooldown > 0 ? `${emailCodeCooldown}s` : (emailCodeSending ? '发送中' : '发送验证码') }}
            </button>
          </div>
          <label class="login-field">
            <span>⌁</span>
            <input
              v-model="password"
              type="password"
              :autocomplete="authMode === 'login' ? 'current-password' : 'new-password'"
              placeholder="请输入密码"
              required
            />
          </label>
          <div class="login-options">
            <label>
              <input type="checkbox" />
              <span>30天内自动登录</span>
            </label>
            <button class="link-button" type="button" @click="authMode = authMode === 'login' ? 'register' : 'login'">
              {{ authMode === 'login' ? '创建账号' : '返回登录' }}
            </button>
          </div>
          <button class="login-submit" type="submit">{{ authMode === 'login' ? '登录' : '注册' }}</button>
        </form>
        <p class="login-agreement">登录即同意《用户协议》和《隐私政策》</p>
        <p class="error login-error" v-if="error">{{ error }}</p>
      </section>
    </section>

    <template v-else>
      <section class="identity">
        <span>{{ user.email }}</span>
      </section>

      <section class="panel account-panel" v-if="account">
        <div class="section-title">
          <div>
            <h2>{{ t('accountTitle') }}</h2>
            <p>{{ t('accountDesc') }}</p>
          </div>
          <button type="button" @click="showPlanModal = true">{{ t('upgrade') }}</button>
        </div>
        <div class="metrics">
          <div>
            <span>{{ t('addressPool') }}</span>
            <strong>{{ account.user.overlay_cidr }}</strong>
          </div>
          <div>
            <span>{{ t('devices') }}</span>
            <strong>{{ account.device_count }}/{{ account.user.max_devices }}</strong>
          </div>
          <div>
            <span>{{ t('netmap') }}</span>
            <strong>v{{ account.user.netmap_version }}</strong>
          </div>
          <div>
            <span>{{ t('plan') }}</span>
            <strong>{{ account.user.plan_code }}</strong>
          </div>
        </div>
        <div class="main-site">
          <span>{{ t('mainSite') }}</span>
          <strong v-if="account.main_site">{{ account.main_site.hostname }} / {{ account.main_site.virtual_ip }}</strong>
          <strong v-else>{{ t('noMainSite') }}</strong>
        </div>
        <div class="capabilities">
          <span>{{ t('capabilities') }}</span>
          <strong>{{ t('subnetFeature') }}: {{ account.capabilities.enable_subnet ? t('enabled') : t('disabled') }}</strong>
          <strong>{{ t('relayFeature') }}: {{ account.capabilities.enable_self_relay ? t('enabled') : t('disabled') }}</strong>
        </div>
        <div class="main-site" v-if="account.subscription">
          <span>Subscription</span>
          <strong>{{ account.subscription.plan_code }} / {{ account.subscription.source }} / expires {{ formatTime(account.subscription.expires_at) }}</strong>
        </div>
        <div class="main-site">
          <span>Free upgrade</span>
          <strong>{{ account.free_upgrade.months_used }}/{{ account.free_upgrade.max_months }} months used</strong>
        </div>
        <div class="token">
          <strong>{{ t('enrollmentToken') }}</strong>
          <code>{{ adminToken }}</code>
          <p>{{ t('tokenNote') }}</p>
        </div>
        <p class="error" v-if="error">{{ error }}</p>
      </section>

      <section class="panel" v-if="account">
        <div class="section-title">
          <div>
            <h2>子网路由</h2>
            <p>主站点 Agent 上报的局域网 CIDR 会先进入待审核，批准后才会下发给客户端。</p>
          </div>
          <span>{{ account.capabilities.enable_subnet ? '套餐已支持' : '需要升级套餐' }}</span>
        </div>
        <div class="table route-table">
          <div class="row head route-row">
            <span>CIDR</span>
            <span>发布设备</span>
            <span>状态</span>
            <span>最近更新</span>
            <span>{{ t('actions') }}</span>
          </div>
          <div class="empty" v-if="(account.subnet_routes || []).length === 0">
            暂无子网路由。请在主站点 agent.json 中配置 advertise_routes 后启动 Agent。
          </div>
          <div class="row route-row" v-for="route in account.subnet_routes || []" :key="route.id">
            <span><code>{{ route.cidr }}</code></span>
            <span>{{ deviceLabel(route.device_id) }}</span>
            <span>
              <span class="badge" :class="{ primary: route.status === 'active' }">
                {{ route.status }} / {{ route.approved ? '已批准' : '待批准' }}
              </span>
            </span>
            <span>{{ formatTime(route.updated_at) }}</span>
            <span class="row-actions">
              <button
                class="small"
                type="button"
                :disabled="route.approved && route.status === 'active'"
                @click="approveSubnetRoute(route, true)"
              >
                批准
              </button>
              <button
                class="ghost small"
                type="button"
                :disabled="!route.approved"
                @click="approveSubnetRoute(route, false)"
              >
                取消批准
              </button>
              <button class="danger small" type="button" :disabled="!route.advertised" @click="disableSubnetRoute(route)">
                停用
              </button>
            </span>
          </div>
        </div>
      </section>

      <section class="panel" v-if="account">
        <div class="section-title">
          <div>
            <h2>Relay 模式</h2>
            <p>启用后客户端只连接 Relay peer，适合 P2P 打不通或需要稳定中转的网络。</p>
          </div>
          <button
            type="button"
            :disabled="!account.capabilities.enable_self_relay || !account.active_relay"
            @click="setRelayMode(!account.user.relay_mode)"
          >
            {{ account.user.relay_mode ? '关闭 Relay 模式' : '开启 Relay 模式' }}
          </button>
        </div>

        <div class="main-site">
          <span>当前模式</span>
          <strong>{{ account.user.relay_mode ? 'Relay 中转' : 'Hub/P2P' }}</strong>
        </div>

        <form class="relay-form" @submit.prevent="createRelay">
          <input v-model="relayForm.name" placeholder="Relay 名称" />
          <input v-model="relayForm.public_key" placeholder="Relay WireGuard 公钥" required />
          <input v-model="relayForm.endpoint" placeholder="Relay endpoint，例如 relay.example.com:51873" required />
          <input v-model="relayForm.virtual_ip" placeholder="Relay 虚拟 IP，默认 100.254.253.1" />
          <button type="submit" :disabled="!account.capabilities.enable_self_relay">创建 Relay</button>
        </form>

        <div class="token relay-token" v-if="relayToken">
          <strong>Relay Token 只显示一次</strong>
          <code>{{ relayToken }}</code>
          <p>请写入 relay-agent 配置文件，后续不会再次展示。</p>
        </div>

        <div class="table relay-table">
          <div class="row head relay-row">
            <span>名称</span>
            <span>虚拟 IP</span>
            <span>Endpoint</span>
            <span>状态</span>
            <span>{{ t('actions') }}</span>
          </div>
          <div class="empty" v-if="(account.relays || []).length === 0">
            暂无 Relay。升级到 Relay 版本后，可创建自建 Relay。
          </div>
          <div class="row relay-row" v-for="relay in account.relays || []" :key="relay.id">
            <span>{{ relay.name }}</span>
            <span><code>{{ relay.virtual_ip }}</code></span>
            <span><code>{{ relay.endpoint }}</code></span>
            <span>
              <span class="badge" :class="{ primary: relay.status === 'active' }">
                {{ relay.status }}
              </span>
            </span>
            <span class="row-actions">
              <button class="small" type="button" :disabled="relay.status === 'active'" @click="enableRelay(relay)">
                启用
              </button>
              <button class="danger small" type="button" :disabled="relay.status !== 'active'" @click="disableRelay(relay)">
                停用
              </button>
            </span>
          </div>
        </div>
      </section>

      <section class="panel">
        <div class="section-title">
          <h2>{{ t('deviceList') }}</h2>
          <span>{{ account?.user?.overlay_cidr }}</span>
        </div>
        <div class="table">
          <div class="row head device-row">
            <span>{{ t('hostname') }}</span>
            <span>{{ t('virtualIP') }}</span>
            <span>{{ t('role') }}</span>
            <span>{{ t('status') }}</span>
            <span>{{ t('lastSeen') }}</span>
            <span>{{ t('actions') }}</span>
          </div>
          <div class="empty" v-if="devices.length === 0">{{ t('noDevices') }}</div>
          <div
            class="row device-row clickable-row"
            v-for="device in devices"
            :key="device.id"
            @click="selectDevice(device)"
          >
            <span>{{ device.hostname }}</span>
            <span>{{ device.virtual_ip }}</span>
            <span><span class="badge" :class="{ primary: device.site_role === 'main_site' }">{{ roleLabel(device.site_role) }}</span></span>
            <span>{{ device.status }}</span>
            <span>{{ formatTime(device.last_seen_at) }}</span>
            <span class="row-actions">
              <button
                class="small"
                type="button"
                :disabled="device.site_role === 'main_site'"
                @click.stop="setMainSite(device)"
              >
                {{ device.site_role === 'main_site' ? t('mainSite') : t('setMainSite') }}
              </button>
              <button class="danger small" type="button" @click.stop="deleteDevice(device)">
                {{ t('deleteDevice') }}
              </button>
            </span>
          </div>
        </div>
      </section>

      <section class="panel detail" v-if="selectedDevice">
        <h2>{{ t('deviceDetail') }}</h2>
        <dl>
          <dt>{{ t('hostname') }}</dt>
          <dd>{{ selectedDevice.device.hostname }}</dd>
          <dt>{{ t('virtualIP') }}</dt>
          <dd>{{ selectedDevice.device.virtual_ip }}</dd>
          <dt>{{ t('os') }}</dt>
          <dd>{{ selectedDevice.device.os }}</dd>
          <dt>{{ t('arch') }}</dt>
          <dd>{{ selectedDevice.device.arch || '-' }}</dd>
          <dt>{{ t('status') }}</dt>
          <dd>{{ selectedDevice.device.status }}</dd>
          <dt>{{ t('role') }}</dt>
          <dd>{{ roleLabel(selectedDevice.device.site_role) }}</dd>
          <dt>{{ t('clientVersion') }}</dt>
          <dd>{{ selectedDevice.device.client_version }}</dd>
          <dt>{{ t('lastSeen') }}</dt>
          <dd>{{ formatTime(selectedDevice.device.last_seen_at) }}</dd>
          <dt>{{ t('createdAt') }}</dt>
          <dd>{{ formatTime(selectedDevice.device.created_at) }}</dd>
          <dt>{{ t('publicKey') }}</dt>
          <dd><code>{{ selectedDevice.device.public_key }}</code></dd>
        </dl>
        <h3>{{ t('endpoints') }}</h3>
        <div class="endpoint-list" v-if="selectedDevice.endpoints.length > 0">
          <div class="endpoint" v-for="endpoint in selectedDevice.endpoints" :key="endpoint.id">
            <span>{{ endpoint.endpoint_type }}</span>
            <code>{{ endpoint.address }}</code>
            <small>{{ endpoint.source }} · {{ formatTime(endpoint.updated_at) }}</small>
          </div>
        </div>
        <div class="empty inline" v-else>-</div>
      </section>

      <div class="modal-backdrop" v-if="showPlanModal" @click.self="showPlanModal = false">
        <section class="modal">
          <div class="section-title">
            <h2>{{ t('planTitle') }}</h2>
            <button class="ghost" type="button" @click="showPlanModal = false">{{ t('close') }}</button>
          </div>
          <p>{{ t('planDesc') }}</p>
          <div class="main-site" v-if="account?.subscription">
            <span>Active</span>
            <strong>{{ account.subscription.plan_code }} until {{ formatTime(account.subscription.expires_at) }}</strong>
            <button class="danger small" type="button" @click="cancelSubscription">Cancel</button>
          </div>
          <div class="main-site">
            <span>Free upgrade quota</span>
            <strong>{{ account?.free_upgrade?.months_remaining || 0 }} months remaining</strong>
          </div>
          <div class="plans">
            <article class="plan" v-for="plan in account?.plans || []" :key="plan.code">
              <strong>{{ plan.name }}</strong>
              <div class="price">{{ formatPrice(plan.price_cents) }} <span v-if="plan.price_cents">{{ t('monthly') }}</span></div>
              <p>{{ t('devices') }}: {{ plan.max_devices }}</p>
              <p>{{ t('subnetFeature') }}: {{ plan.enable_subnet ? t('enabled') : t('disabled') }}</p>
              <p>{{ t('relayFeature') }}: {{ plan.enable_self_relay ? t('enabled') : t('disabled') }}</p>
              <button
                v-if="plan.code !== 'free'"
                type="button"
                :disabled="!canFreeUpgrade(plan)"
                @click="freeUpgrade(plan)"
              >
                {{ freeUpgradeButtonText(plan) }}
              </button>
            </article>
          </div>
        </section>
      </div>
    </template>
  </main>
</template>
