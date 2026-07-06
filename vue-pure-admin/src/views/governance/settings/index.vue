<script setup lang="ts">
import dayjs from "dayjs";
import { computed, onMounted, reactive, ref } from "vue";
import { type FormInstance, type FormRules } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  listAdminSettings,
  updateAdminSetting,
  type AdminSetting,
  type AdminSettingPayload
} from "@/api/admin";
import { useRenderIcon } from "@/components/ReIcon/src/hooks";

defineOptions({
  name: "GovernanceSettings"
});

type SettingRow = Partial<AdminSetting> & Record<string, any>;
type DialogMode = "create" | "edit";

const loading = ref(false);
const saving = ref(false);
const dialogVisible = ref(false);
const dialogMode = ref<DialogMode>("edit");
const formRef = ref<FormInstance>();
const settings = ref<AdminSetting[]>([]);
const authSaving = ref(false);
const settingMap = ref<Record<string, AdminSetting>>({});
const editingOriginalValue = ref("");

const query = reactive({
  group: "",
  status: 0,
  pageSize: 20,
  currentPage: 1,
  total: 0
});

const form = reactive({
  key: "",
  value: "",
  group: "site",
  valueType: "string",
  description: "",
  status: 2,
  clearValue: false
});

const authForm = reactive({
  passwordEnabled: true,
  registerEnabled: true,
  callbackUrl: "http://127.0.0.1:5173/auth/callback",
  githubEnabled: false,
  githubClientId: "",
  githubClientSecret: "",
  githubMinYears: 3,
  googleEnabled: false,
  googleClientId: "",
  googleClientSecret: "",
  qqEnabled: false,
  qqClientId: "",
  qqClientSecret: "",
  webmasterUsername: "webmaster",
  webmasterPassword: ""
});

const authSecretConfigured = reactive({
  github: false,
  google: false,
  qq: false,
  webmaster: false
});

const authClear = reactive({
  github: false,
  google: false,
  qq: false,
  webmaster: false
});

const canList = computed(() => hasPerms("governance:list_settings"));
const canUpdate = computed(() => hasPerms("governance:update_setting"));
const dialogTitle = computed(() =>
  dialogMode.value === "create" ? "新增配置" : "编辑配置"
);
const authPreview = computed(() => ({
  password: authTogglePreview(
    authForm.passwordEnabled,
    "C 端显示账号密码登录",
    "C 端隐藏账号密码登录"
  ),
  register: authTogglePreview(
    authForm.passwordEnabled && authForm.registerEnabled,
    "C 端允许账号密码注册",
    authForm.passwordEnabled ? "C 端隐藏账号密码注册" : "密码登录关闭时注册也不可用"
  ),
  github: oauthProviderPreview({
    enabled: authForm.githubEnabled,
    clientId: authForm.githubClientId,
    secret: authForm.githubClientSecret,
    secretConfigured: authSecretConfigured.github,
    clearSecret: authClear.github,
    label: "GitHub",
    extra: `账号年限不少于 ${authForm.githubMinYears || 3} 年`
  }),
  google: oauthProviderPreview({
    enabled: authForm.googleEnabled,
    clientId: authForm.googleClientId,
    secret: authForm.googleClientSecret,
    secretConfigured: authSecretConfigured.google,
    clearSecret: authClear.google,
    label: "Google"
  }),
  qq: oauthProviderPreview({
    enabled: authForm.qqEnabled,
    clientId: authForm.qqClientId,
    secret: authForm.qqClientSecret,
    secretConfigured: authSecretConfigured.qq,
    clearSecret: authClear.qq,
    label: "QQ"
  }),
  webmaster: webmasterPreview()
}));

type AuthStatusType = "success" | "warning" | "info";

type AuthPreview = {
  description: string;
  label: string;
  type: AuthStatusType;
};

function authTogglePreview(
  enabled: boolean,
  enabledText: string,
  disabledText: string
): AuthPreview {
  return {
    description: enabled ? enabledText : disabledText,
    label: enabled ? "C 端显示" : "C 端隐藏",
    type: enabled ? "success" : "info"
  };
}

