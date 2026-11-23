<script setup>
import { computed, ref } from 'vue'
import { ArrowRight, ClipboardList, Monitor, Search, Server, Waypoints, X } from '@lucide/vue'
import { probeLabel, serviceRows } from './enterprise-status.js'

const props = defineProps({
  account: { type: Object, required: true },
  devices: { type: Array, required: true },
  view: { type: String, required: true },
  deviceLabel: { type: Function, required: true },
  formatTime: { type: Function, required: true },
})
const emit = defineEmits(['navigate'])
const routes = computed(() => props.account.subnet_routes || [])
const services = computed(() => serviceRows(routes.value))
const pending = computed(() => routes.value.filter((route) => route.advertised && !route.approved))
const search = ref('')
const dialog = ref(null)
const selection = ref(null)
const selected = computed(() => services.value.find(({ route, probe }) => route.id === selection.value?.id && probe.target === selection.value.target))
const rows = computed(() => props.view === 'overview' ? services.value.slice(0, 5) : services.value.filter(({ route, probe }) =>
  [route.cidr, probe.target, props.deviceLabel(route.device_id)].some((value) => String(value).toLowerCase().includes(search.value.trim().toLowerCase()))))

function openService(route, probe) {
  selection.value = { id: route.id, target: probe.target }
  dialog.value.showModal()
}
</script>

<template>
  <template v-if="view === 'overview'">
    <div class="metric-grid">
      <article class="metric-card"><span><Waypoints :size="19" /> 主站点网关</span><strong class="metric-state">{{ account.main_site?.hostname || '未设置' }}</strong><em>{{ account.main_site?.virtual_ip || '尚未指定 Hub' }}</em></article>
      <article class="metric-card"><span><Server :size="19" /> 内网服务探测</span><strong>{{ services.length }}</strong><em>已上报的 TCP 探测项</em></article>
      <article class="metric-card"><span><Monitor :size="19" /> 客户端设备</span><strong>{{ devices.filter((device) => device.id !== account.main_site?.id && device.site_role !== 'main_site').length }}</strong><em>不含主站点</em></article>
      <article class="metric-card"><span><ClipboardList :size="19" /> 待审批路由</span><strong>{{ pending.length }}</strong><em>已发布但尚未批准</em></article>
    </div>
    <section class="content-section">
      <div class="section-heading"><h2>待处理事项</h2></div>
      <div class="workspace-todo" v-if="!account.main_site"><span>指定一台设备作为主站点网关</span><button class="text-action" type="button" @click="emit('navigate', 'devices')">管理设备 <ArrowRight :size="16" /></button></div>
      <div class="workspace-todo" v-if="pending.length"><span>{{ pending.length }} 条已发布的子网路由待审批</span><button class="text-action" type="button" @click="emit('navigate', 'routes')">查看路由 <ArrowRight :size="16" /></button></div>
      <p class="table-empty" v-if="account.main_site && !pending.length">暂无待处理事项</p>
    </section>
  </template>

  <section class="content-section">
    <div class="section-heading workspace-heading">
      <div><h2>{{ view === 'overview' ? '内网服务概览' : '服务探测列表' }}</h2><p>探测从网关发起，员工端访问未验证。</p></div>
      <button v-if="view === 'overview'" class="text-action" type="button" @click="emit('navigate', 'services')">查看全部 <ArrowRight :size="16" /></button>
      <label v-else class="workspace-search"><Search :size="17" /><input v-model="search" type="search" aria-label="搜索内网服务" placeholder="搜索地址、网段或网关" /></label>
    </div>
    <div v-if="rows.length" class="workspace-table">
      <table>
        <thead><tr><th scope="col">服务地址</th><th scope="col">出口网关 / 子网</th><th scope="col">最近探测</th><th scope="col">最近上报</th><th scope="col">操作</th></tr></thead>
        <tbody><tr v-for="{ route, probe } in rows" :key="JSON.stringify([route.id, probe.target])">
          <td><code>{{ probe.target }}</code></td><td>{{ deviceLabel(route.device_id) }}<small>{{ route.cidr }}</small></td>
          <td>{{ probeLabel(probe, route.gateway_checked_at) }}</td><td>{{ formatTime(route.gateway_checked_at) }}</td>
          <td><button class="text-action" type="button" @click="openService(route, probe)">查看详情</button></td>
        </tr></tbody>
      </table>
    </div>
    <div v-else class="table-empty">
      {{ services.length ? '没有匹配的服务' : '尚未收到网关 TCP 服务探测结果' }}
      <button v-if="!services.length" class="text-action" type="button" @click="emit('navigate', 'routes')">查看网关与路由</button>
    </div>
  </section>

  <dialog ref="dialog" class="workspace-drawer" aria-labelledby="service-title" @close="selection = null">
    <div class="section-heading"><div><p>内网服务</p><h2 id="service-title">{{ selected?.probe.target || '服务详情' }}</h2></div><form method="dialog"><button class="mini-action" title="关闭" aria-label="关闭服务详情" autofocus><X :size="20" /></button></form></div>
    <dl class="workspace-details" v-if="selected">
      <dt>出口网关</dt><dd>{{ deviceLabel(selected.route.device_id) }} · {{ selected.route.cidr }}</dd>
      <dt>子网路由</dt><dd>{{ !selected.route.advertised ? '未发布' : !selected.route.approved ? '待审批' : selected.route.status === 'active' ? '已批准 · 生效' : '已批准 · 未生效' }}</dd>
      <dt>网关 TCP 探测</dt><dd>{{ probeLabel(selected.probe, selected.route.gateway_checked_at) }}<template v-if="selected.probe.reachable === true && selected.probe.latency_ms != null"> · {{ selected.probe.latency_ms }} ms</template><small v-if="selected.probe.error">{{ selected.probe.error }}</small></dd>
      <dt>最近上报</dt><dd>{{ formatTime(selected.route.gateway_checked_at) }}</dd>
      <dt>员工身份与访问授权</dt><dd>当前未采集</dd>
      <dt>员工端隧道与 LAN 服务访问</dt><dd>未验证</dd>
    </dl>
    <p v-else class="table-empty">此探测项已不在最近一次上报中。</p>
  </dialog>
