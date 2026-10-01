type DisplayTask = { id?: number; clients?: string[]; name?: string };

export function assignedDisplayTasks<T extends DisplayTask>(nodeUuid: string, tasks: T[]): T[] {
  return tasks.filter((task) =>
    Number.isSafeInteger(task.id) && (task.id ?? 0) > 0 && task.clients?.includes(nodeUuid),
  );
}

export function resolveDisplayPingTaskId(nodeUuid: string, tasks: DisplayTask[], preferredId: number): number {
  return assignedDisplayTasks(nodeUuid, tasks).some((task) => task.id === preferredId)
    ? preferredId
    : 0;
}
