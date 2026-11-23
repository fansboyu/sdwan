<script setup>
import { computed, onMounted, ref } from 'vue'
import { CheckCircle2, Clipboard, Clock, KeyRound, Server, ShieldCheck, Waypoints } from '@lucide/vue'

const props = defineProps({ account: Object, apiBase: String, headers: Object })
const emit = defineEmits(['navigate'])
const issued = ref(null)
const items = ref([])
const verifications = ref([])
const message = ref('')
const error = ref('')
const busy = ref(false)
const hub = computed(() => props.account.main_site)
const hubOnline = computed(() => {
  const seen = Date.parse(hub.value?.last_seen_at)
  return Number.isFinite(seen) && Date.now() - seen < 60000 && Date.now() >= seen
})
const approved = computed(() => (props.account.subnet_routes || []).filter((route) =>
  route.device_id === hub.value?.id && route.approved && route.advertised && route.status === 'active'))

async function request(path = '', method = 'GET') {
  const response = await fetch(`${props.apiBase}/admin/enrollments${path}`, { method, headers: props.headers, cache: 'no-store' })
  if (!response.ok) throw new Error(response.status === 404 ? '凭证已不处于可撤销状态，请刷新。' : '无法操作入网凭证，请重试。')
  return response.status === 204 ? null : response.json()
}

async function run(action) {
  busy.value = true
  error.value = ''
  message.value = ''
  try { await action() } catch (err) { error.value = err.message } finally { busy.value = false }
}

function issue() {
  return run(async () => {
    issued.value = await request('', 'POST')
    items.value = await request()
  })
}

function revoke(item) {
  return run(async () => {
    await request('/' + encodeURIComponent(item.id), 'DELETE')
    if (issued.value?.enrollment.id === item.id) issued.value = null
    items.value = await request()
  })
}

async function copy() {
  try {
    await navigator.clipboard.writeText(issued.value.token)
    message.value = '已复制'
  } catch {
    error.value = '无法写入剪贴板，请手动复制。'
  }
}

function status(item) {
  if (item.used_at) return '已使用'
  if (item.revoked_at) return '已撤销'
  return Date.parse(item.expires_at) > Date.now() ? '可使用' : '已过期'
}
const date = (value) => new Date(value).toLocaleString()
function loadVerifications() {
  return run(async () => {
    const response = await fetch(`${props.apiBase}/admin/lan-verifications`, { headers: props.headers, cache: 'no-store' })
    if (!response.ok) throw new Error('无法读取员工端验证结果。')
    verifications.value = await response.json()
  })
}
onMounted(async () => {
  await run(async () => { items.value = await request() })
  await loadVerifications()
})
</script>

<template>
  <section class="content-section setup">
    <div class="section-heading"><div><h2>部署进度</h2></div></div>
    <ol class="steps">
      <li><Server :size="20" /><div><strong>主站点 Hub</strong><span>{{ hubOnline ? '已上线' : hub ? '近期未上报' : '待设置' }}<template v-if="hub"> · {{ hub.hostname }}</template></span></div><button class="text-action" @click="emit('navigate', 'devices')">查看设备</button></li>
      <li><Waypoints :size="20" /><div><strong>批准内网路由</strong><span>{{ approved.length ? `${approved.length} 条已生效` : '待批准' }}</span></div><button class="text-action" @click="emit('navigate', 'routes')">查看路由</button></li>
      <li><ShieldCheck :size="20" /><div><strong>员工端访问验证</strong><span>{{ !verifications.length ? '待验证 · Hub 探测不等于员工端可达' : verifications[0].reachable ? '最近一次员工端 TCP 探测通过' : '最近一次员工端 TCP 探测未通过' }}</span><span v-if="verifications.length">{{ verifications[0].device }} · {{ verifications[0].target }} · {{ date(verifications[0].checked_at) }}</span></div><button class="text-action" :disabled="busy" @click="loadVerifications">刷新结果</button></li>
    </ol>
    <details class="setup-commands">
      <summary>Linux Hub 与员工端验收命令</summary>
      <ol>
        <li>在 Linux 安装 Agent，创建一次性凭证后注册，再启动服务。<pre><code>sudo sdwan-agent register --controller CONTROLLER_URL --enrollment-token TOKEN
sudo systemctl enable --now sdwan-agent</code></pre></li>
        <li>在「成员设备」设为主站点，发布网段并开启转发，再到「网关与路由」批准。<pre><code>sudo sdwan-agent routes add 192.168.50.0/24
sudo sdwan-agent subnet-gateway enable --lan-cidr 192.168.50.0/24</code></pre></li>
        <li>使用已入网的 Linux 员工节点验证目标 TCP 端口。<pre><code>sudo sdwan-agent verify-lan --target 192.168.50.10:443</code></pre></li>
      </ol>
    </details>
  </section>
  <section class="content-section setup">
    <div class="section-heading"><div><h2>一次性设备入网凭证</h2><p>仅用于注册一台设备，30 分钟后失效。</p></div><button class="action-button primary" :disabled="busy" @click="issue"><KeyRound :size="17" /> 创建凭证</button></div>
    <p v-if="error" role="alert">{{ error }}</p><p v-if="message" role="status">{{ message }}</p>
    <div v-if="issued" class="credential">
      <strong>仅在本次显示</strong><code>{{ issued.token }}</code>
      <button class="mini-action" @click="copy"><Clipboard :size="16" /> 复制</button>
      <button class="text-action" @click="issued = null">隐藏</button>
    </div>
    <ul class="issued-list">
      <li v-for="item in items" :key="item.id">
        <span><CheckCircle2 v-if="item.used_at" :size="16" /><Clock v-else :size="16" /> {{ status(item) }}</span>
        <span>{{ date(item.expires_at) }} 到期</span>
        <button v-if="status(item) === '可使用'" class="mini-action" :disabled="busy" @click="revoke(item)">撤销</button>
      </li>
    </ul>
    <p v-if="!items.length && !busy && !error" class="table-empty">暂无入网凭证</p>
  </section>
</template>

<style scoped>
.steps,.issued-list { list-style:none; padding:0; margin:0; }
.steps li,.issued-list li { display:grid; grid-template-columns:auto minmax(0,1fr) auto; gap:12px; align-items:center; padding:14px 0; border-top:1px solid var(--console-border); }
.steps li > div { display:grid; gap:4px; }
.steps span,.issued-list span { color:var(--console-muted); font-size:14px; }
.issued-list span,.action-button,.mini-action { display:inline-flex; align-items:center; gap:7px; }
.credential { display:flex; flex-wrap:wrap; align-items:center; gap:10px; margin:12px 0; padding:12px; border:1px solid var(--console-border); border-radius:6px; }
.credential code { overflow-wrap:anywhere; min-width:0; }
.section-heading { flex-wrap:wrap; }
.setup-commands { border-top:1px solid var(--console-border); padding-top:14px; }
.setup-commands summary { cursor:pointer; }
.setup-commands pre { overflow-x:auto; font-size:13px; }
</style>
