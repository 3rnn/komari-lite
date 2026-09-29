import assert from "node:assert/strict";
import test from "node:test";
import {
  initialLoginStepState,
  nextLoginStepState,
} from "../src/utils/loginFlow.ts";

// The server's request for a one-time code is a flow signal: show the second step, not a red error.
// Otherwise valid credentials might appear invalid when the server says "2FA code is required".
test("a required one-time code moves to step two without an error", () => {
  const next = nextLoginStepState(initialLoginStepState, {
    ok: false,
    requiresTwoFactor: true,
    message: "2FA code is required",
  });
  assert.equal(next.require2FA, true);
  assert.equal(next.errorMsg, "");
});

// Invalid codes keep the user on step two and pass the server message to the UI for friendly wording.
test("an invalid one-time code keeps step two and preserves the message", () => {
  const twoFactorStep = { require2FA: true, errorMsg: "" };
  const next = nextLoginStepState(twoFactorStep, {
    ok: false,
    requiresTwoFactor: false,
    message: "Invalid 2FA code",
  });
  assert.equal(next.require2FA, true);
  assert.equal(next.errorMsg, "Invalid 2FA code");
});

// Invalid credentials remain on step one and display the server message.
test("invalid credentials stay on step one with an error", () => {
  const next = nextLoginStepState(initialLoginStepState, {
    ok: false,
    requiresTwoFactor: false,
    message: "Invalid credentials",
  });
  assert.equal(next.require2FA, false);
  assert.equal(next.errorMsg, "Invalid credentials");
});

test("successful login clears the error", () => {
  const next = nextLoginStepState({ require2FA: true, errorMsg: "Old error" }, { ok: true });
  assert.equal(next.errorMsg, "");
});
