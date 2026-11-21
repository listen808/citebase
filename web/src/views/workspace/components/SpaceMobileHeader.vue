<template>
  <div :style="mobileHeaderStyle">
    <!-- 名字要和顶栏那个"打开菜单"分开：同一页两个同名按钮，读屏念出来完全一样，
         而它们开的是两个不同的东西（全局导航 vs 本页文档树） -->
    <ApButton icon-only size="mini" variant="ghost" :aria-label="$t('ws.k16')" @click="$emit('toggle-menu')">
      <ApIcon name="Menu" :size="16" />
    </ApButton>
    <span :style="mobileTitleStyle">{{ title || siteBrandName }}</span>
  </div>
</template>

<script setup lang="ts">
import { siteBrandName } from '@/utils/core/site-brand';
import { AppleSpace, AppleType, AppleLayout, cv } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';

interface Props {
  title?: string;
}

withDefaults(defineProps<Props>(), {
  // 不给默认值：默认就是"当前站点名"，由 site-brand 提供（模板里 title || siteBrandName）。
  // 以前这里写死 'Citebase'，是全站第七份产品名。
  title: '',
});

defineEmits<{
  'toggle-menu': [];
}>();

const mobileHeaderStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  padding: `0 ${AppleSpace.md}`, height: AppleLayout.touchTargetMin,
  borderBottom: `1px solid ${cv('border-light')}`, background: cv('surface-panel'), flexShrink: 0,
};

const mobileTitleStyle = {
  fontFamily: AppleType.bodyEmphasis.font, fontSize: AppleType.subhead.size,
  fontWeight: String(AppleType.bodyEmphasis.weight), color: cv('text-primary'),
  flex: '1', textAlign: 'center' as const, overflow: 'hidden',
  textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
};
</script>
