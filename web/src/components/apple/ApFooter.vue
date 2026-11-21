<template>
  <footer ref="footerRef" data-ap-shell="footer" :data-scroll-edge="scrollEdge" :style="footerStyle">
    <div :style="metaStyle">
      <span>{{ $t('chrome.tagline', { app: siteBrandName }) }}</span>
      <span :style="dotStyle" aria-hidden="true">·</span>
      <a
        :href="APP_RELEASES_URL"
        target="_blank"
        rel="noopener noreferrer"
        :style="linkStyle"
      >{{ APP_VERSION }}</a>
    </div>
    <nav :style="linksStyle" :aria-label="linkNavLabel || $t('chrome.helpNav')">
      <template v-for="link in APP_HELP_LINKS" :key="link.labelKey">
        <router-link
          v-if="link.to"
          :to="link.to"
          :style="linkStyle"
        >{{ $t(link.labelKey) }}</router-link>
        <a
          v-else
          :href="link.href"
          target="_blank"
          rel="noopener noreferrer"
          :style="linkStyle"
        >{{ $t(link.labelKey) }}</a>
      </template>
    </nav>
  </footer>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { AppleFont, AppleLayout, AppleSpace, AppleType, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { APP_HELP_LINKS, APP_RELEASES_URL, APP_VERSION } from '@/utils/core/app-info';
import { siteBrandName } from '@/utils/core/site-brand';
import { shellSurfaceKey, useScrollEdge, type ScrollEdgeState } from '@/composables/ui/useScrollEdge';
import { useShellGlassLift } from '@/composables/ui/useGlassAdaptive';

const props = withDefaults(defineProps<{
  /** 链接组的读屏名字；嵌在别处时传具体语境，避免一堆同名 landmark。不给时用语言包里的 chrome.helpNav */
  linkNavLabel?: string;
  /** 短文页（登录/分享）把页脚推到视口底部，而不是紧跟内容 */
  pinned?: boolean;
  /** 壳层浮层化（MainLayout）：叠在滚动区下沿之上，内容从它底下滚过去 —— 与顶栏同款几何。
      不传时保持文档流（BlankLayout 的 pinned 布局，身后没有内容）。 */
  overlay?: boolean;
}>(), {
  linkNavLabel: '',
  pinned: false,
  overlay: false,
});

// 浮层壳接滚动边缘探针：滚区不是它的祖先（负 margin 叠上去的兄弟），用 [data-ap-scroll] 指名。
const footerRef = ref<HTMLElement | null>(null);
const footerScroller = () => document.querySelector('[data-ap-scroll]');
// 探针只负责 data-scroll-edge 这个巡检锚点；**底色不跟它走**（见下）。
const scrollEdge = useScrollEdge(footerRef, { scroller: footerScroller });
/**
 * 底栏的材质档恒为 under。
 * 顶栏在 scrollTop=0 时身后只有环境层（内容从它下沿才开始），底栏不是这种几何：
 * 它是**叠在滚区下沿**上的，内容始终从它底下过 —— 首页第一屏底部那排统计卡就压在它后面
 * （用户截图里透出来的橙色图标）。按"滚没滚"判态，等于断言"没滚的时候它身后什么都没有"，
 * 而实测恰好相反：没滚的时候它压着的正是第一屏的内容，于是它停在 0.28 的静止档，
 * 把 72% 的内容透上来 —— 用户判词「穿透过度不自然」。
 * 所以底栏不参与那套切换：一直是加厚板 + 全档模糊，与顶栏滚起来之后的档位相同。
 */
const footerEdge = computed<ScrollEdgeState>(() => 'under');
// 加厚档的厚度仍走自适应（理由见 useGlassAdaptive）：深色下它直接由令牌定死，
// 浅色下它按身后内容在 [静止档, 令牌值] 之间取值 —— 与顶栏共用同一层。
useShellGlassLift(footerRef, { scroller: footerScroller, edge: footerEdge });

const footerStyle = computed(() => ({
  display: 'flex' as const,
  flexWrap: 'wrap' as const,
  alignItems: 'center' as const,
  justifyContent: 'space-between' as const,
  gap: AppleSpace.sm,
  // 高度由壳层标尺定死，不靠 padding 凑数：顶栏、页脚、侧栏用户行才能压在同一个数上
  height: AppleLayout.shellBarHeight,
  flexShrink: 0,
  // 左右留白跟顶栏同一套节奏（ApNavBar 用的就是这个 clamp）：
  // 页脚落在滚动容器里、在各页正文之外，跟着某个页面的 padding 反而对不齐。
  padding: `0 clamp(16px, 3vw, 24px)`,
  // 底栏是**停靠视口的第三条壳层横边**（R14 早就把它和顶栏、侧栏用户行钉在同一把尺上），
  // 所以它拿壳层档的材质：chrome 底色 + 玻璃发丝边 + 朝上的外投影 + 朝内容那条边的滑光亮段。
  // 2026-11-01 浮层化（overlay）：此前它独占 flex 列的一行，正文在它上方结束，
  // 玻璃底色只能透出环境层 —— 用户判词「底栏没穿透效果」。改成负 margin 叠进滚区下沿
  // （MainLayout 的 contentStyle 同时让出等高 padding-bottom），内容真的从它底下滚过，
  // 模糊才有东西可糊；滚区不是它的祖先，探针按 [data-ap-scroll] 锚点指名（同顶栏）。
  position: 'relative' as const,
  zIndex: String(AppleZIndex.sticky),
  marginTop: props.overlay
    ? `calc(-1 * ${AppleLayout.shellBarHeight})`
    : props.pinned ? 'auto' : '0',
  // 底栏压在内容上（不是压在环境层上），所以取加厚档 —— 同顶栏"内容压上来"之后的那个档。
  backgroundColor: cv(shellSurfaceKey(footerEdge.value, 'surface-chrome')),
  // 镜面高光铺在壳面上（非玻璃材质下令牌是 none）：随光源滑动的材料信号，与顶栏同一枚令牌
  backgroundImage: cv('glass-specular'),
  // 全档模糊常驻：底栏身后永远有内容（同 footerEdge 那条理由），不需要"静止轻档"那一支。
  backdropFilter: cv('glass-blur'),
  WebkitBackdropFilter: cv('glass-blur'),
  borderTop: `0.5px solid ${cv('glass-rim')}`,
  boxShadow: `${cv('glass-edge')}, ${cv('glass-foot-shadow')}`,
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-quaternary'),
}));

const metaStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
}));

const linksStyle = computed(() => ({
  display: 'flex',
  flexWrap: 'wrap' as const,
  alignItems: 'center',
  gap: AppleSpace.md,
}));

const dotStyle = computed(() => ({ color: cv('text-quaternary') }));

const linkStyle = computed(() => ({
  color: cv('text-tertiary'),
  textDecoration: 'none',
}));
</script>
