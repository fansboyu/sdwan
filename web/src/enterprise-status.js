export function serviceRows(routes) {
  return routes.flatMap((route) => {
    try {
      const probes = JSON.parse(route.gateway_service_status || '[]')
      return Array.isArray(probes)
        ? probes.filter((probe) => typeof probe?.target === 'string' && probe.target.trim()).map((probe) => ({ route, probe }))
        : []
    } catch {
      return []
    }
  })
}

export function probeLabel(probe, checkedAt) {
  if (!checkedAt || !Number.isFinite(Date.parse(checkedAt))) return '未上报探测时间'
  if (probe.reachable === true) return '上次网关探测可达'
  if (probe.reachable === false) return '上次网关探测失败'
  return '未上报探测结果'
}
