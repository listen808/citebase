<template>
  <div :style="containerStyle">
    <div :style="innerStyle">
      <!-- 面包屑只有一处实现（ApBreadcrumb，门禁 R8）；这里只负责"从路由派生出哪几级"。
           原先这份 nav 是页内手写的扁平结构：没有 ol/li、没有 aria-current，
           而 ApBreadcrumb 的链接又没有 href —— 两边各对一半，合成一份。 -->
      <ApBreadcrumb v-if="showBreadcrumb" :items="crumbItems" />

      <slot name="header">
        <div v-if="title || $slots.title" :style="headerStyle">
          <div :style="headerLeftStyle">
            <ApButton v-if="showBack" variant="ghost" size="mini" icon-only :aria-label="$t('common.back')" @click="handleBack">
              <ArrowLeft :size="16" />
            </ApButton>
            <div>
              <h1 :style="titleStyle" data-ap-page-title>
                <slot name="title">{{ title }}</slot>
              </h1>
              <p v-if="subtitle || $slots.subtitle" :style="subtitleStyle">
                <slot name="subtitle">{{ subtitle }}</slot>
              </p>
            </div>
          </div>
          <div v-if="$slots.actions" :style="headerActionsStyle">
            <slot name="actions" />
          </div>
        </div>
      </slot>

      <div :style="contentStyle">
        <slot name="content">
          <slot />
        </slot>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { ArrowLeft } from 'lucide-vue-next';
import { AppleFontWeight, AppleLayout, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import { navGroupLabel, navGroupTarget } from '@/router/navigation';
import ApButton from '@/components/apple/ApButton.vue';
import ApBreadcrumb from '@/components/apple/ApBreadcrumb.vue';
import type { BreadcrumbItem } from '@/components/apple/ApBreadcrumb.vue';

const props = defineProps<{
  title?: string;
  subtitle?: string;
  showBack?: boolean;
  backTo?: string;
  maxWidth?: string;
  showBreadcrumb?: boolean;
  cardContent?: boolean;
  breadcrumbParent?: { label: string; path: string };
  noScroll?: boolean;
  compact?: boolean;
}>();

const router = useRouter();
const route = useRoute();
const { t } = useI18n();

// 分组父级：来自路由 meta.group（侧边栏同名分区的语义父子）。
// 面包屑层级由此与侧边栏保持一致：首页 › {分组} › {[父级] ›} {当前页}。
// 与显式 breadcrumbParent 叠加而非互斥——真父子页面（角色管理›角色用户、
// AI 设置›系统概览）在分组层级之上继续延伸。
const groupCrumb = computed(() => {
  const group = route.meta.group;
  if (typeof group !== 'string' || !group) return null;
  return { label: navGroupLabel(group), path: navGroupTarget(group) };
});

// 层级在这里派生，渲染交给 ApBreadcrumb —— 分组落地页就是当前页时 path 给空，
// 由它降级成纯文本（自链接是 a11y 反模式）。
const crumbItems = computed<BreadcrumbItem[]>(() => {
  const items: BreadcrumbItem[] = [{ label: t('nav.home'), path: '/' }];
  const g = groupCrumb.value;
  if (g) items.push({ label: g.label, path: g.path && g.path !== route.path ? g.path : '' });
  if (props.breadcrumbParent) items.push({ label: props.breadcrumbParent.label, path: props.breadcrumbParent.path });
  if (props.title) items.push({ label: props.title, path: '' });
  return items;
});

function handleBack() {
  if (props.backTo) {
    router.push(props.backTo);
  } else {
    router.back();
  }
}

const containerStyle = computed(() => {
  if (props.noScroll) {
    return {
      flex: '1',
      display: 'flex',
      flexDirection: 'column' as const,
      minHeight: '0',
      overflow: 'hidden' as const,
      background: cv('surface-canvas'),
    };
  }
  return {
    // P2-5：basis 0（flex:1）会让容器恒定等于滚动视口高度，页面内容全部落在容器外溢出的
    // scrollable overflow 里 —— 滚动容器自己的 padding-bottom 因此不进 scrollHeight，
    // 末行卡片永远被浮层页脚压住。basis:auto 让容器高度=内容高度，短页再靠 grow 撑满。
    flex: '1 0 auto',
    background: cv('surface-canvas'),
    minHeight: '0',
  };
});

const innerStyle = computed(() => {
  const base: Record<string, string> = {
    width: '100%',
    padding: props.compact ? `${AppleSpace.sm} ${AppleSpace.md}` : 'clamp(16px, 3vw, 32px) clamp(20px, 4vw, 40px)',
  };
  // 全宽（padding 归零、不限宽）只由 maxWidth='none' 或「no-scroll 且未声明宽度」触发；
  // 显式传了 maxWidth 的 no-scroll 页面（如 AI 对话）必须拿到限宽，否则 prop 被静默丢弃
  const fullBleed = props.maxWidth === 'none' || (props.noScroll && !props.maxWidth);
  if (fullBleed) {
    base.padding = '0';
  } else {
    base.maxWidth = props.maxWidth || AppleLayout.maxWideContentWidth;
    base.margin = '0 auto';
  }
  if (props.noScroll) {
    base.display = 'flex';
    base.flexDirection = 'column';
    base.flex = '1';
    base.minHeight = '0';
    base.overflow = 'hidden';
  }
  return base;
});

const headerStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  gap: AppleSpace.md,
  marginBottom: AppleSpace.lg,
  // 窄屏上操作区（flexShrink:0）会把标题挤到 min-content —— 中文逐字换行，
  // 症状是移动端标题竖排。允许换行后操作区整组落到标题下方。
  flexWrap: 'wrap' as const,
};

const headerLeftStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: AppleSpace.sm,
};

// 页头 h1 读 pageTitle：原先读 linkActionHeading，于是"页头"和"文档自己的标题"共用一个尺寸档，
// 层级只剩间距一根轴。一个概念一个名字，这里也照办。
const titleStyle = computed(() => ({
  fontFamily: AppleType.pageTitle.font,
  fontSize: AppleType.pageTitle.size,
  fontWeight: String(AppleType.pageTitle.weight),
  lineHeight: String(AppleType.pageTitle.lineHeight),
  color: cv('text-primary'),
  margin: '0',
  letterSpacing: AppleType.pageTitle.letterSpacing,
}));

const subtitleStyle = {
  fontFamily: AppleType.subhead.font,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.regular),
  color: cv('text-secondary'),
  margin: `${AppleSpace.xs} 0 0 0`,
};

const headerActionsStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
  flexShrink: 0,
};

const contentStyle = computed(() => {
  if (props.noScroll) {
    return {
      flex: '1',
      minHeight: '0',
      display: 'flex',
      flexDirection: 'column' as const,
      overflow: 'hidden' as const,
    };
  }
  return { minHeight: '0' };
});
</script>
