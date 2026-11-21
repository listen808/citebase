<template>
  <ApPageContainer
    :title="$t('adminSys.k258')"
    :subtitle="$t('adminSys.k259')"
    :max-width="AppleLayout.maxContentWidth"
    :show-breadcrumb="true"
    :breadcrumb-parent="{ label: $t('vAdminUi.k38'), path: '/settings/sso' }"
    :show-back="true"
    back-to="/settings/sso"
    compact
  >
    <!-- 与 SSOConfigView 同一组「配置/文档/测试」标签（审计 B10）：本页恒选中「文档」，
         点另两档跳回 /settings/sso 并带 ?tab=，标签栏不再断层消失 -->
    <ApSegmentedControl
      :model-value="'docs'"
      :options="ssoSectionTabs"
      :style="tabBarStyle"
      @update:model-value="goSsoSection"
    />

    <!-- P2-12：LDAP/HTTP 切换原先浮在标题右下方中部、脱离语境，上移到页头操作区 -->
    <template #actions>
      <ApSegmentedControl
        v-model="activeTab"
        :options="tabs"
        size="small"
      />
    </template>

    <ApCard size="small" :style="docCardStyle">
    <div :style="docLayoutStyle">
      <nav :style="[tocNavStyle, { flex: '0 0 200px' }]">
        <div :style="tocHeaderStyle">
          <ApIcon name="List" :size="16" :color="cv('text-secondary')" />
          <span :style="tocHeaderLabelStyle">{{ $t('adminSys.k241') }}</span>
        </div>
        <div :style="tocListStyle">
          <div
            v-for="item in tocItems"
            :key="item.id"
            :style="[tocItemStyle(item.level), activeHeading === item.id && tocItemActiveStyle()]"
            @click="scrollToHeading(item.id)"
          >
            {{ item.label }}
          </div>
        </div>
      </nav>

      <div :style="readingAreaStyle">
        <div v-if="activeTab === 'ldap'" :style="contentStyle">
          <h2 id="ldap-overview" :style="headingStyle">{{ $t('adminSys.k219') }}</h2>
          <p :style="bodyTextStyle">{{ $t('adminSys.k220') }}</p>

          <h3 id="ldap-example" :style="subheadingStyle">{{ $t('adminSys.k242') }}</h3>
          <div :style="codeBlockStyle">
            <pre :style="preStyle">{{ $t('adminSys.k243') }}</pre>
          </div>

          <h3 id="ldap-fields" :style="subheadingStyle">{{ $t('adminSys.k244') }}</h3>
          <div :style="tableStyle">
            <div v-for="row in ldapFields" :key="row.field" :style="tableRowStyle">
              <code :style="fieldCodeStyle">{{ row.field }}</code>
              <span :style="fieldDescStyle">{{ row.desc }}</span>
            </div>
          </div>

          <h3 id="ldap-steps" :style="subheadingStyle">{{ $t('adminSys.k245') }}</h3>
          <ol :style="stepsListStyle">
            <li :style="stepItemStyle">{{ $t('adminSys.k246') }}</li>
            <li :style="stepItemStyle">{{ $t('adminSys.k247') }}</li>
            <li :style="stepItemStyle">{{ $t('adminSys.k248') }}</li>
            <li :style="stepItemStyle">{{ $t('adminSys.k249') }}</li>
            <li :style="stepItemStyle">{{ $t('adminSys.k250') }}</li>
          </ol>
        </div>

        <div v-if="activeTab === 'http'" :style="contentStyle">
          <h2 id="http-overview" :style="headingStyle">{{ $t('adminSys.k251') }}</h2>
          <p :style="bodyTextStyle">{{ $t('adminSys.k252') }}</p>

          <h3 id="http-oauth-example" :style="subheadingStyle">{{ $t('adminSys.k253') }}</h3>
          <div :style="codeBlockStyle">
            <pre :style="preStyle">{{ $t('adminSys.k254') }}</pre>
          </div>

          <h3 id="http-php-example" :style="subheadingStyle">{{ $t('adminSys.k255') }}</h3>
          <div :style="codeBlockStyle">
            <pre :style="preStyle">// SSO 回调处理示例 (callback.php)
&lt;?php
$code = $_GET['code'] ?? '';
if (!$code) {
    // 重定向到授权页面
    $authUrl = 'https://sso.example.com/authorize?' . http_build_query([
        'client_id' => 'your-client-id',
        'redirect_uri' => 'https://your-app.com/callback',
        'response_type' => 'code',
        'scope' => 'openid profile email'
    ]);
    header('Location: ' . $authUrl);
    exit;
}

