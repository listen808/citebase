<template>
  <div :style="sectionStyle">
    <div :style="headerStyle">
      <h3 :style="titleStyle">{{ $t('homeUi.k4') }}</h3>
      <router-link to="/activities" :style="linkStyle">
        {{ $t('homeUi.k2') }}
        <ApIcon name="ChevronRight" :size="14" />
      </router-link>
    </div>
    <div v-if="loading" :style="loadingStyle"><ApSkeleton variant="grid" :count="3" /></div>
    <div v-else-if="error && !docs.length" :style="emptyStyle">
      <ApIcon name="FileText" :size="AppleIconSize.listLead" :style="{ color: cv('text-quaternary') }" />
      <span>{{ $t('homeUi.k6') }}</span>
    </div>
    <div v-else-if="!docs.length" :style="emptyStyle">
      <ApIcon name="FileText" :size="AppleIconSize.listLead" :style="{ color: cv('text-quaternary') }" />
      <span>{{ $t('homeUi.k5') }}</span>
    </div>
    <template v-else>
      <div v-for="[group, items] in groupedDocs" :key="group" :style="groupWrapperStyle">
        <div :style="groupTitleStyle">{{ group }}</div>
        <div :style="gridStyle">
          <ApEntityCard
            v-for="doc in items"
            :key="doc.id"
            :title="doc.name"
            :subtitle="doc.space_name"
            :deleted="!!doc.isDeleted"
            :clickable="!doc.isDeleted"
            @click="doc.isDeleted ? undefined : $emit('navigate', doc)"
          >
            <div :style="entityMetaRowStyle()">
              <span v-if="doc.action" :style="actionBadgeStyle(doc)">
                {{ actionLabel(doc.action) }}
              </span>
              <span v-if="doc.creator_name && doc.creator_user_id" :style="creatorLinkStyle">
                <router-link :to="`/user/${doc.creator_user_id}`" :style="creatorLinkStyle" @click.stop>
                  {{ doc.creator_name }}
                </router-link>
              </span>
              <span :style="docTimeStyle">{{ formatTimeAgo(doc.update_time) }}</span>
            </div>
          </ApEntityCard>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { AppleIconSize, cv, onSolid, AppleType, AppleFontWeight, AppleSpace, AppleRadius, appleCardGrid } from '@/utils/core/apple-tokens';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import { loadingStyle, emptyStateStyle as emptyStyle, entityMetaRowStyle, layoutSectionHeaderTitle as titleStyle } from '@/styles/layout/page-styles';
import { formatTimeAgo } from '@/utils/format/format-time';
import { groupByTime } from '@/utils/format/time-group';
import { useI18n } from 'vue-i18n';

export interface RecentDoc {
  id: number;
  name: string;
  space_id: number;
  space_name: string;
  update_time: number;
  action?: string;
  creator_user_id?: number;
  creator_name?: string;
  isDeleted?: boolean;
}

const props = defineProps<{
  docs: RecentDoc[];
  loading: boolean;
  error?: boolean;
}>();

defineEmits<{
  (e: 'navigate', doc: RecentDoc): void;
}>();

// 必须排在宏之后：vue/define-macros-order 是 error 级，非宏语句插在宏前面直接打断 npm run ci。
const { t } = useI18n();

const groupedDocs = computed(() => {
  const groups = groupByTime(props.docs);
  return Array.from(groups.entries());
});

function actionLabel(action: string): string {
  // action 的取值来自后端（'create' / '1' 两套编码都要认），**键是数据、值才是显示文字**。
  const map: Record<string, string> = {
    create: t('common.create'),
    '1': t('common.create'),
    edit: t('common.modify'),
    '2': t('common.modify'),
    delete: t('common.delete'),
    '3': t('common.delete'),
    restore: t('common.restore'),
    '4': t('common.restore'),
  };
  return map[action] || action;
}

const sectionStyle = {
  marginBottom: AppleSpace.lg,
};

const headerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.md,
};

const linkStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.microUI.size,
  color: cv('blue'),
  textDecoration: 'none',
  cursor: 'pointer',
};

const groupWrapperStyle = {
  marginBottom: AppleSpace.md,
};

const groupTitleStyle = {
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-tertiary'),
  margin: `0 0 ${AppleSpace.md} 0`,
  letterSpacing: AppleType.controlLabel.letterSpacing,
};

// 与共享卡片网格同构：直接走工厂，别再抄一份字面量（克隆会被 check:style 记账）
const gridStyle = appleCardGrid('entity');

function actionBadgeStyle(doc: RecentDoc) {
  const action = doc.action || '';
  let bg = cv('text-quaternary');
  if (action === 'create' || action === '1') bg = cv('green');
  else if (action === 'edit' || action === '2') bg = cv('orange');
  else if (action === 'delete' || action === '3') bg = cv('red');
  else if (action === 'restore' || action === '4') bg = cv('blue');
  return {
    display: 'inline-flex',
    alignItems: 'center',
    padding: `${AppleSpace.xs} ${AppleSpace.xs}`,
    borderRadius: AppleRadius.pill,
    background: bg,
    color: onSolid(bg),
    fontSize: AppleType.microUI.size,
    fontWeight: String(AppleFontWeight.semibold),
    fontFamily: AppleType.microUI.font,
    lineHeight: '1',
  };
}

const creatorLinkStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('blue'),
  textDecoration: 'none',
  whiteSpace: 'nowrap' as const,
};

const docTimeStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  whiteSpace: 'nowrap' as const,
  marginLeft: 'auto' as const,
};
</script>
