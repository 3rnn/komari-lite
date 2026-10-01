// Pure node-card ping selection; the Glass asset patch embeds these functions.
export function resolveNodePingSelection(showAll, pingTaskSelection, displayIds, legacyId = 0) {
  const ids = [...new Set((Array.isArray(displayIds) && displayIds.length ? displayIds : [legacyId])
    .filter((id) => Number.isSafeInteger(id) && id > 0))];
  const hasOverride = ids.length > 0;
  return {
    ...pingTaskSelection,
    showAll: hasOverride ? ids.length > 1 : showAll,
    preferredId: hasOverride ? ids[0] : pingTaskSelection?.preferredId,
    ...(hasOverride ? { displayIds: ids } : {}),
    ...(ids.length > 1 ? { fallbackShowAll: showAll, fallbackPreferredId: pingTaskSelection?.preferredId } : {}),
  };
}

export function selectPingTaskData(tasks, records, showAll, preferredId, makeHistory, displayIds, fallbackShowAll, fallbackPreferredId) {
  const byTask = new Map();
  for (const record of records ?? []) {
    const id = Number(record.task_id);
    if (!Number.isInteger(id)) continue;
    const samples = byTask.get(id) ?? [];
    samples.push(record);
    byTask.set(id, samples);
  }
  // The public RPC already returns tasks in database weight/id order but does
  // not expose weight. Re-sorting by ID would silently change that order.
  const assigned = tasks ?? [];
  const filtered = Array.isArray(displayIds) && displayIds.length
    ? assigned.filter((task) => displayIds.includes(Number(task.id)))
    : [];
  const allStale = !filtered.length && fallbackShowAll !== undefined;
  const effectiveShowAll = allStale ? fallbackShowAll : showAll;
  const effectivePreferredId = allStale ? fallbackPreferredId : preferredId;
  const target = effectiveShowAll
    ? filtered.length ? filtered : assigned
    : assigned.length
      ? [assigned.find((task) => Number(task.id) === Number(effectivePreferredId)) ?? assigned[0]]
      : [];
  const summaries = target.map((task) => {
    const samples = byTask.get(Number(task.id)) ?? [];
    const successful = samples.filter((record) => Number.isFinite(record.value) && record.value >= 0);
    return {
      id: Number(task.id),
      name: task.name,
      avgLatency: successful.length
        ? successful.reduce((sum, record) => sum + record.value, 0) / successful.length
        : null,
      loss: samples.length ? (samples.length - successful.length) / samples.length * 100 : null,
      history: makeHistory(samples),
    };
  });
  return { rows: effectiveShowAll ? summaries : [], selected: effectiveShowAll ? null : summaries[0] ?? null, showAll: effectiveShowAll };
}
