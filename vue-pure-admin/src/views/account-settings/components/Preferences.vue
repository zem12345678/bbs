<script setup lang="ts">
import { onMounted, ref } from "vue";
import { message } from "@/utils/message";
import { deviceDetection } from "@pureadmin/utils";

defineOptions({
  name: "Preferences"
});

type PreferenceItem = {
  key: string;
  title: string;
  illustrate: string;
  checked: boolean;
};

const storageKey = "bbs-admin-account-preferences";
const list = ref<PreferenceItem[]>([
  {
    key: "security",
    title: "安全提醒",
    illustrate: "登录异常、权限变化等安全事件在当前设备显示提醒",
    checked: true
  },
  {
    key: "system",
    title: "系统消息",
    illustrate: "系统公告、版本更新和维护消息在当前设备显示提醒",
    checked: true
  },
  {
    key: "todo",
    title: "待办任务",
    illustrate: "审核、发货、售后等待办事项在当前设备显示提醒",
    checked: true
  }
]);

function readSavedPreferences() {
  if (typeof localStorage === "undefined") return;
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || "{}");
    list.value = list.value.map(item => ({
      ...item,
      checked:
        typeof saved[item.key] === "boolean" ? saved[item.key] : item.checked
    }));
  } catch {
    localStorage.removeItem(storageKey);
  }
}

function savePreferences() {
  if (typeof localStorage === "undefined") return;
  const saved = Object.fromEntries(
    list.value.map(item => [item.key, item.checked])
  );
  localStorage.setItem(storageKey, JSON.stringify(saved));
}

function onChange(_val: boolean | string | number, item: PreferenceItem) {
  savePreferences();
  message(`${item.title}偏好已保存到当前设备`, { type: "success" });
}

onMounted(readSavedPreferences);
</script>

<template>
  <div :class="['min-w-45', deviceDetection() ? 'max-w-full' : 'max-w-[70%]']">
    <h3 class="my-8!">偏好设置</h3>
    <el-alert
      title="当前偏好保存在本机浏览器，不影响其他管理员账号或设备。"
      type="info"
      show-icon
      :closable="false"
      class="mb-4!"
    />
    <div v-for="(item, index) in list" :key="index">
      <div class="flex items-center">
        <div class="flex-1">
          <p>{{ item.title }}</p>
          <p class="wp-4">
            <el-text class="mx-1" type="info">
              {{ item.illustrate }}
            </el-text>
          </p>
        </div>
        <el-switch
          v-model="item.checked"
          inline-prompt
          active-text="是"
          inactive-text="否"
          @change="val => onChange(val, item)"
        />
      </div>
      <el-divider />
    </div>
  </div>
</template>

<style lang="scss" scoped>
.el-divider--horizontal {
  border-top: 0.1px var(--el-border-color) var(--el-border-style);
}
</style>
