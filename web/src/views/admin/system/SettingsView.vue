<template>
  <ApPageContainer
    :title="$t('adminSys.k206')"
    :subtitle="$t('adminSys.k209')"
    :max-width="AppleLayout.maxContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <!-- 加载骨架 -->
    <ApCard v-if="loading">
      <ApSkeleton variant="form" :count="4" />
    </ApCard>

    <ApEmpty v-else-if="!groups || groups.length === 0" icon="Settings" :title="$t('adminSys.k207')" />

    <!-- 分组配置 -->
    <template v-else>
      <!-- 左右两栏：左侧分组导航 + 右侧表单 -->
      <div :style="layoutStyle">
        <!-- 左侧分组列表 -->
        <nav :style="sidebarStyle" :aria-label="$t('adminSys.k210')">
          <ApButton
            v-for="g in groups"
            :key="g.key"
            variant="ghost"
            size="small"
            :style="navItemStyle(activeGroup === g.key)"
            :aria-label="$t('uiText.k53', { p1: g.label })"
            @click="activeGroup = g.key"
          >
            <component :is="groupIcon(g.key)" :size="16" :color="activeGroup === g.key ? cv('blue') : cv('text-tertiary')" />
            <span>{{ g.label }}</span>
            <span v-if="pendingCountInGroup(g.key) > 0" :style="countBadgeStyle">{{ pendingCountInGroup(g.key) }}</span>
          </ApButton>
        </nav>

        <!-- 右侧表单 -->
        <div :style="contentStyle">
          <ApCard :key="activeGroup">
            <template #header>
              <div :style="groupHeaderStyle">
                <h2 :style="groupTitleStyle">{{ currentGroupLabel }}</h2>
                <p :style="groupDescStyle">{{ currentGroupDesc }}</p>
              </div>
            </template>

            <div :style="formStyle">
              <div v-for="item in currentItems" :key="item.key" :style="formRowStyle">
                <div :style="labelCellStyle">
                  <span :style="labelStyle">{{ item.label }}</span>
                  <span v-if="pendingChanges[item.key] !== undefined" :style="changedDotStyle" :title="$t('adminSys.k208')" :aria-label="$t('adminSys.k208')" />
                </div>
                <div :style="inputAreaStyle">
                  <ApSwitch
                    v-if="item.type === 'switch'"
                    :aria-label="String(item.label || item.key)"
                    :model-value="savedValues[item.key] === '1' || savedValues[item.key] === 'true'"
                    @update:model-value="(val) => handleSwitch(item, val)"
                  />
                  <ApSelect
                    v-else-if="item.type === 'select'"
                    :aria-label="String(item.label || item.key)"
                    :model-value="savedValues[item.key] || null"
                    :options="parseOptions(item.options)"
                    @update:model-value="(val) => handleSelect(item, val)"
                  />
                  <ApInput
                    v-else-if="item.type === 'number'"
                    type="number"
                    :aria-label="String(item.label || item.key)"
                    :model-value="savedValues[item.key] || ''"
                    @update:model-value="(val) => handleInputChange(item.key, val)"
                  />
                  <ApTextarea
                    v-else-if="item.type === 'textarea'"
                    :aria-label="String(item.label || item.key)"
                    :model-value="savedValues[item.key] || ''"
                    :rows="4"
                    @update:model-value="(val) => handleInputChange(item.key, val)"
                  />
                  <ApInput
                    v-else
                    :aria-label="String(item.label || item.key)"
                    :model-value="savedValues[item.key] || ''"
                    @update:model-value="(val) => handleInputChange(item.key, val)"
                  />
                  <ApButton
                    v-if="pendingChanges[item.key] !== undefined && item.type !== 'switch' && item.type !== 'select'"
                    variant="ghost"
                    size="small"
                    :disabled="saving"
                    @click="handleSaveSingle(item)"
                  >{{ $t('adminSys.k205') }}</ApButton>
                </div>
              </div>
            </div>

            <div :style="formActionsStyle">
              <span :style="pendingHintStyle">{{ pendingCount > 0 ? $t('adminSys.k390', { n: pendingCount }) : $t('adminSys.k391') }}</span>
              <ApButton variant="primary" size="small" :disabled="saving || pendingCount === 0" @click="handleSaveAll">
                {{ saving ? $t('vAdminUi.k1') : $t('vAdminUi.k32') }}
              </ApButton>
            </div>
          </ApCard>
        </div>
      </div>
    </template>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import { AppleSpace, AppleType, AppleRadius, AppleTransition, AppleFont, AppleFontWeight, cv, AppleLayout } from '@/utils/core/apple-tokens';
