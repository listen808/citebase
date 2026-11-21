<template>
  <button
    ref="buttonRef"
    :style="buttonStyle"
    :disabled="disabled || loading"
    :type="type"
    :aria-label="ariaLabel"
    :aria-describedby="tipVisible ? tipBubbleId : undefined"
    :aria-disabled="disabled || loading"
    :aria-busy="loading"
    :class="buttonClass"
    @pointerenter="hover = true; showTip()"
    @pointerleave="hover = false; hideTip()"
    @mousedown="active = true"
    @mouseup="active = false"
    @blur="active = false; focused = false; hideTip()"
    @focus="handleFocus"
  >
    <span v-if="loading" :style="spinnerStyle" aria-hidden="true"></span>
    <slot v-if="!loading" />
    <span v-else :style="loadingTextStyle">{{ $t('apUi1.k1') }}</span>
    <!-- 图标按钮的提示只有这一套实现：气泡被 Teleport 挪到 body，
         所以按钮本身不多包一层 wrapper，:style 透传仍然落在 <button> 上。
         Teleport 本身也要按需挂：全站 2600+ 个按钮都各挂一个空 Teleport 实例是白付的钱。 -->
    <Teleport v-if="tooltip" to="body">
      <transition name="tooltip">
        <span
          v-if="tipVisible"
          :id="tipBubbleId"
          ref="tipBubbleRef"
          :style="tipStyle"
          role="tooltip"
        >{{ tooltip }}</span>
      </transition>
    </Teleport>
  </button>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef } from 'vue';
