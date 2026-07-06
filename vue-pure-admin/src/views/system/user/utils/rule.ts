import { reactive } from "vue";
import type { FormRules } from "element-plus";
import { isPhone, isEmail } from "@pureadmin/utils";

export const passwordPolicyMessage =
  "密码必须为 8-64 位，且同时包含字母、数字和特殊字符，不能包含空白字符";

export function isStrongPassword(value: unknown) {
  const password = String(value ?? "");
  const hasLetter = /\p{L}/u.test(password);
  const hasDigit = /\p{N}/u.test(password);
  const hasSpecial = /[^\p{L}\p{N}]/u.test(password);
  return (
    password.length >= 8 &&
    password.length <= 64 &&
    !/[\s\u0000-\u001f\u007f]/.test(password) &&
    hasLetter &&
    hasDigit &&
    hasSpecial
  );
}

/** 自定义表单规则校验 */
export const formRules = reactive(<FormRules>{
  nickname: [{ required: true, message: "用户昵称为必填项", trigger: "blur" }],
  username: [{ required: true, message: "用户名称为必填项", trigger: "blur" }],
  password: [
    {
      validator: (rule, value, callback) => {
        if (value === "") {
          callback(new Error("用户密码为必填项"));
        } else if (!isStrongPassword(value)) {
          callback(new Error(passwordPolicyMessage));
        } else {
          callback();
        }
      },
      trigger: "blur"
    }
  ],
  phone: [
    {
      validator: (rule, value, callback) => {
        if (value === "") {
          callback();
        } else if (!isPhone(value)) {
          callback(new Error("请输入正确的手机号码格式"));
        } else {
          callback();
        }
      },
      trigger: "blur"
      // trigger: "click" // 如果想在点击确定按钮时触发这个校验，trigger 设置成 click 即可
    }
  ],
  email: [
    {
      validator: (rule, value, callback) => {
        if (value === "") {
          callback();
        } else if (!isEmail(value)) {
          callback(new Error("请输入正确的邮箱格式"));
        } else {
          callback();
        }
      },
      trigger: "blur"
    }
  ]
});
