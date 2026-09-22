export type RuntimeHealth = "healthy" | "pending" | "attention" | "idle";

type RuntimeHealthInput = {
  total: number;
  last_step_at: string | null;
  checkpoint_pending: boolean;
  consecutive_checkpoint_failures: number;
  consecutive_cycle_failures: number;
};

export function getDatabaseRuntimeHealth(
  runtime: RuntimeHealthInput,
): RuntimeHealth {
  if (runtime.total === 0 || runtime.last_step_at === null) return "idle";
  if (
    runtime.consecutive_checkpoint_failures >= 3 ||
    runtime.consecutive_cycle_failures >= 2
  ) {
    return "attention";
  }
  if (
    runtime.checkpoint_pending ||
    runtime.consecutive_checkpoint_failures > 0 ||
    runtime.consecutive_cycle_failures > 0
  ) {
    return "pending";
  }
  return "healthy";
}
