<template>
  <ApModal :model-value="visible" size="medium" @update:model-value="$emit('update:visible', $event)">
    <template #title>
      <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
        <ApIcon name="BookOpen" :size="18" /> {{ $t('aiChat.k57') }}
        <span v-if="sources.length" :style="{ fontSize:AppleType.microUIBold.size,color:cv('text-tertiary'),fontWeight:'normal' }">
          {{ $t('common.itemCount', { n: sources.length }) }}
        </span>
      </div>
    </template>
    <div v-if="sources.length === 0" :style="{ textAlign: 'center', padding: AppleSpace.lg, color: cv('text-tertiary') }">
      {{ $t('aiChat.k58') }}
    </div>
    <div v-else :style="{ display: 'flex', flexDirection: 'column', gap: AppleSpace.sm }">
      <div
        v-for="(s, i) in sources"
        :id="'src-' + (s.citation_index ?? i)"
        :key="i"
        :style="sourceItemStyle(i)"
        @click="toggleExpand(i)"
      >
        <!-- Header row -->
        <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
          <span v-if="s.citation_index" :style="citeBadgeStyle">[{{ s.citation_index }}]</span>
          <span :style="sourceIconStyle">
            <ApIcon name="FileText" :size="12" />
          </span>
          <div style="flex:1;min-width:0">
            <div style="font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis">
              {{ s.document_name || `来源 ${i + 1}` }}
            </div>
            <div v-if="s.heading_path" :style="{ fontSize:AppleType.microUI.size,letterSpacing:AppleType.microUI.letterSpacing,color:cv('text-tertiary'),whiteSpace:'nowrap',overflow:'hidden',textOverflow:'ellipsis' }">
              {{ s.heading_path }}
            </div>
          </div>
          <span v-if="s.verify_status" :style="verifyBadgeStyle(s.verify_status)">
            <ApIcon v-if="s.verify_status === 'cited'" name="CheckCircle" :size="12" />
            <ApIcon v-else-if="s.verify_status === 'not_found'" name="AlertCircle" :size="12" />
            <span>{{ verifyLabel(s.verify_status) }}</span>
          </span>
          <ApIcon name="ChevronDown" :size="14" :style="{ color:cv('text-quaternary'),transition:'transform var(--apple-dur-interactive)',transform:expandedSet.has(i)?'rotate(180deg)':'none',flexShrink:0 }" />
        </div>

        <!-- Expandable snippet -->
        <div v-if="expandedSet.has(i) && s.snippet" :style="snippetExpandStyle">
          <div :style="{ fontSize:AppleType.microUIBold.size,color:cv('text-secondary'),lineHeight:AppleLineHeight.spacious,whiteSpace:'pre-wrap',wordBreak:'break-word' }">
            {{ s.snippet }}
          </div>
        </div>
        <!-- Collapsed snippet preview -->
        <div v-else-if="s.snippet" :style="snippetCollapseStyle">
          {{ s.snippet }}
        </div>

        <!-- Footer row -->
        <div v-if="expandedSet.has(i)" :style="{ display: 'flex', alignItems: 'center', gap: '12px', marginTop: AppleSpace.sm, paddingTop: AppleSpace.sm, borderTop: '1px solid var(--apple-border-light)' }">
          <span v-if="s.score" :style="{ fontSize:AppleType.microUI.size,color:cv('text-tertiary') }">
            相关度 {{ Math.round(s.score * 100) }}%
          </span>
          <span v-if="s.keywords?.length" :style="{ fontSize:AppleType.microUI.size,color:cv('text-tertiary') }">
            关键词: {{ s.keywords.slice(0, 3).join(', ') }}
          </span>
          <span v-if="s.paragraph_id" :style="{ fontSize:AppleType.microUI.size,color:cv('text-tertiary') }">
            ¶{{ s.paragraph_id }}
          </span>
          <a
v-if="s.doc_url" href="javascript:void(0)" :style="{ fontSize:AppleType.microUI.size,color:cv('blue'),textDecoration:'none',marginLeft:'auto',cursor:'pointer' }"
             @click.prevent="openDoc(s.doc_url)">
            {{ $t('aiChat.k59') }}
          </a>
        </div>
      </div>
    </div>
  </ApModal>
