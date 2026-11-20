export function appliedGeneration(applied, target) {
  if (applied == null) return '未上报'
  return applied === target ? `代次 ${applied}` : `代次 ${applied}（与目标不同）`
}

export function overlayStatus(path, now = Date.now()) {
  if (path.client_applied_generation !== path.generation || path.hub_applied_generation !== path.generation) return '等待双端应用'
  if (path.verification_state === 'failed') return '验证失败'
  if (path.verification_state === 'path_mismatch') return '两端路径不一致'
  if (path.verification_state !== 'verified' || path.verified_generation !== path.generation || !path.verified_at) return '未验证'
  return now - Date.parse(path.verified_at) > 60_000 ? '验证已过期' : 'Overlay 已验证'
}
