<template>
  <ApCard>
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k126') }}</h2>
      <div :style="pollingIndicatorStyle">
        <span :style="pollingDotStyle" />
        <span :style="pollingTextStyle">{{ $t('admin.k127') }}</span>
      </div>
    </div>
    <div :style="monitorGridStyle">
      <div v-for="item in serverItems" :key="item.label" :style="monitorItemStyle">
        <div :style="monitorRowStyle">
          <div :style="monitorLabelStyle">{{ item.label }}</div>
          <div :style="monitorValueStyle">{{ item.display }}</div>
        </div>
        <ApProgress :value="item.value" :label="item.label" :tone="loadTone(item)" />
      </div>
    </div>
    <div v-if="errorLogCount !== undefined" :style="errorLogBannerStyle">
      <router-link to="/logs" :style="errorLogLinkStyle">
        {{ $t('admin.k174', { n: errorLogCount }) }}
      </router-link>
    </div>
    <div v-if="recentErrors.length > 0" :style="recentErrorsSectionStyle">
      <div :style="recentErrorsTitleStyle">{{ $t('admin.k128') }}</div>
      <div :style="recentErrorsListStyle">
        <div
          v-for="(err, idx) in recentErrors.slice(0, 3)"
          :key="idx"
          :style="recentErrorItemStyle"
        >
          <span :style="recentErrorLevelStyle(err.level)">{{ err.level }}</span>
          <!-- 错误摘要被 ellipsis 截断（768 宽实测一条少 79px）：不给 title 就等于
               把唯一能定位问题的字符串丢了 -->
          <span :style="recentErrorMessageStyle" :title="err.message">{{ err.message }}</span>
        </div>
      </div>
    </div>
  </ApCard>
</template>

<script setup lang="ts">
import { subtleBg, AppleSpace, AppleRadius, AppleFont, AppleType, AppleFontWeight, cv } from '@/utils/core/apple-tokens';
import { toneForLoad, type ApTone } from '@/utils/core/progress-tone';
import { inlineHitArea } from '@/styles/layout/page-styles';
import ApCard from '@/components/apple/ApCard.vue';
import type { RecentError } from '@/api/modules/system/log';

export interface ServerItem {
  label: string;
  value: number;
  display: string;
  /** 是哪一路资源必须由调用方说清楚：以前靠在上文案里找"内存/磁盘"，
   *  界面切成英文（label 变成 Memory usage）就永远匹配不上，阈值静默回落到默认档。 */
  kind: 'cpu' | 'memory' | 'disk';
}

defineProps<{
  serverItems: ServerItem[];
  errorLogCount?: number;
  recentErrors: RecentError[];
}>();

const cardHeaderStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: AppleSpace.md,
};

const cardTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: '0',
};

const pollingIndicatorStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const pollingDotStyle = {
  width: AppleSpace.sm,
  height: AppleSpace.sm,
  background: cv('green'),
  borderRadius: AppleRadius.circle,
  display: 'inline-block',
};

const pollingTextStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const monitorGridStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};

const monitorItemStyle = {
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.control,
  background: cv('bg-secondary'),
};

const monitorRowStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: AppleSpace.xs,
};

const monitorLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const monitorValueStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

// 每种资源的安全线本来就不同（磁盘留 10% 余量是常态，CPU 到 80% 已经很吃紧），
// 所以阈值留在这里；但"越线之后是什么颜色"全站只有一套答案，交给 toneForLoad。
const LOAD_THRESHOLDS: Record<ServerItem['kind'], [warn: number, critical: number]> = {
  cpu: [60, 80],
  memory: [70, 85],
  disk: [80, 90],
};

function loadTone(item: ServerItem): ApTone {
  const [warn, critical] = LOAD_THRESHOLDS[item.kind];
  return toneForLoad(item.value, warn, critical);
}

const errorLogBannerStyle = {
  marginTop: AppleSpace.md,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  background: subtleBg(cv('red'), 15, 'red-bg-subtle'),
  borderRadius: AppleRadius.control,
  border: `1px solid color-mix(in srgb, ${cv('red')} 30%, transparent)`,
};

const errorLogLinkStyle = {
  color: cv('red'),
  fontWeight: String(AppleFontWeight.medium),
  textDecoration: 'none',
  fontSize: AppleType.microUI.size,
  // 12px 字号的行内链接，实测命中区只有 14px 高（check:apple A5 在 /dashboard 上点名的就是它）。
  ...inlineHitArea,
};

const recentErrorsSectionStyle = {
  marginTop: AppleSpace.md,
  padding: AppleSpace.sm,
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.control,
};

const recentErrorsTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xs,
};

const recentErrorsListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const recentErrorItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

function recentErrorLevelStyle(level: string) {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.legalMeta.size,
    fontWeight: String(AppleFontWeight.semibold),
    color: level === 'error' ? cv('red') : cv('orange'),
    padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.micro,
    background: level === 'error' ? subtleBg(cv('red'), 15, 'red-bg-subtle') : subtleBg(cv('orange'), 15, 'orange-bg-subtle'),
    flexShrink: '0',
  };
}

const recentErrorMessageStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  flex: '1',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};
</script>
