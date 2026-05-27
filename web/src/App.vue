<script setup>
import { onMounted, ref } from 'vue'

const apiBase = import.meta.env.VITE_API_BASE_URL || ''
const locale = ref(localStorage.getItem('locale') || 'zh')
const adminToken = ref(localStorage.getItem('admin_token') || '')
const adminUser = ref(null)
const authMode = ref('login')
const email = ref('')
const password = ref('')
const version = ref(null)
const customers = ref([])
const customerName = ref('')
const createdToken = ref('')
const error = ref('')

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
    createTitle: '创建客户',
    createDesc: '每个客户默认获得 16 个 Overlay 地址和一个客户端接入 Token。',
    customerName: '客户名称',
    create: '创建',
    token: '接入 Token',
    customers: '客户列表',
    name: '名称',
    addressPool: '地址池',
    devices: '设备数',
    netmap: '网络视图',
    noCustomers: '暂无客户',
    api: '接口',
    switchLanguage: 'English',
    createFailed: '创建客户失败',
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
    createTitle: 'Create Customer',
    createDesc: 'Each customer receives a 16-address overlay pool and a client join token.',
    customerName: 'Customer name',
    create: 'Create',
    token: 'Join Token',
    customers: 'Customers',
    name: 'Name',
    addressPool: 'Address Pool',
    devices: 'Devices',
    netmap: 'Netmap',
    noCustomers: 'No customers yet',
    api: 'API',
    switchLanguage: '中文',
    createFailed: 'Create customer failed',
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

async function loadVersion() {
  const response = await fetch(`${apiBase}/api/v1/server/version`)
  version.value = await response.json()
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
  const payload = await response.json()
  adminUser.value = payload.admin_user
}

async function submitAuth() {
  error.value = ''
  const path = authMode.value === 'login' ? '/admin/auth/login' : '/admin/auth/register'
  const response = await fetch(`${apiBase}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email: email.value, password: password.value }),
  })
  const payload = await response.json()
  if (!response.ok) {
    error.value = payload.error || t('authFailed')
    return
  }
  adminToken.value = payload.token
  adminUser.value = payload.admin_user
  localStorage.setItem('admin_token', payload.token)
  password.value = ''
  await loadCustomers()
}

function logout() {
  adminToken.value = ''
  adminUser.value = null
  customers.value = []
  localStorage.removeItem('admin_token')
}

async function loadCustomers() {
  if (!adminToken.value) return
  const response = await fetch(`${apiBase}/admin/customers`, {
    headers: authHeaders(),
  })
  if (response.status === 401) {
    logout()
    return
  }
  const payload = await response.json()
  customers.value = payload.customers || []
}

async function createCustomer() {
  error.value = ''
  createdToken.value = ''
  const response = await fetch(`${apiBase}/admin/customers`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ name: customerName.value || 'default' }),
  })
  const payload = await response.json()
  if (!response.ok) {
    error.value = payload.error || t('createFailed')
    return
  }
  createdToken.value = payload.join_token
  customerName.value = ''
  await loadCustomers()
}

onMounted(async () => {
  await loadVersion()
  await loadMe()
  await loadCustomers()
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
        <button class="ghost" v-if="adminUser" type="button" @click="logout">{{ t('logout') }}</button>
        <div class="version" v-if="version">
          <span>{{ version.server_version }}</span>
          <small>{{ t('api') }} {{ version.api_version }}</small>
        </div>
      </div>
    </section>

    <section class="panel" v-if="!adminUser">
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
        <span>{{ adminUser.email }}</span>
      </section>

      <section class="panel">
        <div>
          <h2>{{ t('createTitle') }}</h2>
          <p>{{ t('createDesc') }}</p>
        </div>
        <form class="create" @submit.prevent="createCustomer">
          <input v-model="customerName" :placeholder="t('customerName')" />
          <button type="submit">{{ t('create') }}</button>
        </form>
        <p class="error" v-if="error">{{ error }}</p>
        <div class="token" v-if="createdToken">
          <strong>{{ t('token') }}</strong>
          <code>{{ createdToken }}</code>
        </div>
      </section>

      <section class="panel">
        <h2>{{ t('customers') }}</h2>
        <div class="table">
          <div class="row head">
            <span>{{ t('name') }}</span>
            <span>{{ t('addressPool') }}</span>
            <span>{{ t('devices') }}</span>
            <span>{{ t('netmap') }}</span>
          </div>
          <div class="empty" v-if="customers.length === 0">{{ t('noCustomers') }}</div>
          <div class="row" v-for="customer in customers" :key="customer.id">
            <span>{{ customer.name }}</span>
            <span>{{ customer.address_cidr }}</span>
            <span>{{ customer.max_devices }}</span>
            <span>v{{ customer.netmap_version }}</span>
          </div>
        </div>
      </section>
    </template>
  </main>
</template>
