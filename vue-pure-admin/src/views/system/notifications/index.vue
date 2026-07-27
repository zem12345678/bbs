<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessageBox } from "element-plus";
import { message } from "@/utils/message";
import { hasPerms } from "@/utils/auth";
import {
  sendAdminSystemNotification,
  type SendSystemNotificationResult
} from "@/api/admin";

defineOptions({
  name: "SystemNotifications"
});

const recipientsInput = ref("");
const title = ref("");
const content = ref("");
const submitting = ref(false);
const pendingIdempotencyKey = ref("");
const canSend = computed(() => hasPerms("system:send_system_notification"));
const recipientIDs = computed(() => parseRecipientIDs(recipientsInput.value));
const recipientError = computed(() => {
  const raw = recipientsInput.value.trim();
  if (!raw) return "";
  const tokens = raw.split(/[\s,，]+/).filter(Boolean);
  if (tokens.length !== recipientIDs.value.length) {
    return "用户 ID 必须是正整数，且不能超过 int64 范围。";
  }
  if (recipientIDs.value.length > 1000) {
    return "单次最多投放给 1000 位用户。";
  }
  return "";
});

function parseRecipientIDs(value: string) {
  const seen = new Set<string>();
  for (const token of value.split(/[\s,，]+/)) {
    if (!token) continue;
    if (!/^\d+$/.test(token)) return [];
    const id = token.replace(/^0+/, "");
    if (
      !id ||
      id.length > 19 ||
      (id.length === 19 && id > "9223372036854775807")
    ) {
      return [];
    }
    seen.add(id);
  }
  return [...seen];
}

watch([recipientsInput, title, content], () => {
  pendingIdempotencyKey.value = "";
});

function newIdempotencyKey() {
  if (
    typeof crypto !== "undefined" &&
    typeof crypto.randomUUID === "function"
  ) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function idempotencyKeyForCurrentDraft() {
  if (!pendingIdempotencyKey.value) {
    pendingIdempotencyKey.value = newIdempotencyKey();
  }
  return pendingIdempotencyKey.value;
}

async function submit() {
  const normalizedTitle = title.value.trim();
  const normalizedContent = content.value.trim();
  if (!canSend.value) {
    message("没有系统通知投放权限", { type: "warning" });
    return;
  }
  if (recipientError.value || recipientIDs.value.length === 0) {
    message(recipientError.value || "请至少填写一个收件用户 ID", {
      type: "warning"
    });
    return;
  }
  if (!normalizedTitle || !normalizedContent) {
    message("请填写通知标题和内容", { type: "warning" });
    return;
  }

  try {
    await ElMessageBox.confirm(
      `将向 ${recipientIDs.value.length} 位用户投放这条系统通知，确认继续吗？`,
      "确认投放",
      { confirmButtonText: "投放", cancelButtonText: "取消", type: "warning" }
    );
  } catch {
    return;
  }

  submitting.value = true;
  try {
    const response = await sendAdminSystemNotification({
      recipient_ids: recipientIDs.value,
      title: normalizedTitle,
      content: normalizedContent,
      idempotency_key: idempotencyKeyForCurrentDraft()
    });
    const result = response.data as SendSystemNotificationResult;
    const delivered = result.delivered_count ?? result.deliveredCount ?? 0;
    message(`已投放给 ${delivered} 位用户`, { type: "success" });
    recipientsInput.value = "";
    title.value = "";
    content.value = "";
    pendingIdempotencyKey.value = "";
  } catch (error) {
    message(error instanceof Error ? error.message : "系统通知投放失败", {
      type: "error"
    });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <div class="system-notification-page">
    <el-alert
      title="首版仅支持按用户 ID 定向投放；不支持广播、标签筛选或异步批量扇出。"
      type="info"
      :closable="false"
      show-icon
      class="mb-4"
    />
    <el-card shadow="never" header="投放系统通知" class="max-w-3xl">
      <el-form label-position="top">
        <el-form-item label="收件用户 ID" :error="recipientError">
          <el-input
            v-model="recipientsInput"
            type="textarea"
            :rows="4"
            placeholder="多个 ID 可用逗号、中文逗号或换行分隔，例如：1001, 1002"
          />
          <div class="mt-2 text-xs text-[var(--el-text-color-secondary)]">
            已识别 {{ recipientIDs.length }} 位唯一用户，单次最多 1000 位。
          </div>
        </el-form-item>
        <el-form-item label="通知标题">
          <el-input
            v-model="title"
            maxlength="200"
            show-word-limit
            placeholder="请输入通知标题"
          />
        </el-form-item>
        <el-form-item label="通知内容">
          <el-input
            v-model="content"
            type="textarea"
            :rows="7"
            maxlength="5000"
            show-word-limit
            placeholder="请输入要发送给指定用户的通知内容"
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :loading="submitting"
            :disabled="!canSend"
            @click="submit"
          >
            {{ submitting ? "投放中..." : "确认投放" }}
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
