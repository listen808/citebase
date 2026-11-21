<template>
  <ApCard>
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k118') }}</h2>
    </div>
    <div :style="actionGridStyle">
      <div
        v-for="action in actions"
        :key="action.label"
        :style="actionItemStyle"
        @click="action.handler"
      >
        <!-- 这里四个动作传的是字符串图标名，`<component :is="'LayoutGrid'">` 渲染不出任何东西：
            图标一直静默缺席。改走 ApIcon（图标名 → 注册表组件）。 -->
        <ApIcon :name="action.icon" :size="16" :color="cv('text-secondary')" :style="{ flexShrink: '0' }" />
        <span :style="actionLabelStyle">{{ action.label }}</span>
      </div>
    </div>
  </ApCard>
</template>

<script setup lang="ts">
import { AppleSpace, AppleFont, AppleType, AppleRadius, cv, AppleTiming } from '@/utils/core/apple-tokens';
import { monitorChartTitleStyle as cardTitleStyle } from '@/styles/layout/page-styles';
import ApCard from '@/components/apple/ApCard.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import type { IconName } from '@/utils/icons';

export interface QuickAction {
  label: string;
  icon: IconName;
  handler: () => void;
}

defineProps<{
  actions: QuickAction[];
}>();

const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
};

const actionGridStyle = {
  display: 'grid',
  // 卡片在仪表盘网格里被拉到与「数据概览」等高，之前 4 个 16px 行挤在顶部，
  // 下面留 200px 空白。改成单列 + 行高等分：动作铺满卡片，每行都是可点的大靶子。
  gridTemplateColumns: '1fr',
  gridAutoRows: '1fr',
  gap: AppleSpace.xs,
  flex: '1',
};

const actionItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.xs}`,
  borderRadius: AppleRadius.panel,
  cursor: 'pointer',
  transition: `background ${AppleTiming.fast}`,
};

const actionLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  minWidth: 0,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};
</script>
