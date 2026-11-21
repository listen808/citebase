<template>
  <ApPageContainer
    :max-width="AppleLayout.maxContentWidth"
    :title="docData?.name || $t('chrome.titleShare')"
    :show-breadcrumb="true"
    compact
  >
    <!-- 加载中 -->
    <ApSkeleton v-if="loading" variant="card" />

    <!-- 错误 -->
    <ApError v-else-if="error" :message="error" :show-retry="retryable" @retry="loadDocument" />

    <!-- 密码验证 -->
    <div v-else-if="needsPassword" :style="passwordCardStyle">
      <div :style="passwordHintStyle">{{ $t('share.k1') }}</div>
      <div :style="passwordInputRowStyle">
        <ApInput v-model="sharePassword" type="password" :placeholder="$t('share.k4')" :style="passwordInputStyle" @keyup.enter="submitPassword" />
        <ApButton variant="primary" size="small" :loading="passwordLoading" @click="submitPassword">{{ $t('share.k2') }}</ApButton>
      </div>
      <div v-if="passwordError" :style="passwordErrorStyle">{{ passwordError }}</div>
    </div>

    <!-- 内容 -->
    <!-- 入场编排（批4）：分享页是唯一会被转发给外部人的版面，顺序就是阅读顺序（元信息 → 标题 → 正文） -->
    <div v-else-if="docData" class="apple-persona-stack">
      <!-- 元信息 -->
      <div :style="metaStyle">
        <router-link
          v-if="isLoggedIn && docData.creator_name && docData.creator_id"
          :to="'/user/' + docData.creator_id"
          :style="shareMetaLinkStyle"
        >{{ $t('common.createdBy', { name: docData.creator_name }) }}</router-link>
        <span v-else-if="docData.creator_name">{{ $t('common.createdBy', { name: docData.creator_name }) }}</span>
        <span v-if="docData.editor_name" :style="{ color: cv('text-quaternary'), padding: `0 ${AppleSpace.xs}` }">·</span>
        <router-link
          v-if="isLoggedIn && docData.editor_name && docData.editor_id"
          :to="'/user/' + docData.editor_id"
          :style="shareMetaLinkStyle"
        >{{ $t('common.editedBy', { name: docData.editor_name }) }}</router-link>
        <span v-else-if="docData.editor_name">{{ $t('common.editedBy', { name: docData.editor_name }) }}</span>
        <span v-if="docData.update_time" :style="{ color: cv('text-quaternary'), padding: `0 ${AppleSpace.xs}` }">·</span>
        <span v-if="docData.update_time">{{ formatTime(docData.update_time) }}</span>
        <span v-if="docData.space_name" :style="{ color: cv('text-quaternary'), padding: `0 ${AppleSpace.xs}` }">·</span>
        <span v-if="docData.space_name">{{ docData.space_name }}</span>
      </div>

      <!-- 文档内容 -->
      <div :style="docLayoutStyle">
        <!-- 分享出去的那张纸就是本页的主对象：底色与 elevation 成对取自 primarySurfaceStyle，
             内边距从 680px 度量之外让出来，保证度量说的是文字栏而不是纸宽 -->
        <div :style="readingSheetStyle">
          <div :style="[cardBodyStyle, readingAreaStyle]">
            <ApMarkdownPreview :content="docData.content || ''" />
          </div>
        </div>
      </div>
    </div>

    <!-- 回到顶部按钮：全站唯一的 FAB 组件。原先这里用 ApButton + 一份手写的
         position:fixed + circle + shadow，等于在 `ApFloatingActionButton` 之外又造一个 FAB；
         而且那份 :style 覆盖了 ApButton 的 background/border/shadow，正是明令禁止的写法
         （底色还取了阶梯外的 bg-secondary）。 -->
    <ApFloatingActionButton
      v-if="showScrollTop"
      variant="ghost"
      :label="$t('share.k3')"
      :bottom="AppleSpace.xl"
      :right="AppleSpace.xl"
      @click="scrollToTop"
    >
      <ApIcon name="ChevronUp" :size="14" />
    </ApFloatingActionButton>

    <!-- Footer -->
    <div :style="footerStyle">
      <span :style="footerTextStyle">{{ $t('share.poweredBy', { app: siteBrandName }) }}</span>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, onUnmounted, shallowRef } from 'vue';
import { useRoute } from 'vue-router';
import { usePageMeta } from '@/composables/shared/usePageMeta';
import { shareDocument, type ShareDocumentResponse } from '@/api/modules/document/document';
import { getApiErrorStatus } from '@/api/client';
import { siteBrandName } from '@/utils/core/site-brand';
import { useAuthStore } from '@/stores/auth/auth';
import { formatTime } from '@/utils/format/format-time';
import { AppleFont, AppleLayout, AppleSpace, AppleType, cv, AppleLineHeight, AppleRadius, primarySurfaceStyle } from '@/utils/core/apple-tokens';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import { docLayoutStyle, readingAreaStyle } from '@/styles/layout/page-styles';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';

