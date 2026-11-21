<template>
  <ApPageContainer
    :title="$t('vCommonUi.k13')"
    :max-width="AppleLayout.maxContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <ApCard size="small" :padding="cardPadding">
      <div :style="cardBodyStyle">
        <div :style="dividerStyle" />

        <div :style="infoSectionStyle">
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k1') }}</span>
            <span :style="valueStyle">{{ $t('chrome.tagline', { app: siteBrandName }) }}</span>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k2') }}</span>
            <a
              :href="APP_RELEASES_URL"
              target="_blank"
              rel="noopener noreferrer"
              :style="linkValueStyle"
            >{{ APP_VERSION }}</a>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k3') }}</span>
            <a
              :href="APP_AUTHOR_URL"
              target="_blank"
              rel="noopener noreferrer"
              :style="linkValueStyle"
                >{{ APP_AUTHOR_NAME }}</a>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k4') }}</span>
            <a
              :href="APP_REPO_URL"
              target="_blank"
              rel="noopener noreferrer"
              :style="linkValueStyle"
                >{{ APP_REPO_LABEL }}</a>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k5') }}</span>
            <span :style="valueStyle">Go + Gin + Vue 3 + TypeScript</span>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k6') }}</span>
            <span :style="valueStyle">Vue 3.5 + Vite 8.0 + Pinia 3.0</span>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k7') }}</span>
            <span :style="valueStyle">Monaco Editor</span>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k8') }}</span>
            <span :style="valueStyle">{{ $t('vCommonUi.k9') }}</span>
          </div>
          <div :style="infoItemStyle">
            <span :style="labelStyle">{{ $t('vCommonUi.k10') }}</span>
            <span :style="valueStyle">{{ $t('vCommonUi.k11') }}</span>
          </div>
        </div>

        <div :style="dividerStyle" />

        <p :style="copyrightStyle">
          {{ $t('vCommonUi.k18', { year: copyrightYear, app: APP_NAME }) }}
        </p>
        <p :style="footerTextStyle">
          <a
            :href="APP_LICENSE_URL"
            target="_blank"
            rel="noopener noreferrer"
            :style="licenseLinkStyle"
          >{{ $t('vCommonUi.k12') }}</a>
        </p>
      </div>
    </ApCard>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { cv, AppleSpace, AppleType, AppleLayout, AppleFontWeight, cssType } from '@/utils/core/apple-tokens';
import { APP_AUTHOR_NAME, APP_AUTHOR_URL, APP_LICENSE_URL, APP_NAME, APP_RELEASES_URL, APP_REPO_LABEL, APP_REPO_URL, APP_VERSION } from '@/utils/core/app-info';
import { siteBrandName } from '@/utils/core/site-brand';
import { dividerStyle, inlineHitArea } from '@/styles/layout/page-styles';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApCard from '@/components/apple/ApCard.vue';

const cardPadding = AppleSpace.lg;

// 版权行的 {app} 刻意传 APP_NAME 而不是 siteBrandName：这句署名说的是**这个软件项目**，
// 不是这个部署实例 —— 自托管的人把站点名改成"团队百科"，© 后面也不能跟着变成
// "团队百科 Contributors"。同页上面那句一句话定位是站点自己的话，所以它取 siteBrandName。
// 版权年份在这里取一次，不在模板里 new Date()：brand-closure 门禁要检查每个 {app} 键的
// 调用点都传了 app 参数，它按 t('key', ...) 的形状匹配，参数表里再套一层调用会读断。
const copyrightYear = new Date().getFullYear();

const cardBodyStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
};

const infoSectionStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};

// P2-14：原先 space-between 右对齐，长技术串（仓库 URL、技术栈）右边缘参差；
// 改成定宽两列、值左对齐，标签一列对齐后整块读起来是一条竖线。
const infoItemStyle = {
  display: 'grid',
  gridTemplateColumns: '160px 1fr',
  gap: AppleSpace.md,
  alignItems: 'center',
};

const labelStyle = {
  ...cssType(AppleType.microUI),
  color: cv('text-secondary'),
};

const valueStyle = {
  ...cssType(AppleType.bodyPrimary),
  color: cv('text-primary'),
  fontWeight: String(AppleFontWeight.medium),
};

const copyrightStyle = {
  ...cssType(AppleType.microUI),
  color: cv('text-tertiary'),
  textAlign: 'center' as const,
  margin: '0',
};

const footerTextStyle = {
  ...cssType(AppleType.microUI),
  color: cv('text-tertiary'),
  textAlign: 'center' as const,
  margin: `${AppleSpace.xs} 0 0`,
};

const linkValueStyle = {
  ...cssType(AppleType.bodyPrimary),
  color: cv('blue'),
  fontWeight: String(AppleFontWeight.medium),
  textDecoration: 'none',
  cursor: 'pointer',
};

const licenseLinkStyle = {
  ...cssType(AppleType.microUI),
  color: cv('blue'),
  textDecoration: 'none',
  cursor: 'pointer',
  // 12px 字号的行内链接，实测命中区 14px 高（check:apple A5 在 /about 上点名的就是它）。
  ...inlineHitArea,
};
</script>
