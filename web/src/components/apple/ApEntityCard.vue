<template>
  <div
    v-bind="clickable ? keyboardClick(() => emit('click')) : { tabindex: '-1' }"
    :style="shellStyle"
    :class="clickable ? 'apple-focus-ring' : undefined"
    @mouseenter="hover = true"
    @mouseleave="hover = false; pressed = false"
    @pointerdown="onPointerDown"
    @pointerup="pressed = false"
    @pointercancel="pressed = false"
  >
    <div :style="headStyle">
      <span v-if="letter" :style="letterStyle">{{ letter }}</span>
      <span v-else-if="resolvedIcon" :style="iconBoxStyle">
        <component :is="resolvedIcon" :size="AppleIconSize.control" :style="{ color: iconColor }" />
      </span>
      <div :style="textWrapStyle">
        <div v-if="title" :style="titleStyle">{{ title }}</div>
        <div v-if="subtitle" :style="subtitleStyle">{{ subtitle }}</div>
      </div>
    </div>
    <div v-if="$slots.default || $slots.actions" :style="tailStyle">
      <div v-if="$slots.default" :style="metaStyle">
        <slot />
      </div>
      <div v-if="$slots.actions" :style="actionsStyle" @click.stop>
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, shallowRef, type Component } from 'vue';
import { AppleFontWeight, AppleIconSize, AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';
import { icons, type IconName } from '@/utils/icons';
import { keyboardClick } from '@/composables/ui/useKeyboardClick';
import { entityIconBoxStyle } from '@/styles/layout/page-styles';

/**
 * 实体条目（空间 / 文档 / 联系人 / 链接 / 知识库）的唯一实现。
 * 取代原 ApGradientCard——按 id 分配的彩虹渐变既随机、又不随主题变，
 * 「图标装在 56px 玻璃方块里」是纯装饰，删掉后信息密度反而更高。
 *
 * layout 只有两档：card 是网格里的卡，row 是列表里的行。同一个条目在两类页面上
 * 走同一份标题 / 副标题 / 元信息 / 操作位实现，「卡片↔列表」切换因此只是排布变化，
 * 不再是两套版面。站点请配 AppleGridFloor.entity 档的网格（或 appleCardGrid()）。
 */
const props = withDefaults(defineProps<{
  title?: string;
  subtitle?: string;
  /** 首字/序号 chip（联系人用姓名首字，排序态用序号） */
  letter?: string;
  /** 图标 chip：共享注册表的名字或组件本身 */
  icon?: Component | IconName;
  /** 图标着色，同时决定 chip 底色浅染 */
  iconColor?: string;
  /** card = 网格卡，row = 列表行 */
  layout?: 'card' | 'row';
  clickable?: boolean;
  /** 实体已删除：标题划掉并压暗，保留原文案（状态要用文字表达，不是换一个图标底） */
  deleted?: boolean;
}>(), {
  layout: 'card',
  iconColor: undefined,
  clickable: true,
  deleted: false,
});

const emit = defineEmits<{ (e: 'click'): void }>();

const hover = shallowRef(false);
/** 按下态：反馈必须发生在 pointer-down（§1），光有 hover 变化不算 */
const pressed = shallowRef(false);

function onPointerDown() {
  if (props.clickable) pressed.value = true;
}

const resolvedIcon = computed(() => {
  const given = props.icon;
  if (!given) return undefined;
  return typeof given === 'string' ? icons[given] : given;
});

const surfaceStyle = {
  background: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  boxShadow: cv('shadow-card'),
  // 实体卡是列表里的流水面（首页实测一屏 18 层模糊全来自它），不付模糊 —— 红线 ≤3。
  transition: AppleTransition.smooth,
  cursor: props.clickable ? 'pointer' : 'default',
};

const shellStyle = computed(() => ({
  // 卡片必须是自身绝对定位的包含块：条目里的 hover 操作区用 position:absolute 贴在右上角，
  // 少了这一句它会相对页面定位，飞出卡片外（联系人页实测：编辑/删除压在字母索引上）。
  position: 'relative' as const,
  ...(props.layout === 'row'
    ? {
        display: 'flex',
        flexDirection: 'row' as const,
        alignItems: 'center',
        gap: AppleSpace.sm,
        padding: `${AppleSpace.xs} ${AppleSpace.md}`,
      }
    : {
        display: 'flex',
        flexDirection: 'column' as const,
        // 等高网格里把元信息压到卡片底部，同一行的卡片元信息才在同一基线上
        justifyContent: 'space-between',
        gap: AppleSpace.sm,
        padding: AppleSpace.md,
      }),
  ...surfaceStyle,
  border: `1px solid ${hover.value && props.clickable ? cv('border') : cv('border-light')}`,
  // 按下比悬停多一分"被按住"的实体收缩：与 hover 态可区分，
  // 指针离开/抬起即回弹（pointerleave 与 pointerup 都清）
  ...(props.clickable && pressed.value ? { transform: 'scale(0.98)' } : {}),
}));

const headStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  minWidth: 0,
  ...(props.layout === 'row' ? { flex: '1' } : {}),
}));

// 元信息与操作位排在同一行（原先 card 布局这里是 {}，两个 div 竖排），
// 于是同一个收藏开关在收藏页落在元信息下面一行、在首页却跟在元信息后面，
// 看着像两个不同位置的控件。卡片与列表现在共用这一份排布。
const tailStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.sm,
  ...(props.layout === 'row' ? { flexShrink: 0 } : {}),
}));

const actionsStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, flexShrink: 0 };

const iconBoxStyle = computed(() => entityIconBoxStyle(props.iconColor ?? cv('blue')));

const letterStyle = computed(() => ({
  width: AppleSpace.xl,
  height: AppleSpace.xl,
  flexShrink: 0,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: AppleRadius.panel,
  background: cv('bg-tertiary'),
  color: cv('text-secondary'),
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
}));

const textWrapStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
  gap: '2px',
  minWidth: 0,
}));

const titleStyle = computed(() => ({
  fontFamily: AppleType.sectionTitle.font,
  fontSize: AppleType.sectionTitle.size,
  fontWeight: String(AppleType.sectionTitle.weight),
  lineHeight: AppleType.sectionTitle.lineHeight,
  color: props.deleted ? cv('text-quaternary') : cv('text-primary'),
  textDecoration: props.deleted ? 'line-through' : 'none',
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
  whiteSpace: 'nowrap' as const,
}));

const subtitleStyle = computed(() => ({
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
  whiteSpace: 'nowrap' as const,
}));

const metaStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
  // 元信息可以收缩，否则长标题/长元信息会把右侧操作位（收藏星）挤出卡片
  minWidth: 0,
  // 收缩解决不了"整块内容没有断行点"：作者名是一个链接，里面一个空格都没有，
  // flex-wrap 也无从可 wrap，长名字直接画到隔壁卡片上（抗压巡检实测越界 305px）。
  // 槽位里是各页面塞进来的任意内容，组件够不着给每个 <a> 加省略号，只能自己裁。
  overflow: 'hidden' as const,
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
}));
</script>