</template>

<script setup lang="ts">
import ApModal from '@/components/apple/ApModal.vue';
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleLineHeight } from '@/utils/core/apple-tokens';
import { watch, nextTick, ref, reactive } from 'vue';
import { useRouter } from 'vue-router';
import type { ChatSource } from '@/composables/ai/useChatMessages';

type VerifyStatus = 'cited' | 'unverified' | 'not_found';

const props = defineProps<{
  visible: boolean;
  sources: ChatSource[];
  highlightCitationIndex?: number;
}>();

defineEmits<{
  'update:visible': [value: boolean];
}>();

const router = useRouter();
const highlightedIndex = ref(-1);
const expandedSet = reactive(new Set<number>());

// 规范化文档 URL，兼容旧格式 /wiki/{spaceId}/{docId}
function normalizeDocUrl(url: string): string {
  const legacy = url.match(/^\/wiki\/(\d+)\/(\d+)$/);
  return legacy ? `/space/${legacy[1]}/doc/${legacy[2]}` : url;
}

function openDoc(url: string) {
  router.push(normalizeDocUrl(url));
}

function toggleExpand(i: number) {
  if (expandedSet.has(i)) expandedSet.delete(i);
  else expandedSet.add(i);
}

watch(() => [props.visible, props.highlightCitationIndex], async ([v, idx]) => {
  if (v && idx != null && (idx as number) > 0) {
    highlightedIndex.value = idx as number;
    // Auto-expand the highlighted source
    expandedSet.add((idx as number) - 1);
    await nextTick();
    const el = document.getElementById(`src-${idx}`);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
      el.style.background = cv('blue-light');
      el.style.transition = 'background var(--apple-dur-smooth)';
      setTimeout(() => {
        el.style.background = cv('bg-secondary');
        highlightedIndex.value = -1;
      }, 2000);
    }
  }
});

const sourceItemStyle = (i: number) => ({
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  background: highlightedIndex.value >= 0 && i === highlightedIndex.value - 1 ? cv('blue-light') : cv('bg-secondary'),
  borderRadius: AppleRadius.small,
  fontSize: AppleType.microUI.size,
  transition: 'background var(--apple-dur-smooth)',
  cursor: 'pointer',
});

const sourceIconStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  width: '24px', height: '24px', borderRadius: AppleRadius.small,
  background: cv('blue-light'), color: cv('blue'), flexShrink: '0',
};

const snippetExpandStyle = {
  marginTop: AppleSpace.xs, paddingLeft: '28px',
};

const snippetCollapseStyle = {
  marginTop: AppleSpace.xxs, paddingLeft: '28px',
  fontSize: AppleType.microUIBold.size, color: cv('text-tertiary'), lineHeight: AppleLineHeight.default,
  whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis' as const,
};

const citeBadgeStyle = {
  flexShrink: '0',
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  minWidth: '22px',
  height: '20px',
  padding: '0 6px',
  borderRadius: AppleRadius['6px'],
  background: cv('blue'),
  color: cv('on-blue'),
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
};

function verifyLabel(status: VerifyStatus): string {
  switch (status) {
    case 'cited': return '已引用';
    case 'unverified': return '未引用';
    case 'not_found': return '未找到';
    default: return status;
  }
}

function verifyBadgeStyle(status: VerifyStatus) {
  const map: Record<VerifyStatus, { bg: string; color: string }> = {
    cited: { bg: cv('green-light'), color: cv('green') },
    unverified: { bg: cv('orange-light'), color: cv('orange') },
    not_found: { bg: cv('red-light'), color: cv('red') },
  };
  const c = map[status] || map.unverified;
  return {
    display: 'inline-flex', alignItems: 'center', gap: AppleSpace.xxs,
    padding: `1px ${AppleSpace.xs}`,
    borderRadius: AppleRadius['6px'],
    fontSize: AppleType.microUI.size,
    letterSpacing: AppleType.microUI.letterSpacing,
    fontWeight: String(AppleFontWeight.medium),
    background: c.bg,
    color: c.color,
    flexShrink: '0',
  };
}
</script>
