<template>
  <div :style="containerStyle">
    <!-- Space Header Card -->
    <ApCard :style="{ marginBottom: AppleSpace.sm }">
      <div :style="headerStyle">
        <div :style="headerLeftRowStyle">
          <div :style="avatarStyle">{{ (spaceName || 'S').charAt(0).toUpperCase() }}</div>
          <div :style="headerLeftInfoStyle">
            <h1 :style="titleStyle" data-ap-object-title>{{ spaceName || $t('fallback.k59') }}</h1>
            <div :style="metaRowStyle">
              <span :style="visitLevelBadgeStyle">{{ visitLevelLabel }}</span>
              <span v-if="creatorName" :style="metaTextStyle">{{ t('common.createdBy', { name: creatorName }) }}</span>
            </div>
          </div>
        </div>
        <div :style="headerActionsStyle">
          <ApFavoriteButton :active="spaceCollected" :noun="$t('lastAttrsUi.k9')" @toggle="$emit('space-collect')" />
          <ApButton size="mini" icon-only variant="ghost" :aria-label="$t('ws.k20')" @click="$emit('space-export')">
            <ApIcon name="Download" :size="16" />
          </ApButton>
          <ApButton size="mini" icon-only variant="ghost" :aria-label="$t('ws.k26')" @click="$emit('open-members')">
            <ApIcon name="Users" :size="16" />
          </ApButton>
        </div>
      </div>
      <p v-if="description" :style="descStyle">{{ description }}</p>
    </ApCard>

    <!-- Stats Cards Row：全站统一 ApStatCard compact -->
    <div :style="statsRowStyle">
      <ApStatCard
        v-for="stat in statCards"
        :key="stat.label"
        size="compact"
        :label="stat.label"
        :value="stat.value"
        :icon="stat.icon"
        :color="stat.color"
      />
    </div>

    <!-- Quick Actions + Document Health -->
    <div :style="twoColStyle">
      <!-- Quick Actions -->
      <ApCard>
        <div :style="sectionHeaderStyle">
          <ApIcon name="Zap" :size="14" :style="{ color: cv('orange') }" />
          <span :style="sectionTitleStyle">{{ $t('ws.k17') }}</span>
        </div>
        <div :style="actionsGridStyle">
          <ApButton
            v-if="canEditInSpace"
            variant="ghost"
            size="small"
            :style="{ width: '100%', justifyContent: 'flex-start' }"
            @click="$emit('create-doc')"
          >
            <ApIcon name="FilePlus" :size="16" />
            <span>{{ $t('ws.k18') }}</span>
          </ApButton>
          <ApButton
            v-if="canEditInSpace"
            variant="ghost"
            size="small"
            :style="{ width: '100%', justifyContent: 'flex-start' }"
            @click="$emit('import-docs')"
          >
            <ApIcon name="Upload" :size="16" />
            <span>{{ $t('ws.k19') }}</span>
          </ApButton>
          <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="$emit('space-export')">
            <ApIcon name="Download" :size="16" />
            <span>{{ $t('ws.k20') }}</span>
          </ApButton>
          <ApButton variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="$emit('open-members')">
            <ApIcon name="Users" :size="16" />
            <span>{{ $t('ws.k21') }}</span>
          </ApButton>
          <ApButton v-if="aiStore.enabled" variant="ghost" size="small" :style="{ width: '100%', justifyContent: 'flex-start' }" @click="$emit('view-graph')">
            <ApIcon name="Network" :size="16" />
            <span>{{ $t('ws.k22') }}</span>
          </ApButton>
        </div>
      </ApCard>

      <!-- Document Health -->
      <ApCard>
        <div :style="sectionHeaderStyle">
          <ApIcon name="Activity" :size="14" :style="{ color: cv('green') }" />
          <span :style="sectionTitleStyle">{{ $t('ws.k23') }}</span>
        </div>
        <div v-if="recentDocs.length" :style="docListStyle">
          <div v-for="doc in recentDocs" :key="doc.id" :style="docItemStyle" @click="$emit('navigate-to-doc', doc.id)">
            <ApIcon name="FileText" :size="12" :style="{ color: cv('text-tertiary'), flexShrink: 0 }" />
            <span :style="docNameStyle">{{ doc.name }}</span>
            <span :style="docTimeStyle">{{ doc.update_time ? formatTimeAgo(doc.update_time) : '' }}</span>
          </div>
        </div>
        <ApEmpty v-else icon="Folder" :title="$t('ws.k25')" :description="$t('vWorkspaceUi.k1')" />
      </ApCard>
    </div>

    <!-- Tags & Info -->
    <ApCard v-if="tags.length" :style="{ marginTop: AppleSpace.sm }">
      <div :style="sectionHeaderStyle">
        <ApIcon name="Tag" :size="14" :style="{ color: cv('orange') }" />
        <span :style="sectionTitleStyle">{{ $t('ws.k24') }}</span>
      </div>
      <div :style="tagsWrapStyle">
        <span v-for="tag in tags" :key="tag" :style="tagStyle">{{ tag }}</span>
      </div>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import { AppleFontWeight, AppleLineHeight, AppleSpace, AppleType, AppleRadius, AppleLayout, cv, AppleGridFloor, appleCardGrid } from '@/utils/core/apple-tokens';
import type { IconName } from '@/utils/icons';
import ApCard from '@/components/apple/ApCard.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFavoriteButton from '@/components/apple/ApFavoriteButton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import { useDocumentStore } from '@/stores/workspace/document';
import { useAIStore } from '@/stores/ai/ai';
import { formatTimeAgo } from '@/utils/format/format-time';
import type { Document } from '@/types/models';