// 用授权码换取 token
$ch = curl_init('https://sso.example.com/token');
curl_setopt($ch, CURLOPT_POST, 1);
curl_setopt($ch, CURLOPT_POSTFIELDS, http_build_query([
    'grant_type' => 'authorization_code',
    'code' => $code,
    'client_id' => 'your-client-id',
    'client_secret' => 'your-client-secret',
    'redirect_uri' => {{ $t('adminSys.k256') }}</pre>
          </div>

          <h3 id="http-ext-config" :style="subheadingStyle">{{ $t('adminSys.k257') }}</h3>
          <div :style="codeBlockStyle">
            <pre :style="preStyle">{
  "callback_url": "https://your-app.com/callback",
  "username_field": "preferred_username",
  "given_name_field": "given_name",
  "email_field": "email",
  "mobile_field": "mobile",
  "extra_headers": {
    "X-Custom-Header": "value"
  }
}</pre>
          </div>
        </div>
      </div>
    </div>
    </ApCard>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import ApIcon from '@/components/apple/ApIcon.vue';
import { AppleLayout, AppleSpace, cv } from '@/utils/core/apple-tokens';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import { docLayoutStyle, tocNavStyle, tocHeaderStyle, tocHeaderLabelStyle, tocListStyle, tocItemStyle, tocItemActiveStyle, readingAreaStyle, contentStyle, docHeadingStyle, docSubheadingStyle, docBodyTextStyle, docCodeBlockStyle, docPreStyle, docTableStyle, docTableRowStyle, docFieldCodeStyle, docFieldDescStyle, docStepsListStyle, docStepItemStyle, tabBarStyle } from '@/styles/layout/page-styles';

const activeTab = ref<'ldap' | 'http'>('ldap');
const activeHeading = ref('');

const tabs = [
  { value: 'ldap' as const, label: t('adminSys.k260') },
  { value: 'http' as const, label: t('adminSys.k261') },
];

// 与 SSOConfigView 的「配置/文档/测试」标签同一组文案与键（审计 B10）
const router = useRouter();
const ssoSectionTabs = [
  { value: 'config' as const, label: t('adminSys.k189') },
  { value: 'docs' as const, label: t('adminSys.k236') },
  { value: 'test' as const, label: t('adminSys.test') },
];
function goSsoSection(value: string | number) {
  if (value === 'config' || value === 'test') {
    router.push({ path: '/settings/sso', query: { tab: value } });
  }
}

const ldapToc = [
  { id: 'ldap-overview', label: t('adminSys.k219'), level: 1 },
  { id: 'ldap-example', label: t('adminSys.k242'), level: 2 },
  { id: 'ldap-fields', label: t('adminSys.k244'), level: 2 },
  { id: 'ldap-steps', label: t('adminSys.k245'), level: 2 },
];

const httpToc = [
  { id: 'http-overview', label: t('adminSys.k251'), level: 1 },
  { id: 'http-oauth-example', label: t('adminSys.k253'), level: 2 },
  { id: 'http-php-example', label: t('adminSys.k255'), level: 2 },
  { id: 'http-ext-config', label: t('adminSys.k257'), level: 2 },
];

const tocItems = computed(() => activeTab.value === 'ldap' ? ldapToc : httpToc);

const ldapFields = [
  { field: 'host', desc: t('adminSys.k262') },
  { field: 'port', desc: t('adminSys.k263') },
  { field: 'base_dn', desc: t('adminSys.k264') },
  { field: 'user_filter', desc: t('adminSys.k265') },
  { field: 'bind_dn', desc: t('adminSys.k266') },
  { field: 'bind_password', desc: t('adminSys.k267') },
  { field: 'username_attribute', desc: t('adminSys.k268') },
  { field: 'given_name_attribute', desc: t('adminSys.k269') },
  { field: 'email_attribute', desc: t('adminSys.k270') },
];

function scrollToHeading(id: string) {
  activeHeading.value = id;
  const el = document.getElementById(id);
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
}

function onScroll() {
  const ids = tocItems.value.map(t => t.id);
  for (let i = ids.length - 1; i >= 0; i--) {
    const el = document.getElementById(ids[i]!);
    if (el) {
      const rect = el.getBoundingClientRect();
      if (rect.top <= 120) {
        activeHeading.value = ids[i]!;
        return;
      }
    }
  }
  if (ids.length > 0) activeHeading.value = ids[0]!;
}

watch(activeTab, () => {
  activeHeading.value = '';
});

onMounted(() => {
  window.addEventListener('scroll', onScroll, { passive: true });
});

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll);
});

// 样式别名映射
const headingStyle = docHeadingStyle;
const subheadingStyle = docSubheadingStyle;
const bodyTextStyle = docBodyTextStyle;
const codeBlockStyle = docCodeBlockStyle;
const preStyle = docPreStyle;
const tableStyle = docTableStyle;
const tableRowStyle = docTableRowStyle;
const fieldCodeStyle = docFieldCodeStyle;
const fieldDescStyle = docFieldDescStyle;
const stepsListStyle = docStepsListStyle;
const stepItemStyle = docStepItemStyle;
// 正文与文档页同一种承载：卡片包住目录 + 阅读区（P2-12）
const docCardStyle = { marginTop: AppleSpace.md };
</script>
