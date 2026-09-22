export type BackendOrderedNode = {
  uuid: string;
  weight?: number | null;
  created_at?: string | null;
};

export const compareNodesByBackendOrder = (
  left: BackendOrderedNode,
  right: BackendOrderedNode
): number => {
  const weightDifference = (left.weight ?? 0) - (right.weight ?? 0);
  if (weightDifference !== 0) return weightDifference;

  const leftCreatedAt = left.created_at ?? "";
  const rightCreatedAt = right.created_at ?? "";
  if (leftCreatedAt !== rightCreatedAt) {
    return leftCreatedAt < rightCreatedAt ? -1 : 1;
  }
  if (left.uuid === right.uuid) return 0;
  return left.uuid < right.uuid ? -1 : 1;
};
