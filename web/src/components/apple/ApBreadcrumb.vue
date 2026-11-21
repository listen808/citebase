<template>
  <nav :style="rootStyle" :aria-label="$t('common.breadcrumb')" data-ap="breadcrumb">
    <ol :style="listStyle">
      <li v-for="(item, index) in items" :key="index" :style="itemStyle">
        <span v-if="index > 0" :style="separatorStyle" aria-hidden="true">›</span>
        <!-- 末项 = 当前页：aria-current，且永远不是链接（自链接是 a11y 反模式） -->
        <span
          v-if="index === items.length - 1 && !alwaysInteractive"
          :style="activeTextStyle"
          :title="item.label"
          aria-current="page"
        >{{ item.label }}</span>
        <!-- 中间项但拿不到目的地（分组落地页就是当前页）：降级为纯文本，
             cursor:default 至少不会骗人说它可点 -->
        <span v-else-if="!item.path" :style="plainStyle" :title="item.label">{{ item.label }}</span>
        <!-- 真路径 → router-link：有 href 才进 Tab 序、才吃全局 a[href]:active 的按压反馈 -->
        <router-link
          v-else-if="isRealPath(item.path)"
          :to="item.path"
          :title="item.label"
          :style="linkStyle"
        >{{ item.label }}</router-link>
        <!-- 哨兵路径（__space__ / __doc__3）→ 只能 emit，由宿主决定去哪 -->
        <a v-else :style="linkStyle" :title="item.label" @click.prevent="emit('navigate', item, index)">{{ item.label }}</a>
      </li>
    </ol>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { AppleLayout, AppleSpace, AppleTiming } from '@/utils/core/apple-tokens';
import { useMobile } from '@/composables/ui/useMobile';
import {
  breadcrumbRowStyle,
  breadcrumbLinkStyle,
  breadcrumbCurrentStyle,
  breadcrumbSepStyle,
} from '@/styles/layout/page-styles';

/**
 * 全站面包屑只这一处实现（门禁 R8）。`ApPageContainer` 的 `:show-breadcrumb` 是它的薄壳，
 * `DocBreadcrumb` 是「返回键 + 本组件 + 编辑徽章」的外壳。
 * 之前容器另写了一份扁平 `<nav>`，同一件东西于是有两种 DOM：那份没有 `ol/li`、没有 `aria-current`，
 * 而本组件的链接没有 `href`（不进 Tab 序、不吃全局按压反馈）—— 两边各有一半是对的，这里合成一份。
 */
export interface BreadcrumbItem {
  label: string;
  /** 真路径走 router-link；`__xxx__` 哨兵走 navigate 事件；空 = 不可点（纯文本） */
  path?: string;
}

withDefaults(defineProps<{
  items: BreadcrumbItem[];
  /** 末项也渲染成可点（阅读区用它：末项就是当前文档，但宿主自己接管导航） */
  alwaysInteractive?: boolean;
}>(), {
  alwaysInteractive: false,
});

const emit = defineEmits<{
  (e: 'navigate', item: BreadcrumbItem, index: number): void;
}>();

const { isMobile } = useMobile();

const isRealPath = (path?: string) => !!path && path.startsWith('/');

// 竖向节奏取容器那一档（40 个页面在用；本组件原先是 padding md 0 + marginBottom md，
// 只有阅读区两处用）。宿主要别的节奏就用 :style 覆盖 —— 内联样式会合并，覆盖得掉。
const rootStyle = { ...breadcrumbRowStyle, marginBottom: AppleSpace.sm };

const listStyle = {
  listStyle: 'none',
  margin: '0',
  padding: '0',
  display: 'flex',
  alignItems: 'center',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.sm,
};

const itemStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  lineHeight: '1',
};

const linkStyle = computed(() => ({
  ...breadcrumbLinkStyle,
  // 行内面包屑只有 13px 高，手指点不中；触屏档补到触控最小目标（与 ApButton 同一条轴）
  minHeight: isMobile.value ? AppleLayout.touchTargetMin : undefined,
  transition: `color ${AppleTiming.smooth}`,
  cursor: 'pointer' as const,
}));

const plainStyle = { ...breadcrumbLinkStyle, cursor: 'default' };

const activeTextStyle = { ...breadcrumbCurrentStyle };

const separatorStyle = { ...breadcrumbSepStyle, userSelect: 'none' as const };
</script>