function oauthProviderPreview({
  clientId,
  clearSecret,
  enabled,
  extra = "",
  label,
  secret,
  secretConfigured
}: {
  clientId: string;
  clearSecret: boolean;
  enabled: boolean;
  extra?: string;
  label: string;
  secret: string;
  secretConfigured: boolean;
}): AuthPreview {
  const missing: string[] = [];
  if (!enabled) missing.push("启用开关");
  if (!clientId.trim()) missing.push("Client ID");
  if (clearSecret || (!secretConfigured && !secret.trim())) {
    missing.push("Client Secret");
  }
  if (missing.length === 0) {
    return {
      description: `${label} 会在 C 端登录/注册入口显示${extra ? `，${extra}` : ""}`,
      label: "C 端显示",
      type: "success"
    };
  }
  return {
    description: `缺少 ${missing.join("、")}，${label} 会在 C 端禁用态显示`,
    label: enabled ? "待配置" : "未启用",
    type: enabled ? "warning" : "info"
  };
}

function webmasterPreview(): AuthPreview {
  const missing: string[] = [];
  if (!authForm.webmasterUsername.trim()) missing.push("用户名");
  if (
    authClear.webmaster ||
    (!authSecretConfigured.webmaster && !authForm.webmasterPassword.trim())
  ) {
    missing.push("密码");
  }
  if (missing.length === 0) {
    return {
      description: "站长账号可在 C 端账号密码登录入口使用",
      label: "可登录",
      type: "success"
    };
  }
  return {
    description: `缺少 ${missing.join("、")}，站长账号不可登录`,
    label: "不可登录",
    type: "warning"
  };
}

const columns: TableColumnList = [
  { prop: "key", label: "配置键", minWidth: 180, showOverflowTooltip: true },
  { label: "分组", width: 110, slot: "group" },
  { label: "类型", width: 100, slot: "valueType" },
  { label: "配置值", minWidth: 260, slot: "value" },
  {
    prop: "description",
    label: "说明",
    minWidth: 220,
    showOverflowTooltip: true
  },
  { label: "状态", width: 100, slot: "status" },
  { label: "更新时间", width: 170, slot: "updatedAt" },
  { label: "操作", fixed: "right", width: 110, slot: "operation" }
];

const groupOptions = [
  { label: "全部", value: "" },
  { label: "站点", value: "site" },
  { label: "登录", value: "auth" },
  { label: "SEO", value: "seo" },
  { label: "上传", value: "upload" },
  { label: "邮件", value: "email" },
  { label: "安全", value: "security" },
  { label: "运营", value: "operation" }
];

const statusOptions = [
  { label: "全部", value: 0 },
  { label: "启用", value: 2 },
  { label: "停用", value: 1 }
];

const valueTypeOptions = [
  { label: "字符串", value: "string" },
  { label: "数字", value: "int" },
  { label: "布尔", value: "bool" },
  { label: "密码", value: "password" },
  { label: "JSON", value: "json" },
  { label: "文本", value: "text" }
];

const rules: FormRules = {
  key: [
    { required: true, message: "请输入配置键", trigger: "blur" },
    {
      pattern: /^[a-z0-9_.-]+$/,
      message: "只允许小写字母、数字、下划线、点和短横线",
      trigger: "blur"
    }
  ],
  group: [{ required: true, message: "请选择分组", trigger: "change" }],
  valueType: [{ required: true, message: "请选择类型", trigger: "change" }],
  status: [{ required: true, message: "请选择状态", trigger: "change" }]
};

function statusMeta(status?: number) {
  switch (status) {
    case 2:
      return { label: "启用", type: "success" as const };
    case 1:
      return { label: "停用", type: "info" as const };
    default:
      return { label: `未知(${status ?? "-"})`, type: "danger" as const };
  }
}

function groupLabel(group?: string) {
  return groupOptions.find(item => item.value === group)?.label ?? group ?? "-";
}

function valueTypeOf(row: SettingRow) {
  return row.value_type ?? row.valueType ?? "string";
}

function valueTypeLabel(type?: string) {
  return (
    valueTypeOptions.find(item => item.value === type)?.label ?? type ?? "-"
  );
}

function displayValue(row: SettingRow) {
  if (valueTypeOf(row) === "password") {
    return row.value ? "已配置" : "未配置";
  }
  return row.value ?? "-";
}

