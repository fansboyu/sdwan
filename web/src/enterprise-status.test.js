import { test } from 'node:test'
import assert from 'node:assert/strict'
import { serviceRows, probeLabel } from './enterprise-status.js'

test('service rows use actual probe targets and ignore malformed data', () => {
  assert.deepEqual(serviceRows([{ gateway_service_status: 'broken' }]), [])
  const [row] = serviceRows([{ id: 'route', gateway_service_status: '[{"target":"10.0.0.2:443","reachable":true}]' }])
  assert.equal(row.probe.target, '10.0.0.2:443')
  assert.equal(row.route.id, 'route')
})

test('successful historical probes make no employee reachability claim', () => {
  assert.equal(probeLabel({ reachable: true }), '未上报探测时间')
  assert.equal(probeLabel({ reachable: true }, '2026-06-15'), '上次网关探测可达')
  assert.equal(probeLabel({ reachable: false }, '2026-06-15'), '上次网关探测失败')
})