import { AppleAnimation, AppleGlass, AppleInteractionState, AppleTransition, AppleButtonSize, AppleLayout, AppleRadius, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import type { AppleButtonSizeName } from '@/utils/core/apple-tokens';
import { useMobile } from '@/composables/ui/useMobile';
import { tooltipBubbleStyle, useTooltip, type TooltipPlacement } from '@/composables/shared/useTooltip';

const props = withDefaults(defineProps<{
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline' | 'tint' | 'overlay';
  size?: AppleButtonSizeName;
  disabled?: boolean;
  loading?: boolean;
  type?: 'button' | 'submit' | 'reset';
  ariaLabel?: string;
  /** 悬停/键盘聚焦时显示的气泡提示；与原生 title 二选一，站点内只认这一套 */
  tooltip?: string;
  tooltipPlacement?: TooltipPlacement;
  /** 使用胶囊圆角（完全圆角） */
  capsule?: boolean;
  /** 仅图标模式 — 紧凑正方形按钮，无 padding 调整 */
  iconOnly?: boolean;
}>(), {
  variant: 'primary',
  size: 'medium',
  disabled: false,
  loading: false,
  type: 'button',
  tooltip: '',
  tooltipPlacement: 'top',
  capsule: false,
  iconOnly: false,
});

const hover = shallowRef(false);
const active = shallowRef(false);
const focused = shallowRef(false);
const { isMobile } = useMobile();

/** 焦点环只给键盘：鼠标点击/触屏轻点同样会触发 focus，原先无条件画蓝环，
 *  SPA 导航后焦点仍留在刚点过的按钮上 —— 侧栏里就是用户报的「选中残留」蓝圈。
 *  触屏的悬停残留同理走 pointerleave（抬手即清，mouse* 事件在触屏上只进不出）。 */
function handleFocus(e: FocusEvent) {
  focused.value = (e.target as HTMLElement | null)?.matches(':focus-visible') ?? true;
  showTip();
}

const buttonRef = ref<HTMLElement | null>(null);
const tipBubbleRef = ref<HTMLElement | null>(null);
const {
  visible: tipVisible, ready: tipReady, position: tipPosition, bubbleId: tipBubbleId, show: showTip, hide: hideTip,
} = useTooltip(buttonRef, tipBubbleRef, {
  content: () => props.tooltip,
  placement: () => props.tooltipPlacement,
});
const tipStyle = computed(() => tooltipBubbleStyle(tipReady.value, tipPosition.value));

const buttonClass = computed(() => [
  'apple-focus-ring',
  props.capsule ? 'ap-button--capsule' : '',
]);

const buttonStyle = computed(() => {
  const size = props.size;
  const variant = props.variant;
  const isCapsule = props.capsule;

  const sizeToken = AppleButtonSize[size as keyof typeof AppleButtonSize] || AppleButtonSize.medium;

  const base: Record<string, string> = {
    fontFamily: AppleType.bodyEmphasis.font,
    fontWeight: String(AppleType.bodyEmphasis.weight),
    lineHeight: '1',
    border: 'none',
    cursor: props.disabled ? AppleInteractionState.disabledCursor : 'pointer',
    transition: AppleTransition.interactive,
    opacity: props.disabled ? AppleInteractionState.disabledOpacity : '1',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: props.iconOnly ? '0' : AppleSpace.sm,
    fontSize: sizeToken.fontSize,
    padding: props.iconOnly
      ? (size === 'mini' ? AppleSpace.xs : AppleSpace.sm)
      : (isCapsule ? `0 ${sizeToken.padding.match(/\d+px\s+(\d+px)/)?.[1] || '20px'}` : sizeToken.padding),
    // 图标按钮悬停浅染是圆形（macOS 工具栏语言）：方形薄纱压着圆形色块/图标，
    // 两种几何同框就是用户报的「不和谐」。
    // 2026-10-26：iconOnly 的圆角从 circle(50%) 改为 pill(980px) —— 50% 在按钮被
    // width:100% 拉宽时渲染成**椭圆**（实测侧栏收起按钮 223×32 拉成巨大椭圆灰纱）；
    // 980px 对方形按钮渲染出完全相同的圆形，被拉宽时退化为胶囊而不是椭圆。
    borderRadius: isCapsule || props.iconOnly ? AppleRadius.pill : AppleRadius.control,
    height: props.iconOnly ? (size === 'mini' ? '28px' : sizeToken.height) : sizeToken.height,
    WebkitTapHighlightColor: 'transparent',
    WebkitUserSelect: 'none',
    userSelect: 'none',
    // 工具栏是 flex，按钮默认可收缩；一旦被压到内容宽度以下，中文就逐字断行
    // （实测：「新建用户」「新建空间」「上传」「全部/图片/附件」全断成两行）。
    whiteSpace: 'nowrap',
    flexShrink: '0',
    // 44px 触控底线只对 large 与触屏成立。原先 medium 图标按钮被 minHeight 抬到 44，
    // 而同一行的 small 文本按钮是 32 —— 一排控件三种高度就是这么来的。
    // 桌面按尺寸令牌走（与输入框/下拉同高）。触屏这条原先只补图标按钮，于是窄视口下
    // 同一行里文字按钮 32、图标按钮 44 又分成两档（390 宽实测 12 处）；触控目标是
    // "每个可点的东西"都要够大，不是只有图标的要够大，所以高度对全部按钮一起补。
    minHeight: size === 'large' || isMobile.value ? AppleLayout.touchTargetMin : sizeToken.height,
    minWidth: size === 'large' || (props.iconOnly && isMobile.value) ? AppleLayout.touchTargetMin : sizeToken.height,
    ...(props.iconOnly ? { width: size === 'mini' ? '28px' : sizeToken.height } : {}),
  };

  // 悬停与点击的流畅过渡
  if (!props.disabled) {
    if (active.value) {
      base.transform = 'scale(0.97)';
    } else if (hover.value) {
      base.transform = 'scale(1.02)';
    } else {
      base.transform = 'scale(1)';
    }
  }

  // 焦点环
  if (focused.value && !props.disabled) {
    base.boxShadow = cv('blue-focus-ring');
  } else if (hover.value && !props.disabled && variant !== 'ghost') {
    // ghost 悬停不配阴影：它常贴在玻璃头部/侧栏上，柔影在渐变底上会糊成一圈光晕（用户报「不美观」），
    // 浅染本身已足够表达可点。其余变体维持原有浮起语言。
    base.boxShadow = cv('shadow-level2');
  }

  switch (variant) {
    case 'primary':
      base.background = cv('blue');
      // 实心强调色上的前景必须由色值本身决定（on-blue = contrastInkOn(blue)），
      // 不能用恒白的 on-accent：个性化批加了强调色档位之后，深色外观那一列压不住白字（实测 2.3~2.6:1）。
      // 教条本来就是这么写的（theme-system.md：实心色块前景用 onSolid，浅染上用 textOnTint），
      // 这一处是历史遗留的捷径 —— 内置强调色恰好都够深，所以从没露出来过。
      base.color= cv('on-blue');
      // 主按钮同色柔光（焦点环优先，不被覆盖）
      if (!(focused.value && !props.disabled)) {
        base.boxShadow = `0 0.125rem 0.625rem color-mix(in srgb, ${cv('blue')} 30%, transparent)`;
      }
      if (active.value && !props.disabled) {
        base.filter = 'brightness(0.88)';
      } else if (hover.value && !props.disabled) {
        base.filter = 'brightness(1.08)';
      }
      break;
    case 'secondary':
      base.background = cv('bg-secondary');
      base.color = cv('text-primary');
      // 控件玻璃档：高光 + 发丝边 + 内棱线，与图标钮同一套（原先这里只有 border-light）
      Object.assign(base, AppleGlass.control);
      if (active.value && !props.disabled) {
        base.background = cv('bg-hover');
        base.filter = 'brightness(0.92)';
      } else if (hover.value && !props.disabled) {
        base.background = cv('bg-hover');
      }
      break;
    case 'ghost':
      if (props.iconOnly) {
        // 图标钮的控件档玻璃（2026-10-26 用户拍板）：透明底上的裸图标读不出「按钮」——
        // 顶栏设置/铃铛/主题、弹窗关闭 ×、编辑工具栏、目录排序全是这批（用户截图逐一点名）。
        // 语言与 secondary 同一档（AppleGlass.control）：底色走次级面，不再借浮层档的
        // surface-float 与 dropdown 阴影 —— 控件比浮层薄一档，借浮层的底就把层级读反了。
        // **idle 不付逐颗 blur**（2026-10-26 全面普查修正）：图标钮是稠密流水面，权限树全展开
        // 实测同屏 66 层实时模糊（红线 16）。surface-float 0.78 的底色下 blur 与否肉眼几乎不可分，
        // 玻璃观感由高光/斜面/描边承担；悬停浅染出现时再付（走下方 ghost 悬停分支）。
        base.background = cv('bg-secondary');
        Object.assign(base, AppleGlass.control);
        base.color = cv('text-secondary');
        if (active.value && !props.disabled) {
          base.background = cv('bg-hover');
          base.color = cv('text-primary');
          base.filter = 'brightness(0.92)';
        } else if (hover.value && !props.disabled) {
          base.background = cv('bg-hover');
          base.color = cv('text-primary');
        }
      } else {
        base.background = 'transparent';
        base.color = cv('text-secondary');
        if (active.value && !props.disabled) {
          base.background = cv('hover-veil');
        } else if (hover.value && !props.disabled) {
          base.background = cv('hover-veil');
          base.color = cv('text-primary');
        }
      }
      break;
    case 'overlay':
      // 压在图片或深色遮罩上的控件：语言与 ghost 一样（无底、hover 才给一块浅染），
      // 但浅染与文字色都必须由"恒浅色"派生 —— text-secondary 在深色底上看不见，
      // 而主题的 bg-hover 在 light 下是近白，白图标压上去就糊成一整块白方块。
      base.background = 'transparent';
      base.color = cv('on-accent');
      if (active.value && !props.disabled) {
        base.background = cv('veil-on-dark-active');
      } else if (hover.value && !props.disabled) {
        base.background = cv('veil-on-dark');
      }
      break;
    case 'outline':
      base.background = 'transparent';
      base.color = cv('blue');
      base.border = `1px solid ${cv('blue')}`;
      if (active.value && !props.disabled) {
        base.background = cv('blue-light');
      } else if (hover.value && !props.disabled) {
        base.background = cv('blue-light');
      }
      break;
    case 'tint':
      // 选中态导航行 / 活动筛选 chip：与顶栏当前项同一套语言（同色浅染 + 压暗后的同色文字），
      // 不用实心色块——实心块会让侧栏在当前项上出现一整块强调色，与内容区语言打架。
      base.background = cv('blue-selection');
      base.color = cv('blue-text');
      if (hover.value && !props.disabled) {
        base.background = cv('blue-selection-strong');
      }
      if (active.value && !props.disabled) {
        base.filter = 'brightness(0.96)';
      }
      break;
    case 'danger':
      base.background = cv('red');
      // 同 primary：实心色块的前景跟着色值走（on-red 由 readableForegroundVars 按该主题的 red 算）
      base.color= cv('on-red');
      if (active.value && !props.disabled) {
        base.filter = 'brightness(0.88)';
      } else if (hover.value && !props.disabled) {
        base.filter = 'brightness(1.08)';
      }
      break;
  }

  // 液态玻璃的控件档（2026-10-26 用户拍板恢复：按钮也是液态玻璃）。
  // 白名单而不是黑名单 —— 黑名单要靠判断"这个 background 计算出来是不是实心"，
  // 而 base 上只有令牌名（cv('blue-light') 是实心浅蓝、cv('blue-selection') 是 18% 浅染），
  // 拿字符串猜必然猜错。判据是"底本身半透明"：
  //   secondary（bg-secondary 0.74~0.90）/ tint（blue-selection 0.14~0.18）恒有半透底 → 恒付；
  //   ghost 图标钮 idle 不付（稠密流水面，权限树实测同屏 66 层；玻璃观感走 AppleGlass.dropdown
  //   的高光/斜面/描边），悬停浅染出现时付；ghost 文本钮 / overlay 的 idle 是透明底，
  //   透明元素上付 blur 会把身后内容糊出一块"洞"（那不是玻璃，是失焦），只在悬停浅染出现时付；
  //   primary / danger / outline 实心 → 模糊零视觉效果，白付一次 GPU，不付。
  // 半径取内容档（--apple-blur-panel），比壳层浅一档：按钮比顶栏轻，这是层级。
  // 非玻璃材质下该令牌是 none，自动退化为纯半透明底，不用这里再分支。
  // 2026-10-26 曾以「流水面控件数量不可控（首页实测 24 层）」收掉一轮；用户拍板按钮要玻璃，
  // 同屏层数红线随之改为运行时实测基线后钉住（check-glass-runtime.mjs 的 MAX_BLUR_LAYERS）。
  // 2026-11-01 收口：判据从「底本身半透明」改成「底够实到读得出一块板」。
  // tint（blue-selection 0.14~0.18）与 overlay 悬停（veil 浅染）都是**浅染**，
  // 在浅染上付 blur 等于把身后内容糊出一个洞 —— 那不是玻璃，是失焦（这条判据本文件早就写过，
  // 只是当时没管住 tint）。secondary 0.74~0.90 / ghost 悬停 bg-hover 才是一块板。
  if (variant === 'secondary' ||
      (variant === 'ghost' && (hover.value || active.value))) {
    base.backdropFilter = cv('blur-panel');
    base.WebkitBackdropFilter = cv('blur-panel');
  }

  return base;
});

const spinnerStyle = {
  display: 'inline-block',
  width: AppleInteractionState.spinnerSizeSmall,
  height: AppleInteractionState.spinnerSizeSmall,
  border: `${AppleInteractionState.spinnerBorderWidthSmall} solid currentColor`,
  borderTopColor: 'transparent',
  borderRadius: AppleRadius.circle,
  animation: `${AppleAnimation.spin} ${AppleInteractionState.spinnerDuration} linear infinite`,
  marginRight: AppleSpace.xs,
};

const loadingTextStyle = {
  opacity: '0.8',
};
</script>

<style scoped>
/* 菜单/窄容器里的图标曾被 flex 压缩到 0 宽：按钮内容 white-space:nowrap + flex-shrink:0
   不可缩，SVG（flex-shrink 默认 1、可缩）先被压没 —— 实测 More 菜单「Image management /
   Attachment manager」两项图标整体消失。图标承载语义，禁止收缩。
   图标来自默认 slot（属父组件作用域），须用 :slotted 命中。 */
.apple-focus-ring :slotted(svg) {
  flex-shrink: 0;
}
</style>

