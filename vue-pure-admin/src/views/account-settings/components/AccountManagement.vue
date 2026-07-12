<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import type { FormInstance, FormRules } from "element-plus";
import { changeMinePassword, getMine, type UserInfo } from "@/api/user";
import { message } from "@/utils/message";
import { deviceDetection } from "@pureadmin/utils";

defineOptions({
  name: "AccountManagement"
});

const emit = defineEmits<{
  (event: "switch-pane", pane: "profile" | "securityLog"): void;
}>();

type ManagementItem = {
  title: string;
  illustrate: string;
  button: string;
  action: "password" | "profile" | "securityLog";
};

const loading = ref(false);
const passwordSubmitting = ref(false);
const passwordDialogVisible = ref(false);
const passwordFormRef = ref<FormInstance>();
const profile = ref<UserInfo>({
  avatar: "",
  username: "",
  nickname: "",
  email: "",
  phone: "",
  description: ""
});

const passwordForm = reactive({
  oldPassword: "",
  newPassword: "",
  confirmPassword: ""
});

const passwordRules: FormRules = {
  oldPassword: [{ required: true, message: "请输入当前密码", trigger: "blur" }],
  newPassword: [
    { required: true, message: "请输入新密码", trigger: "blur" },
    { validator: validateNewPassword, trigger: "blur" }
  ],
  confirmPassword: [
    { required: true, message: "请再次输入新密码", trigger: "blur" },
    { validator: validateConfirmPassword, trigger: "blur" }
  ]
};

const list = computed<ManagementItem[]>(() => [
  {
    title: "账户密码",
    illustrate: "已启用后台账号密码登录。建议定期更新密码，并避免复用默认密码。",
    button: "修改密码",
    action: "password"
  },
  {
    title: "密保手机",
    illustrate: profile.value.phone
      ? `已绑定手机：${maskPhone(profile.value.phone)}`
      : "未绑定手机号，可在个人信息中补充。",
    button: profile.value.phone ? "修改" : "绑定",
    action: "profile"
  },
  {
    title: "备用邮箱",
    illustrate: profile.value.email
      ? `已绑定邮箱：${maskEmail(profile.value.email)}`
      : "未绑定邮箱，可在个人信息中补充。",
    button: profile.value.email ? "修改" : "绑定",
    action: "profile"
  },
  {
    title: "登录审计",
    illustrate: "后台登录记录、来源 IP 和登录状态可在安全日志中查看。",
    button: "查看",
    action: "securityLog"
  }
]);

function validateNewPassword(_rule: unknown, value: string, callback: (error?: Error) => void) {
  const password = String(value || "");
  if (
    password.length < 8 ||
    password.length > 64 ||
    !/[A-Za-z]/.test(password) ||
    !/\d/.test(password) ||
    !/[^A-Za-z0-9\s]/.test(password) ||
    /\s/.test(password)
  ) {
    callback(new Error("密码需为 8-64 位，并同时包含字母、数字和特殊字符，不能包含空白字符"));
    return;
  }
  callback();
}

function validateConfirmPassword(_rule: unknown, value: string, callback: (error?: Error) => void) {
  if (value !== passwordForm.newPassword) {
    callback(new Error("两次输入的新密码不一致"));
    return;
  }
  callback();
}

function maskPhone(value: string) {
  const text = String(value || "").trim();
  if (text.length < 7) return text;
  return `${text.slice(0, 3)}****${text.slice(-4)}`;
}

function maskEmail(value: string) {
  const text = String(value || "").trim();
  const [name, domain] = text.split("@");
  if (!name || !domain) return text;
  const visible = name.length <= 2 ? name.slice(0, 1) : name.slice(0, 3);
  return `${visible}***@${domain}`;
}

function onClick(item: ManagementItem) {
  if (item.action === "password") {
    passwordDialogVisible.value = true;
    return;
  }
  emit("switch-pane", item.action);
}

function resetPasswordForm() {
  passwordForm.oldPassword = "";
  passwordForm.newPassword = "";
  passwordForm.confirmPassword = "";
  passwordFormRef.value?.clearValidate();
}

async function submitPasswordChange() {
  const form = passwordFormRef.value;
  if (!form) return;
  const valid = await form.validate().catch(() => false);
  if (!valid) return;
  passwordSubmitting.value = true;
  try {
    const { code, message: msg } = await changeMinePassword({
      oldPassword: passwordForm.oldPassword,
      newPassword: passwordForm.newPassword
    });
    if (code !== 0) {
      message(msg || "修改密码失败", { type: "error" });
      return;
    }
    message("密码已修改", { type: "success" });
    passwordDialogVisible.value = false;
    resetPasswordForm();
  } finally {
    passwordSubmitting.value = false;
  }
}

onMounted(async () => {
  loading.value = true;
  try {
    const { code, data, message: msg } = await getMine();
    if (code !== 0) {
      message(msg || "加载账户信息失败", { type: "error" });
      return;
    }
    profile.value = data;
  } finally {
    loading.value = false;
  }
});
</script>

<template>
  <div
    v-loading="loading"
    :class="['min-w-45', deviceDetection() ? 'max-w-full' : 'max-w-[70%]']"
  >
    <h3 class="my-8!">账户管理</h3>
    <div v-for="(item, index) in list" :key="index">
      <div class="flex items-center">
        <div class="flex-1">
          <p>{{ item.title }}</p>
          <el-text class="mx-1" type="info">{{ item.illustrate }}</el-text>
        </div>
        <el-button type="primary" text @click="onClick(item)">
          {{ item.button }}
        </el-button>
      </div>
      <el-divider />
    </div>

    <el-dialog
      v-model="passwordDialogVisible"
      title="修改账户密码"
      width="420px"
      :close-on-click-modal="false"
      @closed="resetPasswordForm"
    >
      <el-form
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        label-width="96px"
      >
        <el-form-item label="当前密码" prop="oldPassword">
          <el-input
            v-model="passwordForm.oldPassword"
            type="password"
            show-password
            autocomplete="current-password"
            placeholder="请输入当前密码"
          />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="passwordForm.newPassword"
            type="password"
            show-password
            autocomplete="new-password"
            placeholder="8-64 位，含字母、数字和特殊字符"
          />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="passwordForm.confirmPassword"
            type="password"
            show-password
            autocomplete="new-password"
            placeholder="请再次输入新密码"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="passwordSubmitting"
          @click="submitPasswordChange"
        >
          确认修改
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style lang="scss" scoped>
.el-divider--horizontal {
  border-top: 0.1px var(--el-border-color) var(--el-border-style);
}
</style>