import { affectsSiteBrand, refreshSiteBrand } from '@/utils/core/site-brand';
import { icons } from '@/utils/icons';
import { useUIStore } from '@/stores/app/ui';
import { getConfigs, updateConfig } from '@/api/modules/system/config';
import type { ConfigGroup, ConfigItem } from '@/api/modules/system/config';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';

const ui = useUIStore();

// 分组 icon 映射（与后端 groupLabels 的 key 对齐；未知/历史分组回退 Globe）
const groupIconMap: Record<string, (typeof icons)[keyof typeof icons]> = {
  basic: icons['Settings'],
  search: icons['Search'],
  ai: icons['Zap'],
  system: icons['Info'],
};
function groupIcon(key: string) {
  return groupIconMap[key] || icons['Globe'];
}

// ── 状态 ──
const loading = shallowRef(false);
const saving = shallowRef(false);
const groups = ref<ConfigGroup[]>([]);
const activeGroup = shallowRef('');
const savedValues = ref<Record<string, string>>({});
const originalValues = ref<Record<string, string>>({});
const pendingChanges = ref<Record<string, string>>({});

// ── 计算属性 ──
const currentItems = computed(() => groups.value.find(g => g.key === activeGroup.value)?.items || []);
const currentGroupLabel = computed(() => groups.value.find(g => g.key === activeGroup.value)?.label || '');
const currentGroupDesc = computed(() => {
  const m: Record<string, string> = {
    basic: t('adminSys.k385'),
    search: t('adminSys.k386'),
    ai: t('adminSys.k387'),
    system: t('adminSys.k388'),
  };
  return m[activeGroup.value] || t('adminSys.k389');
});
const pendingCount = computed(() => Object.keys(pendingChanges.value).length);

function pendingCountInGroup(groupKey: string): number {
  const g = groups.value.find(x => x.key === groupKey);
  if (!g) return 0;
  return g.items.filter(item => pendingChanges.value[item.key] !== undefined).length;
}

// ── 布局样式 ──
const layoutStyle = {
  display: 'grid',
  gridTemplateColumns: 'minmax(160px, 200px) 1fr',
  gap: AppleSpace.lg,
  alignItems: 'start',
};

const sidebarStyle = {
  position: 'sticky' as const,
  top: AppleSpace.md,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  padding: `${AppleSpace.xxs} 0`,
};

const navItemStyle = (active: boolean) => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.medium,
  cursor: 'pointer',
  border: 'none',
  background: active ? cv('blue-selection-strong') : 'transparent',
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: active ? String(AppleFontWeight.medium) : String(AppleFontWeight.regular),
  color: active ? cv('blue') : cv('text-primary'),
  transition: AppleTransition.smooth,
  textAlign: 'left' as const,
  width: '100%',
});

const countBadgeStyle = {
  marginLeft: 'auto',
  minWidth: '18px',
  height: '18px',
  padding: `0 ${AppleSpace.xs}`,
  borderRadius: AppleRadius.circle,
  background: cv('orange'),
  color: cv('on-orange'),
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const contentStyle = { minWidth: '0' };

// ── 表单样式 ──
const groupHeaderStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
const groupTitleStyle = {
  fontFamily: AppleType.subhead.font,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: '0',
};
const groupDescStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  margin: '0',
};
const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
};
const formRowStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: AppleSpace.md,
  padding: `${AppleSpace.sm} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
};
const labelCellStyle = {
  width: '160px',
  flexShrink: '0',
  paddingTop: AppleSpace.xxs,
};
const labelStyle = {
  fontFamily: AppleType.controlLabel.font,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};
const changedDotStyle = {
  display: 'inline-block',
  width: '6px',
  height: '6px',
  borderRadius: AppleRadius.circle,
  background: cv('orange'),
};
const inputAreaStyle = { flex: '1', minWidth: '0', display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs };
const formActionsStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.sm,
  marginTop: AppleSpace.md,
  paddingTop: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}`,
};
const pendingHintStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-tertiary'),
};

// ── 数据操作 ──
async function loadConfigs() {
  loading.value = true;
  try {
    const data = await getConfigs();
    groups.value = data;
    if (data.length > 0) {
      activeGroup.value = data[0]!.key;
      for (const g of data) {
        for (const item of g.items) {
          savedValues.value[item.key] = item.value ?? '';
          originalValues.value[item.key] = item.value ?? '';
        }
      }
    }
  } catch {
    ui.showToast(t('adminSys.k211'), 'error');
  } finally {
    loading.value = false;
  }
}