function updatedAt(row: SettingRow) {
  return row.updated_at ?? row.updatedAt;
}

function formatTime(value?: number) {
  if (!value) return "-";
  const timestamp = value > 9999999999 ? value : value * 1000;
  return dayjs(timestamp).format("YYYY-MM-DD HH:mm");
}

function resetFormModel() {
  form.key = "";
  form.value = "";
  form.group = "site";
  form.valueType = "string";
  form.description = "";
  form.status = 2;
  form.clearValue = false;
  formRef.value?.clearValidate();
}

function buildPayload(): AdminSettingPayload {
  return {
    key: form.key.trim(),
    value: form.value.trim(),
    group: form.group.trim(),
    value_type: form.valueType.trim(),
    description: form.description.trim(),
    status: form.status,
    clear_value: form.clearValue
  };
}

function settingValue(key: string, fallback = "") {
  return settingMap.value[key]?.value ?? fallback;
}

function settingBool(key: string, fallback = false) {
  const value = settingValue(key, fallback ? "true" : "false")
    .trim()
    .toLowerCase();
  return ["1", "true", "yes", "on"].includes(value);
}

function settingNumber(key: string, fallback: number) {
  const parsed = Number(settingValue(key, String(fallback)));
  return Number.isFinite(parsed) ? parsed : fallback;
}

function applyAuthSettings() {
  authForm.passwordEnabled = settingBool("auth.password.enabled", true);
  authForm.registerEnabled = settingBool("auth.register.enabled", true);
  authForm.callbackUrl = settingValue(
    "auth.oauth.frontend_callback_url",
    "http://127.0.0.1:5173/auth/callback"
  );
  authForm.githubEnabled = settingBool("auth.github.enabled", false);
  authForm.githubClientId = settingValue("auth.github.client_id");
  authSecretConfigured.github =
    settingValue("auth.github.client_secret").trim() !== "";
  authForm.githubClientSecret = "";
  authForm.githubMinYears = settingNumber("auth.github.min_account_years", 3);
  authClear.github = false;
  authForm.googleEnabled = settingBool("auth.google.enabled", false);
  authForm.googleClientId = settingValue("auth.google.client_id");
  authSecretConfigured.google =
    settingValue("auth.google.client_secret").trim() !== "";
  authForm.googleClientSecret = "";
  authClear.google = false;
  authForm.qqEnabled = settingBool("auth.qq.enabled", false);
  authForm.qqClientId = settingValue("auth.qq.client_id");
  authSecretConfigured.qq = settingValue("auth.qq.client_secret").trim() !== "";
  authForm.qqClientSecret = "";
  authClear.qq = false;
  authForm.webmasterUsername = settingValue(
    "site.webmaster.username",
    "webmaster"
  );
  authSecretConfigured.webmaster =
    settingValue("site.webmaster.password").trim() !== "";
  authForm.webmasterPassword = "";
  authClear.webmaster = false;
}

async function loadAuthSettings() {
  if (!canList.value) {
    settingMap.value = {};
    return;
  }
  const [authResp, siteResp] = await Promise.all([
    listAdminSettings({ group: "auth", status: 2, limit: 100, offset: 0 }),
    listAdminSettings({ group: "site", status: 2, limit: 100, offset: 0 })
  ]);
  if (authResp.code !== 0 || siteResp.code !== 0) {
    message("加载登录配置失败", { type: "error" });
    return;
  }
  const next: Record<string, AdminSetting> = {};
  for (const item of [...(authResp.data.items ?? []), ...(siteResp.data.items ?? [])]) {
    next[item.key] = item;
  }
  settingMap.value = next;
  applyAuthSettings();
}

function authPayload(
  key: string,
  value: string,
  group: string,
  valueType: string,
  description: string
): AdminSettingPayload {
  return { key, value, group, value_type: valueType, description, status: 2 };
}

function appendSecretPayload(
  payloads: AdminSettingPayload[],
  key: string,
  value: string,
  group: string,
  description: string,
  clearValue = false
) {
  const nextValue = value.trim();
  if (!clearValue && !nextValue) return;
  payloads.push({
    ...authPayload(key, clearValue ? "" : nextValue, group, "password", description),
    clear_value: clearValue
  });
}

