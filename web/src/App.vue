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
const devices = ref([])
const selectedDevice = ref(null)
const error = ref('')
const showPlanModal = ref(false)

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
  const response = await fetch(`${apiBase}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: email.value, password: password.value }),
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
  await loadDashboard()
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

onMounted(async () => {
  await loadMe()
  await loadDashboard()
})
</script>

<template>
  <main class="shell">
    <section class="topbar">
      <div>
        <p class="eyebrow">controller.englishlisten.cn</p>
        <h1>{{ t('title') }}</h1>
      </div>
      <div class="actions">
        <button class="ghost" type="button" @click="toggleLocale">{{ t('switchLanguage') }}</button>
        <button class="ghost" v-if="user" type="button" @click="logout">{{ t('logout') }}</button>
      </div>
    </section>

    <section class="panel" v-if="!user">
      <div>
        <h2>{{ authMode === 'login' ? t('login') : t('register') }}</h2>
        <p>{{ t('emailOnly') }}</p>
      </div>
      <div class="tabs">
        <button :class="{ active: authMode === 'login' }" type="button" @click="authMode = 'login'">
          {{ t('login') }}
        </button>
        <button :class="{ active: authMode === 'register' }" type="button" @click="authMode = 'register'">
          {{ t('register') }}
        </button>
      </div>
      <form class="auth-form" @submit.prevent="submitAuth">
        <input v-model="email" type="email" autocomplete="email" :placeholder="t('email')" required />
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          :placeholder="`${t('password')} - ${t('passwordHint')}`"
          required
        />
        <button type="submit">{{ authMode === 'login' ? t('login') : t('register') }}</button>
      </form>
      <p class="error" v-if="error">{{ error }}</p>
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
        <div class="capabilities">
          <span>{{ t('capabilities') }}</span>
          <strong>{{ t('subnetFeature') }}: {{ account.capabilities.enable_subnet ? t('enabled') : t('disabled') }}</strong>
          <strong>{{ t('relayFeature') }}: {{ account.capabilities.enable_self_relay ? t('enabled') : t('disabled') }}</strong>
        </div>
        <div class="token">
          <strong>{{ t('enrollmentToken') }}</strong>
          <code>{{ adminToken }}</code>
          <p>{{ t('tokenNote') }}</p>
        </div>
        <p class="error" v-if="error">{{ error }}</p>
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
            <span>{{ t('status') }}</span>
            <span>{{ t('lastSeen') }}</span>
          </div>
          <div class="empty" v-if="devices.length === 0">{{ t('noDevices') }}</div>
          <button
            class="row device-row clickable-row"
            v-for="device in devices"
            :key="device.id"
            type="button"
            @click="selectDevice(device)"
          >
            <span>{{ device.hostname }}</span>
            <span>{{ device.virtual_ip }}</span>
            <span>{{ device.status }}</span>
            <span>{{ formatTime(device.last_seen_at) }}</span>
          </button>
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
          <div class="plans">
            <article class="plan" v-for="plan in account?.plans || []" :key="plan.code">
              <strong>{{ plan.name }}</strong>
              <div class="price">{{ formatPrice(plan.price_cents) }} <span v-if="plan.price_cents">{{ t('monthly') }}</span></div>
              <p>{{ t('devices') }}: {{ plan.max_devices }}</p>
              <p>{{ t('subnetFeature') }}: {{ plan.enable_subnet ? t('enabled') : t('disabled') }}</p>
              <p>{{ t('relayFeature') }}: {{ plan.enable_self_relay ? t('enabled') : t('disabled') }}</p>
            </article>
          </div>
        </section>
      </div>
    </template>
  </main>
</template>
