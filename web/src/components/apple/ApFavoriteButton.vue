<template>
  <ApButton
    variant="ghost"
    :size="size"
    :icon-only="!labeled"
    :loading="busy"
    :aria-label="labeled ? undefined : label"
    :aria-pressed="active"
    :tooltip="labeled ? '' : label"
    @click="emit('toggle')"
  >
    <Star :size="iconSize" :style="starStyle" />
    <span v-if="labeled">{{ active ? $t('apUi1.k3') : $t('apUi1.favorite') }}</span>
  </ApButton>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { Star } from 'lucide-vue-next';
import { cv } from '@/utils/core/apple-tokens';
import type { AppleButtonSizeName } from '@/utils/core/apple-tokens';
import ApButton from './ApButton.vue';

/**
 * 收藏开关的唯一实现。全站原本有五处各写一份、四种视觉语言：
 * 首页空间卡 `Star` + `fill: orange`；收藏页卡片红色 `StarOff`；收藏页表格 `variant="danger"` 的
 * 「取消」文字按钮；文档工具栏 `fill="currentColor"`（星跟着文字色走，压根不是橙色）；
 * SQL 历史 `color: cv('warning')` 但没有 `fill`（收藏了仍是描边星，与未收藏只差一个颜色）。
 *
 * 约定：状态只用一颗星的**亮暗**表达——实心橙 = 已收藏，描边灰 = 未收藏；
 * 动作由点击本身发现，不换图标、不用 danger 色（取消收藏不是破坏性操作）。
 * 可访问名与气泡随状态一起变（首页那版即使已收藏仍念"收藏空间"，读屏听不出当前状态）。
 *
 * 位置：条目卡请放进 `ApEntityCard` 的 `#actions` 槽——那里与元信息同一行、右对齐，
 * 卡片与列表两种排布落在这同一份排布上。
 */
const props = withDefaults(defineProps<{
  /** 当前是否已收藏 */
  active?: boolean;
  /** 被收藏对象的名词，用于可访问名与提示（空间 / 文档 / 内容） */
  noun?: string;
  /** 带文字标签的形态（工具栏、表格行里用），图标与文字共同表达状态 */
  labeled?: boolean;
  /** 按钮密度；带文字的形态一般用 small，纯图标用 mini */
  size?: Extract<AppleButtonSizeName, 'mini' | 'small'>;
  /**
   * 星标光学尺寸，默认 14。允许宿主按同排图标调（文档工具栏那一排是 16），
   * 但状态语言不许调：实心橙 / 描边灰由本组件定死。
   */
  iconSize?: number;
  /** 请求进行中：按钮转 loading 并禁点，避免连点把状态打反 */
  busy?: boolean;
}>(), {
  active: false,
  noun: '内容',
  labeled: false,
  size: 'mini',
  /** 按钮里的图标全站一档 16（check:icons 的 I5）；这里以前默认 14，
   *  静态扫查不到它（是 prop 默认值不是模板字面量），是运行时 E5 分层后露出来的 */
  iconSize: 16,
  busy: false,
});

const emit = defineEmits<{ (e: 'toggle'): void }>();

const label = computed(() => `${props.active ? '取消收藏' : '收藏'}${props.noun}`);

const starStyle = computed(() => (props.active
  ? { fill: cv('orange'), color: cv('orange') }
  : { color: cv('text-quaternary') }));
</script>