async function saveAuthSettings() {
  if (!canUpdate.value) {
    message("没有维护登录配置权限", { type: "warning" });
    return;
  }
  authSaving.value = true;
  try {
    const payloads: AdminSettingPayload[] = [
      authPayload(
        "auth.password.enabled",
        String(authForm.passwordEnabled),
        "auth",
        "bool",
        "是否允许 C 端账号密码登录。"
      ),
      authPayload(
        "auth.register.enabled",
        String(authForm.registerEnabled),
        "auth",
        "bool",
        "是否允许 C 端账号密码注册。"
      ),
      authPayload(
        "auth.oauth.frontend_callback_url",
        authForm.callbackUrl.trim(),
        "auth",
        "string",
        "C 端 OAuth 登录完成后的回跳地址。"
      ),
      authPayload(
        "auth.github.enabled",
        String(authForm.githubEnabled),
        "auth",
        "bool",
        "是否开启 GitHub 登录。"
      ),
      authPayload(
        "auth.github.client_id",
        authForm.githubClientId.trim(),
        "auth",
        "string",
        "GitHub OAuth Client ID。"
      ),
      authPayload(
        "auth.github.min_account_years",
        String(authForm.githubMinYears || 3),
        "auth",
        "int",
        "GitHub 登录要求账号至少创建的年限。"
      ),
      authPayload(
        "auth.google.enabled",
        String(authForm.googleEnabled),
        "auth",
        "bool",
        "是否开启 Google 登录。"
      ),
      authPayload(
        "auth.google.client_id",
        authForm.googleClientId.trim(),
        "auth",
        "string",
        "Google OAuth Client ID。"
      ),
      authPayload(
        "auth.qq.enabled",
        String(authForm.qqEnabled),
        "auth",
        "bool",
        "是否开启 QQ 登录。"
      ),
      authPayload(
        "auth.qq.client_id",
        authForm.qqClientId.trim(),
        "auth",
        "string",
        "QQ Connect App ID。"
      ),
      authPayload(
        "site.webmaster.username",
        authForm.webmasterUsername.trim(),
        "site",
        "string",
        "C 端站长账号用户名。"
      )
    ];
    appendSecretPayload(
      payloads,
      "auth.github.client_secret",
      authForm.githubClientSecret,
      "auth",
      "GitHub OAuth Client Secret。",
      authClear.github
    );
    appendSecretPayload(
      payloads,
      "auth.google.client_secret",
      authForm.googleClientSecret,
      "auth",
      "Google OAuth Client Secret。",
      authClear.google
    );
    appendSecretPayload(
      payloads,
      "auth.qq.client_secret",
      authForm.qqClientSecret,
      "auth",
      "QQ Connect App Key。",
      authClear.qq
    );
    appendSecretPayload(
      payloads,
      "site.webmaster.password",
      authForm.webmasterPassword,
      "site",
      "C 端站长账号密码；为空时不启用站长账号直登。",
      authClear.webmaster
    );
    for (const payload of payloads) {
      const { code, message: msg } = await updateAdminSetting(
        payload.key,
        payload
      );
      if (code !== 0) {
        message(msg || `保存 ${payload.key} 失败`, { type: "error" });
        return;
      }
    }
    message("登录配置已保存", { type: "success" });
    await Promise.all([loadAuthSettings(), loadSettings()]);
  } finally {
    authSaving.value = false;
  }
}

async function loadSettings() {
  if (!canList.value) {
    settings.value = [];
    query.total = 0;
    return;
  }
  loading.value = true;
  try {
    const {
      code,
      data,
      message: msg
    } = await listAdminSettings({
      group: query.group,
      status: query.status,
      limit: query.pageSize,
      offset: (query.currentPage - 1) * query.pageSize
    });
    if (code !== 0) {
      message(msg || "加载站点设置失败", { type: "error" });
      return;
    }
    settings.value = data.items ?? [];
    query.total = data.total ?? settings.value.length;
  } finally {
    loading.value = false;
  }
}

function resetQuery() {
  query.group = "";
  query.status = 0;
  query.currentPage = 1;
  loadSettings();
}

