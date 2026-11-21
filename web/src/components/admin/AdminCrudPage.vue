<script setup lang="ts">
import type { Component } from 'vue';
import { computed, watch } from 'vue';
import type { IconName } from '@/utils/icons';
import { storageGet, storageSet, StorageKeys } from '@/utils/core/storage';
import { AppleSpace } from '@/utils/core/apple-tokens';
import type { AppleDensity } from '@/utils/core/apple-tokens';
import { crudStatsRowStyle, crudFilterBarStyle, crudPaginationStyle, crudSearchStyle, toolbarSpacerStyle } from '@/styles/layout/page-styles';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApSearch from '@/components/apple/ApSearch.vue';
import ApViewToggle from '@/components/apple/ApViewToggle.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApError from '@/components/apple/ApError.vue';
import ApPagination from '@/components/apple/ApPagination.vue';

export interface StatConfig {
  label: string
  value: string | number
  icon?: Component | IconName
  color?: string
  theme?: string
}

export interface FilterConfig {
  search?: { placeholder: string }
  viewToggle?: boolean
}

const props = withDefaults(defineProps<{
  stats: StatConfig[]
  filters?: FilterConfig
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
  // 与 ApEmpty / ApTable 同档：除注册表名外还要能收组件，
  // 否则导航上那些不在 icons.ts 里的专指图标（Fingerprint / LockKeyhole）无法在空态复用同一符号。
  emptyIcon?: Component | IconName
  emptyText?: string
  errorText?: string | null
  showEmpty?: boolean
  // 密度轴与 ApTable / ApModal / ApConfirmModal 同名：这一档从来不是"尺寸"
  density?: AppleDensity
}>(), {
  filters: () => ({}),
  loading: false,
  total: 0,
  page: 1,
  pageSize: 20,
  emptyIcon: 'Inbox',
  showEmpty: true,
  size: 'compact',
})

const emit = defineEmits<{
  search: [keyword: string]
  'page-change': [page: number]
  'update:page': [page: number]
  retry: []
}>()

const keyword = defineModel<string>('keyword', { default: '' })
// 档位默认值取全站那一个键：以前每页各自 'card' 起步，刷新/切页即丢失选择
const viewMode = defineModel<'card' | 'table'>('viewMode', {
  default: storageGet(StorageKeys.UI_COLLECTION_VIEW_MODE) === 'table' ? 'table' : 'card',
})
watch(viewMode, (mode) => storageSet(StorageKeys.UI_COLLECTION_VIEW_MODE, mode))

const toolbarStyle = computed(() => crudFilterBarStyle(props.density))
</script>

<template>
  <slot name="stats" :stats-style="crudStatsRowStyle(density)">
    <div v-if="stats.length" :style="crudStatsRowStyle(density)">
      <ApStatCard
        v-for="(stat, i) in stats"
        :key="i"
        :size="density"
        :label="stat.label"
        :value="stat.value"
        :icon="stat.icon"
        :color="stat.color"
        :theme="stat.theme"
      />
    </div>
  </slot>

  <div :style="toolbarStyle">
    <div :style="{ display: 'flex', alignItems: 'flex-end', gap: AppleSpace.md, flex: '1', flexWrap: 'wrap' }">
      <!-- 顺序与 FilterBar 同一条规则：左「看哪些」→ 弹簧 → 右「怎么看」（视图切换 + 搜索）→ 动作。
           原先搜索与视图切换占最左、筛选插槽排在它们后面，同一套"左看哪些右怎么看"在两套外壳里
           是两个方向，页面在两套之间搬控件时位置会左右对调。 -->
      <slot name="filter-extra" />
      <div :style="toolbarSpacerStyle()" />
      <ApViewToggle v-if="filters.viewToggle" v-model="viewMode" />
      <div v-if="filters.search" :style="crudSearchStyle()">
        <ApSearch
          v-model="keyword"
          :placeholder="filters.search.placeholder"
          @search="(val: string) => emit('search', val)"
        />
      </div>
      <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
        <slot name="actions" />
      </div>
    </div>
  </div>

  <!-- 错误态归外壳：页面一旦覆写 #content，slot fallback 里的 ApError 就永不可达，
       接口失败时 10/11 个后台页因此把「加载失败」渲染成「暂无数据」。 -->
  <ApError v-if="errorText && !loading" :message="errorText" show-retry @retry="emit('retry')" />
  <slot v-else name="content" :loading="loading" :view-mode="viewMode">
    <ApEmpty v-if="showEmpty && !loading" :icon="emptyIcon" :title="emptyText || $t('common.noData')" />
  </slot>

  <div v-if="total > (pageSize || 20)" :style="crudPaginationStyle(density)">
    <ApPagination
      :page="page"
      :total="total"
      :page-size="pageSize"
      @change="(p: number) => { emit('page-change', p); emit('update:page', p) }"
    />
  </div>

  <slot name="modals" />
</template>
