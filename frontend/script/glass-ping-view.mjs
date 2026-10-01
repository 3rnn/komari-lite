// Pure node-card ping selection; the Glass asset patch embeds these functions.
export function resolveNodePingSelection(showAll, pingTaskSelection, displayId) {
  const hasOverride = Number.isSafeInteger(displayId) && displayId > 0;
  return {
    ...pingTaskSelection,
    showAll: hasOverride ? false : showAll,
    preferredId: hasOverride ? displayId : pingTaskSelection?.preferredId,
  };
}

export function selectPingTaskData(tasks, records, showAll, preferredId, makeHistory) {
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
  const target = showAll
    ? assigned
    : assigned.length
      ? [assigned.find((task) => Number(task.id) === Number(preferredId)) ?? assigned[0]]
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
  return { rows: showAll ? summaries : [], selected: showAll ? null : summaries[0] ?? null };
}