function openCreateDialog() {
  if (!canUpdate.value) {
    message("没有维护站点设置权限", { type: "warning" });
    return;
  }
  dialogMode.value = "create";
  editingOriginalValue.value = "";
  resetFormModel();
  dialogVisible.value = true;
}

function openEditDialog(row: SettingRow) {
  if (!canUpdate.value) {
    message("没有维护站点设置权限", { type: "warning" });
    return;
  }
  dialogMode.value = "edit";
  editingOriginalValue.value = row.value ?? "";
  form.key = row.key ?? "";
  form.group = row.group ?? "site";
  form.valueType = valueTypeOf(row);
  form.value = form.valueType === "password" ? "" : row.value ?? "";
  form.description = row.description ?? "";
  form.status = Number(row.status ?? 2);
  form.clearValue = false;
  formRef.value?.clearValidate();
  dialogVisible.value = true;
}

async function saveSetting() {
  if (!canUpdate.value) {
    message("没有维护站点设置权限", { type: "warning" });
    return;
  }
  const valid = await formRef.value?.validate().catch(() => false);
  if (!valid) return;
  const payload = buildPayload();
  if (
    dialogMode.value === "edit" &&
    payload.value_type === "password" &&
    !payload.clear_value &&
    !payload.value
  ) {
    payload.value = editingOriginalValue.value;
  }
  if (!payload.key) {
    message("请输入配置键", { type: "warning" });
    return;
  }
  saving.value = true;
  try {
    const { code, message: msg } = await updateAdminSetting(
      payload.key,
      payload
    );
    if (code !== 0) {
      message(msg || "保存站点设置失败", { type: "error" });
      return;
    }
    message("站点设置已保存", { type: "success" });
    dialogVisible.value = false;
    await loadSettings();
  } finally {
    saving.value = false;
  }
}

function onPageSizeChange(size: number) {
  query.pageSize = size;
  query.currentPage = 1;
  loadSettings();
}

function onCurrentPageChange(page: number) {
  query.currentPage = page;
  loadSettings();
}

onMounted(() => {
  loadAuthSettings();
  loadSettings();
});
</script>

