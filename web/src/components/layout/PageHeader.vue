<template>
  <div :style="containerStyle">
    <!-- Profile Mode -->
    <div v-if="mode === 'profile'" :style="profileStyle">
      <div :style="heroLeftStyle">
        <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.md }">
          <div v-if="avatarSrc" :style="profileAvatarRingStyle">
            <img :src="avatarSrc" :alt="title" :style="profileAvatarImgStyle" />
          </div>
          <div v-else :style="profileAvatarPlaceholderStyle">
            <ApIcon name="User" :size="20" :color="cv('text-secondary')" />
          </div>
          <div :style="profileInfoStyle">
            <h1 :style="heroTitleStyle" data-ap-page-title>{{ title }}</h1>
            <div v-if="subtitle" :style="heroSubStyle">{{ subtitle }}</div>
          </div>
        </div>
      </div>
      <div v-if="$slots.actions || actions" :style="heroRightStyle">
        <slot name="actions">
          <ApButton v-for="(btn, idx) in actions" :key="idx" v-bind="btn.props" @click="btn.onClick">
            {{ btn.label }}
          </ApButton>
        </slot>
      </div>
    </div>

    <!-- Greeting Mode -->
    <div v-else-if="mode === 'greeting'" :style="heroStyle">
      <div :style="heroLeftStyle">
        <h1 :style="heroTitleStyle" data-ap-page-title>{{ title }}</h1>
        <div v-if="quote" :style="heroQuoteStyle">{{ quote }}</div>
        <div :style="heroDateStyle">{{ subtitle }}</div>
      </div>
      <div v-if="$slots.actions || actions" :style="heroRightStyle">
        <slot name="actions">
          <ApButton v-for="(btn, idx) in actions" :key="idx" v-bind="btn.props" @click="btn.onClick">
            {{ btn.label }}
          </ApButton>
        </slot>
      </div>
    </div>

    <!-- Simple Mode -->
    <div v-else-if="mode === 'simple'" :style="simpleStyle">
      <div v-if="icon" :style="simpleIconStyle">
        <component :is="icon" :size="18" :color="cv('text-secondary')" />
      </div>
      <div :style="simpleInfoStyle">
        <h1 :style="heroTitleStyle" data-ap-page-title>{{ title }}</h1>
        <div v-if="subtitle" :style="heroSubStyle">{{ subtitle }}</div>
      </div>
      <div v-if="$slots.actions || actions" :style="heroRightStyle">
        <slot name="actions">
          <ApButton v-for="(btn, idx) in actions" :key="idx" v-bind="btn.props" @click="btn.onClick">
            {{ btn.label }}
          </ApButton>
        </slot>
      </div>
    </div>

    <!-- Gradient Mode -->
    <div v-else-if="mode === 'gradient'" :style="stackColumnStyle">
      <h2 :style="gradientTitleStyle">{{ title }}</h2>
      <div v-if="subtitle" :style="gradientDescStyle">{{ subtitle }}</div>
      <div v-if="$slots.actions || actions" :style="gradientActionsStyle">
        <slot name="actions">
          <ApButton v-for="(btn, idx) in actions" :key="idx" v-bind="btn.props" @click="btn.onClick">
            {{ btn.label }}
          </ApButton>
        </slot>
      </div>
    </div>

    <!-- Crud Mode -->
    <div v-else-if="mode === 'crud'" :style="crudStyle">
      <div :style="crudLeftStyle">
        <h1 :style="crudTitleStyle" data-ap-page-title>{{ title }}</h1>
        <div v-if="subtitle" :style="crudSubtitleStyle">{{ subtitle }}</div>
      </div>
      <div v-if="$slots.actions || actions" :style="crudRightStyle">
        <slot name="actions">
          <ApButton v-for="(btn, idx) in actions" :key="idx" v-bind="btn.props" @click="btn.onClick">
            {{ btn.label }}
          </ApButton>
        </slot>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue';
import { stackColumnStyle } from '@/styles/layout/page-styles';
import { tocListStyle } from '@/styles/component/doc-styles';
import { AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApButton from '@/components/apple/ApButton.vue';

interface ActionButton {
  label: string;
  props?: Record<string, unknown>;
  onClick?: () => void;
}

withDefaults(defineProps<{
  mode?: 'profile' | 'greeting' | 'simple' | 'gradient' | 'crud';
  title: string;
  subtitle?: string;
  quote?: string;
  avatarSrc?: string;
  icon?: Component;
  actions?: ActionButton[];
}>(), {
  mode: 'greeting',
});

const containerStyle = {
  marginBottom: AppleSpace.xl,
};

const heroRowStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.xl,
};

const heroStyle = {
  ...heroRowStyle,
};

const profileStyle = {
  ...heroRowStyle,
};

const simpleStyle = {
  ...heroRowStyle,
};

const crudStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.xl,
  marginBottom: AppleSpace.md,
  paddingBottom: AppleSpace.lg,
  borderBottom: `1px solid ${cv('border')}`,
};

const heroLeftStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  flex: '1',
  minWidth: '0',
};

const heroRightStyle = {
  flexShrink: '0',
};

const crudLeftStyle = tocListStyle;

const crudRightStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

const heroTitleStyle = {
  fontFamily: AppleType.pageTitle.font,
  fontSize: AppleType.pageTitle.size,
  fontWeight: String(AppleType.pageTitle.weight),
  lineHeight: String(AppleType.pageTitle.lineHeight),
  letterSpacing: AppleType.pageTitle.letterSpacing,
  color: cv('text-primary'),
  margin: '0',
};

const heroSubStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleType.microUI.weight),
  color: cv('text-secondary'),
  margin: '0',
};

const heroDateStyle = {
  fontFamily: AppleType.controlLabel.font,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleType.controlLabel.weight),
  color: cv('text-tertiary'),
  margin: '0',
  marginTop: AppleSpace.xs,
};

const heroQuoteStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  margin: '0',
  lineHeight: '1.5',
};;

const crudTitleStyle = {
  fontFamily: AppleType.pageTitle.font,
  fontSize: AppleType.pageTitle.size,
  fontWeight: String(AppleType.pageTitle.weight),
  color: cv('text-primary'),
  margin: '0',
};

const crudSubtitleStyle = {
  fontFamily: AppleType.controlLabel.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  margin: '0',
};

const profileAvatarRingStyle = {
  borderRadius: AppleRadius.panel,
  display: 'flex',
  lineHeight: '0',
  flexShrink: '0',
};

const profileAvatarImgStyle = {
  width: '40px',
  height: '40px',
  borderRadius: AppleRadius.panel,
};

const profileAvatarPlaceholderStyle = {
  width: '40px',
  height: '40px',
  borderRadius: AppleRadius.panel,
  background: cv('bg-tertiary'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: '0',
  transition: AppleTransition.smooth,
};

const profileInfoStyle = tocListStyle;

const simpleIconStyle = {
  width: '32px',
  height: '32px',
  borderRadius: AppleRadius.panel,
  background: cv('bg-tertiary'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: '0',
  transition: AppleTransition.smooth,
};

const simpleInfoStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  flex: '1',
};

const gradientTitleStyle = {
  fontFamily: AppleType.linkActionHeading.font,
  fontSize: AppleType.linkActionHeading.size,
  fontWeight: String(AppleType.linkActionHeading.weight),
  lineHeight: String(AppleType.linkActionHeading.lineHeight),
  color: cv('text-primary'),
  margin: '0',
};

const gradientDescStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
  margin: '0',
};

const gradientActionsStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
  marginTop: AppleSpace.sm,
};
</script>
