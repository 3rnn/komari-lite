import type { PasswordLoginResult } from "./adminAuth";

export type LoginStepState = {
  /** 是否已经进入「输入动态口令」这一步 */
  require2FA: boolean;
  /** 展示给用户的错误文案（空字符串表示不显示） */
  errorMsg: string;
};

export const initialLoginStepState: LoginStepState = {
  require2FA: false,
  errorMsg: "",
};

/**
 * 根据一次登录尝试的结果推导下一步界面状态。
 *
 * 关键点：服务端在账号开启 2FA 且本次没带口令时返回 `2FA code is required`，
 * 这只是「流程进入第二步」的信号，不是错误——把它当红色报错抛给用户会让人以为账号有问题。
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
