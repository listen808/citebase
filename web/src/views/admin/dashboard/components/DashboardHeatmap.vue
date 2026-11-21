<template>
  <!-- 热力图 -->
  <ApCard v-if="showSection('heatmap')">
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k111') }}</h2>
    </div>
    <div :style="heatmapContainerStyle">
      <div :style="heatmapGridStyle">
        <div
          v-for="(day, idx) in heatmapData"
          :key="idx"
          :style="getHeatmapCellStyle(day.intensity)"
          :title="$t('admin.k175', { d: day.date, n: day.count })"
        />
      </div>
      <div :style="heatmapLegendStyle">
        <span :style="legendTextStyle">{{ $t('admin.k112') }}</span>
        <div :style="legendGradientStyle" />
        <span :style="legendTextStyle">{{ $t('admin.k113') }}</span>
      </div>
    </div>
  </ApCard>

  <!-- 热门文档 TOP10 -->
  <ApCard v-if="showSection('docs')">
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k114') }}</h2>
    </div>
    <div v-if="hotDocuments.length > 0" :style="rankListStyle">
      <div
        v-for="(doc, idx) in hotDocuments.slice(0, 5)"
        :key="idx"
        :style="rankItemStyle(idx + 1)"
      >
        <span :style="rankNumberStyle(idx + 1)">{{ idx + 1 }}</span>
        <span :style="rankNameStyle" :title="doc.name">{{ doc.name }}</span>
        <span :style="rankCountStyle">{{ doc.count }} {{ $t('vAdminUi.k12') }}</span>
      </div>
    </div>
    <ApEmpty v-else icon="FileText" :title="$t('admin.k116')" />
  </ApCard>

  <!-- 活跃用户 TOP10 -->
  <ApCard v-if="showSection('users')">
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k115') }}</h2>
    </div>
    <div v-if="activeUsers.length > 0" :style="userGridStyle">
      <div
        v-for="(user, idx) in activeUsers.slice(0, 6)"
        :key="idx"
        :style="userItemStyle"
      >
        <ApAvatar
          :text="(user.given_name || user.username || '').slice(0, 1).toUpperCase()"
          size="small"
          :online="user.is_online"
        />
        <div :style="userInfoStyle">
          <div :style="userNameStyle" :title="user.given_name || user.username">{{ user.given_name || user.username }}</div>
          <div :style="userMetaStyle">{{ user.count }} {{ $t('vAdminUi.k13') }}</div>
        </div>
      </div>
    </div>
    <ApEmpty v-else icon="User" :title="$t('admin.k117')" />
  </ApCard>
</template>

<script setup lang="ts">
import { AppleSpace, AppleType, AppleFontWeight, cv, AppleRadius, AppleFont, subtleBg, AppleGridFloor } from '@/utils/core/apple-tokens';
import { monitorChartTitleStyle as cardTitleStyle } from '@/styles/layout/page-styles';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApAvatar from '@/components/apple/ApAvatar.vue';

export interface HeatmapDay {
  date: string;
  count: number;
  intensity: number;
}

export interface HotDocument {
  name: string;
  count: number;
}

export interface ActiveUser {
  username: string;
  given_name: string;
  count: number;
  is_online: boolean;
}

// section 拆分渲染：三块内容密度差异大，拆开才能与相邻卡片按行等高排布（'all' 保留旧的整体渲染）
const props = withDefaults(defineProps<{
  heatmapData: HeatmapDay[];
  hotDocuments: HotDocument[];
  activeUsers: ActiveUser[];
  section?: 'all' | 'heatmap' | 'docs' | 'users';
}>(), {
  section: 'all',
});

function showSection(section: 'heatmap' | 'docs' | 'users') {
  return props.section === 'all' || props.section === section;
}

const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
};

const heatmapContainerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  alignItems: 'center',
  width: '100%',
};

// 格子上限 44px：避免在宽面板里 aspect-ratio 随列宽无限放大、把卡片顶出两屏高
const heatmapGridStyle = {
  display: 'grid',
  gridTemplateColumns: 'repeat(7, minmax(0, 44px))',
  justifyContent: 'center',
  gap: AppleSpace.xxs,
};

function getHeatmapCellStyle(intensity: number) {
  // 用 color-mix 而不是 hexToRgba(cv(...))：后者拿到的是 var() 字符串，量不出 alpha，
  // 结果就是所有格子一样淡、在浅底上几乎看不见
  const pct = intensity <= 0 ? 0 : Math.max(22, Math.round(intensity * 92));
  return {
    aspectRatio: '1',
    borderRadius: AppleRadius.micro,
    background: pct ? subtleBg(cv('blue'), pct, 'blue-bg-subtle') : cv('bg-tertiary'),
    minHeight: '12px',
  };
}

const heatmapLegendStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  alignSelf: 'center',
};

const legendTextStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-quaternary'),
};

const legendGradientStyle = {
  width: '80px',
  height: AppleSpace.xs,
  borderRadius: AppleRadius.micro,
  background: `linear-gradient(to right, ${cv('blue-bg-subtle')}, ${cv('blue')})`,
};

const rankListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

function rankItemStyle(rank: number) {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.control,
    background: rank <= 3 ? cv('blue-bg-subtle') : 'transparent',
  };
}

function rankNumberStyle(rank: number) {
  // 金/银/铜：全部由 token 派生（铜 = orange 与 text-quaternary 调和），不留裸 hex
  const colors: Record<number, string> = {
    1: cv('orange'),
    2: cv('text-tertiary'),
    3: `color-mix(in srgb, ${cv('orange')} 55%, ${cv('text-quaternary')})`,
  };
  return {
    width: AppleSpace.md,
    height: AppleSpace.md,
    borderRadius: AppleRadius.circle,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: AppleType.legalMeta.size,
    fontWeight: String(AppleFontWeight.semibold),
    background: colors[rank] || cv('bg-tertiary'),
    color: rank <= 3 ? cv('on-accent') : cv('text-secondary'),
    flexShrink: '0',
  };
}

const rankNameStyle = {
  flex: '1',
  minWidth: '0',
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const rankCountStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  flexShrink: '0',
};

const userGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.tile})`,
  gap: AppleSpace.xs,
};

const userItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  padding: AppleSpace.xs,
  borderRadius: AppleRadius.control,
  background: cv('bg-secondary'),
};

const userInfoStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const userNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const userMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
};
</script>