const route = useRoute();
const authStore = useAuthStore();
const isLoggedIn = computed(() => authStore.isLoggedIn);

const id = computed(() => Number(route.params.id));

const loading = shallowRef(true);
const error = ref('');
// 404/403 是"这个链接现在就是看不了"的事实陈述，重试按钮按下去只会得到同一句 ——
// 只有网络抖动/服务端错误（其余状态码）才值得给用户一个"再试一次"。
const retryable = ref(false);
const docData = ref<ShareDocumentResponse | null>(null);

// 分享页是全站唯一"链接会被发到别处"的入口，原先连 og:title 都没有 ——
// 粘到聊天工具里读不出标题。图片没有按文档生成的资产，用应用图标兜底。
usePageMeta(() => ({
  title: docData.value?.name,
  // 带密码的分享在输对密码之前服务端只回 has_password，此时 space_name 是 undefined，
  // 直接拼进模板字符串会渲染出字面量 "undefined"。
  description: docData.value?.space_name
    ? `${docData.value.space_name}${docData.value.creator_name ? ` · 由 ${docData.value.creator_name} 创建` : ''}`
    : undefined,
  image: `${window.location.origin}/pwa-icon.svg`,
  url: `/share/${id.value}`,
}));
const showScrollTop = ref(false);
const needsPassword = ref(false);
const sharePassword = ref('');
const passwordLoading = ref(false);
const passwordError = ref('');

const { t } = useI18n();
async function loadDocument(password?: string) {
  if (!id.value) {
    error.value = t('sentUi.k55');
    loading.value = false;
    return;
  }

  try {
    docData.value = await shareDocument(id.value, password);
    error.value = '';
    if (docData.value.has_password && !docData.value.content) {
      needsPassword.value = true;
      loading.value = false;
      return;
    }
    needsPassword.value = false;
  } catch (e: unknown) {
    // P1-2: 拦截器已改为 ApiError，状态码从 err.status 读取（404/403 区分恢复生效）
    const status = getApiErrorStatus(e);
    if (status === 404) {
      error.value = t('sentUi.k56');
    } else if (status === 403) {
      error.value = t('sentUi.k57');
    } else {
      error.value = t('sentUi.k58');
      retryable.value = true;
    }
  } finally {
    loading.value = false;
  }
}

async function submitPassword() {
  if (!sharePassword.value) return;
  passwordLoading.value = true;
  passwordError.value = '';
  try {
    docData.value = await shareDocument(id.value, sharePassword.value);
    needsPassword.value = false;
  } catch (e: unknown) {
    passwordError.value = getApiErrorStatus(e) === 403 ? '密码错误，请重试' : '验证失败，请稍后重试';
  } finally {
    passwordLoading.value = false;
  }
}

function onScroll() {
  showScrollTop.value = window.scrollY > 400;
}

function scrollToTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' });
}

onMounted(() => {
  loadDocument();
  window.addEventListener('scroll', onScroll, { passive: true });
});

onUnmounted(() => {
  window.removeEventListener('scroll', onScroll);
});

// ---- Styles ----
const metaStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  fontFamily: AppleFont.text,
  marginBottom: AppleSpace.md,
  paddingBottom: AppleSpace.sm,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const shareMetaLinkStyle = {
  color: cv('blue'),
  textDecoration: 'none',
  cursor: 'pointer',
};

const cardBodyStyle = {
  lineHeight: AppleLineHeight.wide,
  flex: 1,
  minWidth: 0,
};

const readingSheetStyle = primarySurfaceStyle({
  width: '100%',
  maxWidth: `calc(${readingAreaStyle.maxWidth} + ${AppleSpace.xl} * 2)`,
  padding: AppleSpace.xl,
  margin: `0 auto ${AppleSpace.lg}`,
});

const footerStyle = {
  textAlign: 'center' as const,
  padding: `${AppleSpace.lg} ${AppleSpace.md}`,
  marginTop: AppleSpace.lg,
  borderTop: `1px solid ${cv('border-light')}`,
};

const footerTextStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};

const passwordCardStyle = {
  background: cv('surface-panel'),
  // P1-1：半透明面板必须有 1px 发丝边，否则在浅色画布上边界消失（与 ApCard 同一条）
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.card,
  padding: `${AppleSpace.lg} ${AppleSpace.md}`,
  maxWidth: '440px',
  margin: '0 auto',
  textAlign: 'center' as const,
};
const passwordHintStyle = {
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'),
  marginBottom: AppleSpace.md,
};
const passwordInputRowStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  justifyContent: 'center',
};
const passwordInputStyle = {
  flex: '1',
  maxWidth: AppleSpace['260px'],
};
const passwordErrorStyle = {
  color: cv('red'),
  fontSize: AppleType.microUI.size,
  marginTop: AppleSpace.xs,
};
</script>

