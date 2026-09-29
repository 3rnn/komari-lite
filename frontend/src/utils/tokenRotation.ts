const rotationInProgressMessages = new Set([
  "a token rotation is already in progress",
]);

export function localizeTokenRotationError(message: unknown) {
  if (typeof message !== "string" || !message.trim()) return "Token rotation failed";
  if (rotationInProgressMessages.has(message.trim().toLocaleLowerCase())) {
    return "Token rotation is still in transition. Redeploy the Agent with the new token; rotation will be available again after the new token connects successfully.";
  }
  return message;
}