</template>

<style scoped>
.metric-card { min-width: 0; overflow-wrap: anywhere; }
.metric-card svg { color: var(--console-accent, #427ad1); }
.workspace-todo { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; padding: 12px 0; border-top: 1px solid var(--console-border); }
.workspace-heading { flex-wrap: wrap; gap: 12px; }
.text-action { display: inline-flex; align-items: center; gap: 6px; }
.workspace-search { display: flex; align-items: center; gap: 8px; border: 1px solid var(--console-border); border-radius: 6px; padding: 0 10px; color: var(--console-muted); }
.workspace-search input { width: min(190px, 100%); min-width: 0; border: 0; background: transparent; color: var(--console-text); font: inherit; padding: 8px 0; }
.workspace-table { overflow-x: auto; }
.workspace-table table { width: 100%; min-width: 650px; border-collapse: collapse; text-align: left; }
.workspace-table th,.workspace-table td { padding: 12px; border-bottom: 1px solid var(--console-border); overflow-wrap: anywhere; }
.workspace-table th { color: var(--console-muted); font-size: 14px; }
.workspace-table td { font-size: 15px; }
.workspace-table small,.workspace-details small { display: block; color: var(--console-muted); margin-top: 4px; }
.workspace-drawer { position: fixed; inset: 0 0 0 auto; margin: 0; width: min(480px, 100%); max-width: 100%; height: 100%; max-height: 100%; padding: 24px; box-sizing: border-box; border: 0; border-left: 1px solid var(--console-border); background: var(--console-side); color: var(--console-text); overflow-wrap: anywhere; }
.workspace-drawer::backdrop { background: rgb(12 19 29 / 55%); }
.workspace-details dt { color: var(--console-muted); margin-top: 18px; }
.workspace-details dd { margin: 5px 0 0; }
</style>
