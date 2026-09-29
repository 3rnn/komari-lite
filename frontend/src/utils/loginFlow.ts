import type { PasswordLoginResult } from "./adminAuth";

export type LoginStepState = {
  /** Whether the user has reached the one-time-code step. */
  require2FA: boolean;
  /** Error text to display; an empty string hides it. */
  errorMsg: string;
};

export const initialLoginStepState: LoginStepState = {
  require2FA: false,
  errorMsg: "",
};

/**
 * Derive the next UI state from a login attempt.
 *
 * The server responds with `2FA code is required` for 2FA-enabled accounts when no code is supplied.
 * That is a transition to step two, not an error; displaying it as an error wrongly suggests bad credentials.
 */
export function nextLoginStepState(
  previous: LoginStepState,
  result: PasswordLoginResult,
): LoginStepState {
  if (result.ok) {
    return { require2FA: previous.require2FA, errorMsg: "" };
  }
  if (result.requiresTwoFactor) {
    return { require2FA: true, errorMsg: "" };
  }
  return { require2FA: previous.require2FA, errorMsg: result.message };
}