<template>
  <div class="settings-page">
    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>登录配置</h2>
          <p>维护 C 端账号密码、第三方登录和站长账号</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/save-line')"
          :disabled="!canUpdate"
          :loading="authSaving"
          @click="saveAuthSettings"
        >
          保存登录配置
        </el-button>
      </div>

      <el-form label-width="132px" class="auth-config-form">
        <div class="auth-config-grid">
          <div class="auth-config-block">
            <h3>账号密码</h3>
            <el-form-item label="密码登录">
              <el-switch v-model="authForm.passwordEnabled" />
            </el-form-item>
            <el-form-item label="开放注册">
              <el-switch v-model="authForm.registerEnabled" />
            </el-form-item>
            <el-form-item label="OAuth 回跳">
              <el-input
                v-model="authForm.callbackUrl"
                placeholder="https://bbs.example.com/auth/callback"
              />
            </el-form-item>
            <div class="auth-preview-list">
              <div class="auth-preview-row">
                <el-tag :type="authPreview.password.type" effect="plain">
                  {{ authPreview.password.label }}
                </el-tag>
                <span>{{ authPreview.password.description }}</span>
              </div>
              <div class="auth-preview-row">
                <el-tag :type="authPreview.register.type" effect="plain">
                  {{ authPreview.register.label }}
                </el-tag>
                <span>{{ authPreview.register.description }}</span>
              </div>
            </div>
          </div>

          <div class="auth-config-block">
            <h3>GitHub</h3>
            <el-form-item label="启用">
              <el-switch v-model="authForm.githubEnabled" />
            </el-form-item>
            <el-form-item label="Client ID">
              <el-input v-model="authForm.githubClientId" />
            </el-form-item>
            <el-form-item label="Client Secret">
              <el-input
                v-model="authForm.githubClientSecret"
                type="password"
                show-password
                :disabled="authClear.github"
                :placeholder="
                  authSecretConfigured.github ? '已配置，留空不修改' : '未配置'
                "
              />
              <el-checkbox
                v-if="authSecretConfigured.github"
                v-model="authClear.github"
                class="secret-clear-check"
              >
                清空已配置密钥
              </el-checkbox>
            </el-form-item>
            <el-form-item label="账号年限">
              <el-input-number
                v-model="authForm.githubMinYears"
                :min="1"
                :max="20"
                controls-position="right"
              />
            </el-form-item>
            <div class="auth-preview-row">
              <el-tag :type="authPreview.github.type" effect="plain">
                {{ authPreview.github.label }}
              </el-tag>
              <span>{{ authPreview.github.description }}</span>
            </div>
          </div>

          <div class="auth-config-block">
            <h3>Google</h3>
            <el-form-item label="启用">
              <el-switch v-model="authForm.googleEnabled" />
            </el-form-item>
            <el-form-item label="Client ID">
              <el-input v-model="authForm.googleClientId" />
            </el-form-item>
            <el-form-item label="Client Secret">
              <el-input
                v-model="authForm.googleClientSecret"
                type="password"
                show-password
                :disabled="authClear.google"
                :placeholder="
                  authSecretConfigured.google ? '已配置，留空不修改' : '未配置'
                "
              />
              <el-checkbox
                v-if="authSecretConfigured.google"
                v-model="authClear.google"
                class="secret-clear-check"
              >
                清空已配置密钥
              </el-checkbox>
            </el-form-item>
            <div class="auth-preview-row">
              <el-tag :type="authPreview.google.type" effect="plain">
                {{ authPreview.google.label }}
              </el-tag>
              <span>{{ authPreview.google.description }}</span>
            </div>
          </div>

          <div class="auth-config-block">
            <h3>QQ</h3>
            <el-form-item label="启用">
              <el-switch v-model="authForm.qqEnabled" />
            </el-form-item>
            <el-form-item label="App ID">
              <el-input v-model="authForm.qqClientId" />
            </el-form-item>
            <el-form-item label="App Key">
              <el-input
                v-model="authForm.qqClientSecret"
                type="password"
                show-password
                :disabled="authClear.qq"
                :placeholder="
                  authSecretConfigured.qq ? '已配置，留空不修改' : '未配置'
                "
              />
              <el-checkbox
                v-if="authSecretConfigured.qq"
                v-model="authClear.qq"
                class="secret-clear-check"
              >
                清空已配置密钥
              </el-checkbox>
            </el-form-item>
            <div class="auth-preview-row">
              <el-tag :type="authPreview.qq.type" effect="plain">
                {{ authPreview.qq.label }}
              </el-tag>
              <span>{{ authPreview.qq.description }}</span>
            </div>
          </div>

          <div class="auth-config-block">
            <h3>站长账号</h3>
            <el-form-item label="用户名">
              <el-input v-model="authForm.webmasterUsername" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input
                v-model="authForm.webmasterPassword"
                type="password"
                show-password
                :disabled="authClear.webmaster"
                :placeholder="
                  authSecretConfigured.webmaster ? '已配置，留空不修改' : '未配置'
                "
              />
              <el-checkbox
                v-if="authSecretConfigured.webmaster"
                v-model="authClear.webmaster"
                class="secret-clear-check"
              >
                清空已配置密码
              </el-checkbox>
            </el-form-item>
            <div class="auth-preview-row">
              <el-tag :type="authPreview.webmaster.type" effect="plain">
                {{ authPreview.webmaster.label }}
              </el-tag>
              <span>{{ authPreview.webmaster.description }}</span>
            </div>
          </div>
        </div>
      </el-form>
    </section>

    <section class="governance-panel">
      <div class="panel-header">
        <div>
          <h2>站点设置</h2>
          <p>维护站点基础信息、SEO、上传和邮件等运行配置</p>
        </div>
        <el-button
          type="primary"
          :icon="useRenderIcon('ri/add-line')"
          :disabled="!canUpdate"
          @click="openCreateDialog"
        >
          新增配置
        </el-button>
      </div>

      <el-alert
        v-if="!canList"
        title="当前账号没有 governance:list_settings 权限"
        type="warning"
        show-icon
        :closable="false"
        class="permission-alert"
      />

      <el-form :inline="true" class="search-form">
        <el-form-item label="分组">
          <el-select v-model="query.group" class="w-36!" @change="loadSettings">
            <el-option
              v-for="item in groupOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="query.status"
            class="w-36!"
            @change="loadSettings"
          >
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :icon="useRenderIcon('ri/search-line')"
            :disabled="!canList"
            :loading="loading"
            @click="loadSettings"
          >
            查询
          </el-button>
          <el-button :icon="useRenderIcon('ep/refresh')" @click="resetQuery">
            重置
          </el-button>
        </el-form-item>
      </el-form>

      <pure-table
        row-key="key"
        adaptive
        :adaptiveConfig="{ offsetBottom: 156 }"
        align-whole="center"
        table-layout="auto"
        :loading="loading"
        :data="settings"
        :columns="columns"
        :pagination="{
          total: query.total,
          pageSize: query.pageSize,
          currentPage: query.currentPage,
          background: true
        }"
        :header-cell-style="{
          background: 'var(--el-fill-color-light)',
          color: 'var(--el-text-color-primary)'
        }"
        @page-size-change="onPageSizeChange"
        @page-current-change="onCurrentPageChange"
      >
        <template #group="{ row }">
          <el-tag effect="plain">{{ groupLabel(row.group) }}</el-tag>
        </template>
        <template #valueType="{ row }">
          {{ valueTypeLabel(valueTypeOf(row)) }}
        </template>
        <template #value="{ row }">
          {{ displayValue(row) }}
        </template>
        <template #status="{ row }">
          <el-tag :type="statusMeta(row.status).type">
            {{ statusMeta(row.status).label }}
          </el-tag>
        </template>
        <template #updatedAt="{ row }">
          {{ formatTime(updatedAt(row)) }}
        </template>
        <template #operation="{ row }">
          <el-button
            link
            type="primary"
            :disabled="!canUpdate"
            :icon="useRenderIcon('ri/edit-line')"
            @click="openEditDialog(row)"
          >
            编辑
          </el-button>
        </template>
      </pure-table>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="620px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="92px"
        class="setting-form"
      >
        <el-form-item label="配置键" prop="key">
          <el-input
            v-model="form.key"
            :disabled="dialogMode === 'edit'"
            placeholder="例如 site_name"
          />
        </el-form-item>
        <el-form-item label="分组" prop="group">
          <el-select v-model="form.group" class="w-full!">
            <el-option
              v-for="item in groupOptions.filter(item => item.value)"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="类型" prop="valueType">
          <el-select v-model="form.valueType" class="w-full!">
            <el-option
              v-for="item in valueTypeOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="配置值">
          <el-input
            v-model="form.value"
            type="textarea"
            :disabled="form.clearValue"
            :rows="
              form.valueType === 'json' || form.valueType === 'text' ? 6 : 3
            "
            :placeholder="
              form.valueType === 'password' && dialogMode === 'edit'
                ? '已配置，留空不修改'
                : '请输入配置值'
            "
          />
          <el-checkbox
            v-if="form.valueType === 'password' && dialogMode === 'edit'"
            v-model="form.clearValue"
            class="secret-clear-check"
          >
            清空已配置值
          </el-checkbox>
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="form.status">
            <el-radio-button :value="2">启用</el-radio-button>
            <el-radio-button :value="1">停用</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="说明">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            maxlength="240"
            show-word-limit
            placeholder="配置用途、取值说明或影响范围"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveSetting">
          保存
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.settings-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.governance-panel {
  width: 100%;
  padding: 16px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.panel-header {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 14px;
}

.panel-header h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.panel-header p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.permission-alert {
  margin-bottom: 8px;
}

.search-form {
  padding: 12px 0 4px;
}

.setting-form {
  padding-right: 10px;
}

.auth-config-form {
  margin-top: 4px;
}

.auth-config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.auth-config-block {
  padding: 14px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
}

.auth-config-block h3 {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.auth-preview-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.auth-preview-row {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  padding: 8px 10px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--el-text-color-secondary);
  background: var(--el-fill-color-lighter);
  border-radius: 6px;
}

.auth-preview-row .el-tag {
  flex: 0 0 auto;
}

.secret-clear-check {
  margin-top: 6px;
}

@media (width <= 768px) {
  .panel-header {
    flex-direction: column;
  }
}
</style>