function handleSwitch(item: ConfigItem, val: boolean) {
  const orig = originalValues.value[item.key] ?? '';
  // 沿用原值的布尔字面量风格（'true'/'false' 或 '1'/'0'），避免同义不同值触发伪修改提示
  const usesTrueFalse = orig === 'true' || orig === 'false';
  const newVal = usesTrueFalse ? (val ? 'true' : 'false') : val ? '1' : '0';
  if (newVal === orig) {
    delete pendingChanges.value[item.key];
  } else {
    pendingChanges.value[item.key] = newVal;
  }
  savedValues.value[item.key] = newVal;
}

function handleSelect(item: ConfigItem, val: string | number | null) {
  const sVal = val != null ? String(val) : '';
  if (sVal === (originalValues.value[item.key] ?? '')) {
    delete pendingChanges.value[item.key];
  } else {
    pendingChanges.value[item.key] = sVal;
  }
  savedValues.value[item.key] = sVal;
}

function handleInputChange(key: string, val: string) {
  if (val === (originalValues.value[key] ?? '')) {
    delete pendingChanges.value[key];
  } else {
    pendingChanges.value[key] = val;
  }
}

async function handleSaveAll() {
  if (saving.value) return;
  const entries = Object.entries(pendingChanges.value);
  if (entries.length === 0) return;
  saving.value = true;
  const backup = { ...savedValues.value };
  try {
    const results = await Promise.allSettled(entries.map(([k, v]) => updateConfig(k, v)));
    const failed: string[] = [];
    results.forEach((r, i) => {
      const entry = entries[i];
      if (!entry) return;
      if (r.status === 'fulfilled') {
        savedValues.value[entry[0]] = entry[1];
        originalValues.value[entry[0]] = entry[1];
        delete pendingChanges.value[entry[0]];
      } else {
        failed.push(entry[0]);
      }
    });
    if (failed.length === 0) {
      ui.showToast(t('adminSys.k212'), 'success');
      await loadConfigs();
      // 这几栏改的是全站显示的名字与那句描述，保存成功后必须立刻反映到标签页标题、
      // 外壳与 <meta name="description">，而不是等用户刷新一次 ——
      // 之前"改了不生效"正是这个配置键被人骂的原因。键清单落在 site-brand.ts。
      if (entries.some(([key]) => affectsSiteBrand(key))) {
        await refreshSiteBrand();
      }
    } else if (failed.length < entries.length) {
      ui.showToast(t('sentUi.k25', { p1: entries.length - failed.length, p2: failed.length }), 'warning');
      await loadConfigs();
    } else {
      savedValues.value = backup;
      ui.showToast(t('adminSys.k213'), 'error');
    }
  } catch (e: unknown) {
    savedValues.value = backup;
    ui.showToast((e instanceof Error ? e.message : '') || t('fallback.k8'), 'error');
  } finally {
    saving.value = false;
  }
}

async function handleSaveSingle(item: ConfigItem) {
  if (saving.value) return;
  const val = pendingChanges.value[item.key];
  if (val === undefined) return;
  saving.value = true;
  try {
    await updateConfig(item.key, val);
    savedValues.value[item.key] = val;
    originalValues.value[item.key] = val;
    delete pendingChanges.value[item.key];
    // 单行保存走的是另一个分支，此前只有"批量保存"那条会刷新品牌 ——
    // 主页标题/描述那一行恰恰用的是单行按钮，改完永远要手动刷新一次。
    if (affectsSiteBrand(item.key)) await refreshSiteBrand();
    ui.showToast(t('sentUi.k26', { p1: item.label }), 'success');
  } catch (e: unknown) {
    ui.showToast((e instanceof Error ? e.message : '') || t('fallback.k8'), 'error');
  } finally {
    saving.value = false;
  }
}

// options 兼容两种存储格式：逗号分隔字符串（seedDefaults 实际格式）与 JSON 数组
interface SelectOption { label: string; value: string | number }
function parseOptions(options: string): SelectOption[] {
  const s = (options || '').trim();
  if (!s) return [];
  if (s.startsWith('[')) {
    try {
      const arr: unknown = JSON.parse(s);
      if (Array.isArray(arr)) {
        return arr.map((o) => {
          if (typeof o === 'object' && o !== null) {
            const rec = o as Record<string, unknown>;
            return { label: String(rec.label ?? rec.value), value: rec.value as string | number };
          }
          return { label: String(o), value: o as string | number };
        });
      }
    } catch { /* 退回逗号分隔解析 */ }
  }
  return s.split(',').map((t) => t.trim()).filter(Boolean).map((t) => ({ label: t, value: t }));
}

onMounted(() => { loadConfigs(); });
</script>
