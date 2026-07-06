<script setup lang="ts">
type TagType = "primary" | "success" | "warning" | "info" | "danger";

type DetailField = {
  label: string;
  value?: string | number | null;
  tags?: string[];
  status?: {
    label: string;
    type?: TagType;
  };
  imageUrl?: string;
};

type DetailSection = {
  title: string;
  content?: string | number | null;
};

defineOptions({
  name: "GovernanceDetailDrawer"
});

defineProps<{
  modelValue: boolean;
  title: string;
  fields: DetailField[];
  sections?: DetailSection[];
}>();

const emit = defineEmits<{
  (event: "update:modelValue", value: boolean): void;
}>();

function valueText(value?: string | number | null) {
  if (value === undefined || value === null || value === "") return "-";
  return String(value);
}
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    :title="title"
    size="min(720px, 92vw)"
    class="governance-detail-drawer"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <div class="detail-body">
      <el-descriptions border :column="1" class="detail-descriptions">
        <el-descriptions-item
          v-for="field in fields"
          :key="field.label"
          :label="field.label"
        >
          <el-image
            v-if="field.imageUrl"
            :src="field.imageUrl"
            fit="cover"
            class="detail-image"
          />
          <el-tag v-else-if="field.status" :type="field.status.type">
            {{ field.status.label }}
          </el-tag>
          <div v-else-if="field.tags" class="detail-tags">
            <el-tag
              v-for="tag in field.tags"
              :key="tag"
              size="small"
              effect="plain"
            >
              {{ tag }}
            </el-tag>
            <span v-if="field.tags.length === 0" class="muted-text">-</span>
          </div>
          <span v-else>{{ valueText(field.value) }}</span>
        </el-descriptions-item>
      </el-descriptions>

      <section
        v-for="section in sections || []"
        :key="section.title"
        class="detail-section"
      >
        <h3>{{ section.title }}</h3>
        <div class="detail-block">{{ valueText(section.content) }}</div>
      </section>
    </div>
  </el-drawer>
</template>

<style scoped>
.detail-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.detail-image {
  width: 100%;
  max-height: 220px;
  overflow: hidden;
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
}

.detail-section h3 {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.detail-block {
  max-height: 48vh;
  padding: 10px 12px;
  overflow: auto;
  line-height: 1.65;
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
  word-break: break-word;
  background: var(--el-fill-color-lighter);
  border: 1px solid var(--el-border-color-light);
  border-radius: 4px;
}

.muted-text {
  color: var(--el-text-color-placeholder);
}

:deep(.el-descriptions__label) {
  width: 112px;
}
</style>
