<template>
  <div :style="containerStyle" :role="type === 'error' ? 'alert' : 'status'" :aria-label="ariaLabel || $t('common.noData')">
    <slot name="icon">
      <component :is="iconComponent" :size="iconSize" :stroke-width="iconStrokeFor(iconSize)" :style="iconStyle" />
    </slot>
    <p :style="titleStyle">
      <slot>{{ title }}</slot>
    </p>
    <p v-if="description" :style="descStyle">{{ description }}</p>
    <ApButton
      v-if="actionText"
      variant="outline"
      size="small"
      :style="{ marginTop: AppleSpace.lg }"
      @click="handleAction"
    >
      {{ actionText }}
    </ApButton>
  </div>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue';
import { icons, type IconName } from '@/utils/icons';
import { cv, AppleFontWeight, AppleSpace, AppleType, AppleLineHeight, AppleIconSize, iconStrokeFor } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';

interface Props {
  type?: 'empty' | 'search' | 'error' | 'inbox' | 'list' | 'permission' | 'default';
  /**
   * 共享图标注册表里的名字，或组件本身。
   * 以前这里收裸 string 并查一份私有 iconMap（11 个键），名字写错不报错、
   * 静默回退成文件夹 —— 全站有 32 处因此画错图。现在交给 IconName 在编译期拦。
   */
  icon?: Component | IconName;
  title?: string;
  description?: string;
  actionText?: string;
}

const props = withDefaults(defineProps<Props>(), {
  type: 'empty',
  title: '',
});

const emit = defineEmits<{
  (e: 'action'): void;
}>();

const TYPE_ICON: Record<NonNullable<Props['type']>, IconName> = {
  // 缺省语义是"这里空了"，不是"这里有个文件夹"。原先 empty 与 default 分别落在
  // Folder 与 Inbox 两个图标上，于是全站十几个没传 icon 的表格空态都在断言
  // 自己是文件夹（"暂无数据"配文件夹图标）。统一收到中性的 Inbox。
  empty: 'Inbox',
  search: 'Search',
  error: 'AlertTriangle',
  inbox: 'Inbox',
  list: 'FileText',
  permission: 'Shield',
  default: 'Inbox',
};

const ariaLabel = computed(() => props.title);

const iconComponent = computed<Component>(() => {
  const given = props.icon;
  if (!given) return icons[TYPE_ICON[props.type]];
  return typeof given === 'string' ? icons[given] : given;
});

const isError = computed(() => props.type === 'error');

// 空态与错误态各走注册表的一档（40 = empty、48 = hero），不再在模板里写裸数字；
// 40/48 这两档的描边比 16 档细，必须由 iconStrokeFor 解出，否则大图标画成粗图标。
const iconSize = computed(() => (isError.value ? AppleIconSize.empty : AppleIconSize.empty));

function handleAction() {
  emit('action');
}

const containerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  justifyContent: 'center',
  padding: `${AppleSpace['3xl']} ${AppleSpace.xl}`,
};

const iconStyle = computed(() => ({
  color: isError.value ? cv('red') : cv('text-quaternary'),
  marginBottom: AppleSpace.md,
}));

const titleStyle = {
  color: cv('text-tertiary'),
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  margin: '0',
  textAlign: 'center' as const,
};

const descStyle = {
  color: cv('text-quaternary'),
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  margin: `${AppleSpace.sm} 0 0 0`,
  textAlign: 'center' as const,
  maxWidth: '320px',
  lineHeight: AppleLineHeight.loose,
};
</script>