interface RecentDoc {
  id: number;
  name: string;
  update_time: number;
}

interface Props {
  spaceName?: string;
  creatorName?: string;
  description?: string;
  visitLevel?: string;
  spaceCollected?: boolean;
  canEditInSpace?: boolean;
  docCount?: number;
  memberCount?: number;
  tags?: string;
  spaceId?: number;
}

const props = withDefaults(defineProps<Props>(), {
  spaceName: '我的空间',
  creatorName: '',
  description: '',
  visitLevel: 'private',
  spaceCollected: false,
  canEditInSpace: false,
  docCount: 0,
  memberCount: 0,
  tags: '',
  spaceId: 0,
});

defineEmits<{
  'space-collect': [];
  'space-export': [];
  'open-members': [];
  'create-doc': [];
  'import-docs': [];
  'navigate-to-doc': [docId: number];
  'view-graph': [];
}>();
const { t } = useI18n();

const docStore = useDocumentStore();
const aiStore = useAIStore();

// P1-3 修复：复用 docStore.tree，避免重复请求 getDocumentTree
// 进入空间时 loadRootDocuments 已加载根节点，子节点按需懒加载
// recentDocs 会随树加载进度自动响应式更新
const recentDocs = computed<RecentDoc[]>(() => {
  const result: RecentDoc[] = [];
  const flatten = (nodes: Document[]) => {
    for (const n of nodes) {
      if (n.type === 1) {
        result.push({ id: n.id, name: n.name, update_time: n.update_time || 0 });
      }
      if (n.children && n.children.length) flatten(n.children);
    }
  };
  flatten(docStore.tree);
  result.sort((a, b) => b.update_time - a.update_time);
  return result.slice(0, 5);
});

const tags = computed(() => {
  if (!props.tags) return [];
  return props.tags.split(',').map(t => t.trim()).filter(Boolean);
});

const visitLevelLabel = computed(() => {
  const level = props.visitLevel;
  if (level === '0' || level === 'public') return '公开';
  if (level === '1' || level === 'login') return '登录可见';
  return '私有';
});

const visitLevelBadgeStyle = computed(() => {
  const level = props.visitLevel;
  let bg = cv('red');
  if (level === '0' || level === 'public') bg = cv('blue');
  else if (level === '1' || level === 'login') bg = cv('orange');
  return {
    display: 'inline-flex', alignItems: 'center',
    padding: `${AppleSpace.xxs} ${AppleSpace.sm}`, borderRadius: AppleRadius.control,
    background: bg, color: cv('on-accent'),
    // 11px 不在小字档上；实心徽章取 ApStatusBadge 小档的 10px
    fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold),
    whiteSpace: 'nowrap' as const,
  };
});

const statCards = computed<{ icon: IconName; value: number; label: string; color: string }[]>(() => [
  { icon: 'Folder', value: props.docCount, label: t('ws.k27'), color: cv('blue') },
  { icon: 'Users', value: props.memberCount, label: t('ws.k28'), color: cv('green') },
  { icon: 'Hash', value: tags.value.length, label: t('ws.k29'), color: cv('purple') },
]);

const containerStyle = {
  maxWidth: AppleLayout.maxContentWidth,
  margin: '0 auto',
};

const headerStyle = {
  display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between',
  gap: AppleSpace.md,
};

const headerLeftRowStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace['12px'], flex: '1', minWidth: '0',
};

const headerLeftInfoStyle = {
  minWidth: '0',
};

const avatarStyle = {
  width: '36px', height: '36px', borderRadius: AppleRadius.panel,
  background: cv('blue'),
  color: cv('on-blue'), display: 'flex', alignItems: 'center', justifyContent: 'center',
  fontSize: AppleType.subhead.size, fontWeight: String(AppleFontWeight.semibold), flexShrink: '0',
};

const titleStyle = {
  fontFamily: AppleType.pageTitle.font,
  fontSize: AppleType.pageTitle.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'), margin: '0',
};

const metaRowStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm, marginTop: AppleSpace.xs,
};

const metaTextStyle = {
  fontSize: AppleType.microUI.size, color: cv('text-tertiary'),
};

const descStyle = {
  marginTop: AppleSpace.sm, fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'), lineHeight: AppleLineHeight.loose,
};

const headerActionsStyle = {
  display: 'flex', gap: AppleSpace.xs, flexShrink: 0,
};

const statsRowStyle = {
  display: 'grid', gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.stat})`, gap: AppleSpace.sm,
  marginBottom: AppleSpace.sm,
};

const twoColStyle = appleCardGrid('entity', AppleSpace.sm);

const sectionHeaderStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace['6px'], marginBottom: AppleSpace.sm,
};

const sectionTitleStyle = {
  fontSize: AppleType.microUIBold.size, fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};

const actionsGridStyle = {
  display: 'grid', gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.tile})`, gap: AppleSpace.xs,
};

const docListStyle = {
  display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs,
};

const docItemStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  padding: `${AppleSpace['6px']} ${AppleSpace.sm}`, borderRadius: AppleRadius.small,
  cursor: 'pointer', transition: 'background var(--apple-dur-fast)',
};

const docNameStyle = {
  flex: 1, fontSize: AppleType.microUIBold.size, color: cv('text-primary'),
  whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis' as const,
};

const docTimeStyle = {
  fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, color: cv('text-tertiary'), flexShrink: 0,
};

const tagsWrapStyle = {
  display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.xs,
};

const tagStyle = {
  display: 'inline-flex', alignItems: 'center',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  background: cv('bg-secondary'),
  fontSize: AppleType.microUI.size, color: cv('text-secondary'),
};
</script>
