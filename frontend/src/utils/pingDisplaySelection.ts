type DisplayTask = { id?: number; clients?: string[]; name?: string };

export function assignedDisplayTasks<T extends DisplayTask>(nodeUuid: string, tasks: T[]): Array<T & { id: number }> {
  return tasks.filter((task): task is T & { id: number } =>
    Number.isSafeInteger(task.id) && (task.id ?? 0) > 0 && Boolean(task.clients?.includes(nodeUuid)),
  );
}

export function resolveDisplayPingTaskIds(nodeUuid: string, tasks: DisplayTask[], preferredIds?: number[], legacyId = 0): number[] {
  const allowed = new Set(assignedDisplayTasks(nodeUuid, tasks).map((task) => task.id));
  const selected = preferredIds?.length ? preferredIds : [legacyId];
  return [...new Set(selected.filter((id) => Number.isSafeInteger(id) && id > 0 && allowed.has(id)))];
}
