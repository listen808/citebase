<template>
  <ApPageContainer no-scroll>
    <div :style="wrapperStyle">
      <!-- 入场编排（批4）：安装向导是低频版面，允许有编排 -->
      <div :style="cardStyle" class="apple-persona-stack">
        <div :style="logoAreaStyle">
        <ApBrand :size="40" :show-wordmark="false" enter />
        <h1 :style="titleStyle" data-ap-page-title>{{ $t('vAuthUi.k1') }}</h1>
        <p :style="subtitleStyle">{{ $t('vAuthUi.k2', { app: siteBrandName }) }}</p>
      </div>

      <!-- 步骤指示器 -->
      <div :style="stepsStyle">
        <div v-for="(step, i) in steps" :key="i" :style="stepItemStyle">
          <div :style="stepNumStyle(i)">{{ i + 1 }}</div>
          <span :style="stepLabelStyle(i)">{{ step }}</span>
        </div>
      </div>

      <!-- Step 1: 检查状态 -->
      <div v-if="currentStep === 0" :style="stepContentStyle">
        <div :style="stateCardStyle">
          <!-- 检查结果区初始为空：不交代一句，这块灰卡看起来就是渲染坏了（审查 B18） -->
          <div v-if="!loading.next && !Object.keys(installState).length" :style="stateHintStyle">{{ $t('vAuthUi.k27') }}</div>
          <div v-for="(val, key) in installState" :key="key" :style="stateRowStyle">
            <span :style="stateKeyStyle">{{ stateLabels[key] || key }}</span>
            <span v-if="key === 'system_version'" :style="stateVersionStyle">{{ val }}</span>
            <ApIcon v-else-if="val" name="CheckCircle" :size="16" :style="stateOkStyle()" />
            <ApIcon v-else name="XCircle" :size="16" :style="stateFailStyle()" />
          </div>
        </div>
        <ApButton variant="primary" size="large" :loading="loading.next" :style="{ width: '100%' }" @click="checkAndProceed">
          {{ $t('vAuthUi.k3') }}
        </ApButton>
      </div>

      <!-- Step 2: 初始化数据库 -->
      <div v-if="currentStep === 1" :style="stepContentStyle">
        <p :style="descStyle">{{ $t('vAuthUi.k4') }}</p>
        <ApButton variant="primary" size="large" :loading="loading.db" :style="{ width: '100%' }" @click="initDatabaseFunc">
          {{ $t('vAuthUi.k5') }}
        </ApButton>
        <p v-if="msg" :style="successStyle">{{ msg }}</p>
        <ApButton v-if="dbDone" variant="primary" size="large" :style="{ width: '100%' }" @click="currentStep = 2">{{ $t('vAuthUi.k3') }}</ApButton>
      </div>

      <!-- Step 3: 创建管理员 -->
      <div v-if="currentStep === 2" :style="stepContentStyle">
        <p :style="descStyle">{{ $t('vAuthUi.k6') }}</p>
        <ApInput
          v-model="adminForm.username"
          :label="$t('vAuthUi.k11')"
          :placeholder="$t('vAuthUi.k15')"
          @keyup.enter="initSystemFunc"
        />
        <ApInput
          v-model="adminForm.given_name"
          :label="$t('vAuthUi.k12')"
          :placeholder="$t('vAuthUi.k16')"
          @keyup.enter="initSystemFunc"
        />
        <ApInput
          v-model="adminForm.email"
          :label="$t('vAuthUi.k13')"
          :placeholder="$t('vAuthUi.k17')"
          type="email"
          @keyup.enter="initSystemFunc"
        />
        <ApInput
          v-model="adminForm.password"
          :label="$t('vAuthUi.k14')"
          type="password"
          :placeholder="$t('vAuthUi.k14')"
          @keyup.enter="initSystemFunc"
        />
        <ApButton variant="primary" size="medium" :loading="loading.admin" :style="{ width: '100%' }" @click="initSystemFunc">
          {{ $t('vAuthUi.k7') }}
        </ApButton>
      </div>

      <!-- 完成 -->
      <div v-if="currentStep === 3">
        <div :style="successBoxStyle">
          <div :style="successIconStyle">
            <!-- 40px 的描边必须反解（同页 16px 的那两枚由 lucide 默认 2 正好合档），否则成功页的勾比列表里的粗 2.5 倍 -->
            <ApIcon name="CheckCircle" :size="40" :color="cv('green')" />
          </div>
          <h2 :style="successTitleStyle">{{ $t('vAuthUi.k8') }}</h2>
          <p :style="successDescStyle">{{ $t('vAuthUi.k9') }}</p>
          <ApButton variant="primary" size="large" :style="{ width: '100%' }" @click="goLogin">{{ $t('vAuthUi.k10') }}</ApButton>
        </div>
      </div>
    </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, ref, shallowRef, reactive } from 'vue';
import { useRouter } from 'vue-router';
import { AppleZIndex, AppleFontWeight, AppleRadius, AppleSpace, AppleType, cv, AppleGlass, AppleLayout } from '@/utils/core/apple-tokens';
import { authCardPadding } from '@/styles/layout/page-styles';
import { siteBrandName } from '@/utils/core/site-brand';
import ApInput from '@/components/apple/ApInput.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApBrand from '@/components/apple/ApBrand.vue';
import { getInstallState, initDatabase, initSystem, isBackendHealthy } from '@/api/modules/auth/install';
import { validators, getValidationMessage } from '@/utils/core/validators';
import { useUIStore } from '@/stores/app/ui';

const router = useRouter();
const ui = useUIStore();
const currentStep = shallowRef(0);
const loading = reactive({ next: false, db: false, admin: false });
const msg = shallowRef('');
const dbDone = shallowRef(false);
const installState = ref<Record<string, unknown>>({});
const backendOnline = shallowRef(true);
const adminForm = ref({ username: '', given_name: '', email: '', password: '' });
const { t } = useI18n();

// 两处都必须是 computed：常量数组/对象在定义期只求值一次，
// 语言切换后向导步骤还留着中文 —— 而语言包里明明有英文。
const steps = computed(() => [t('vAuthUi.stepEnv'), t('vAuthUi.stepDb'), t('vAuthUi.stepAdmin'), t('vAuthUi.stepDone')]);

const stateLabels = computed<Record<string, string>>(() => ({
  db_connected: t('vAuthUi.stateDb'),
  tables_exist: t('vAuthUi.stateTables'),
  admin_exists: t('vAuthUi.stateAdmin'),
  system_version: t('vAuthUi.stateVersion') }));

function stateOkStyle() { return { color: cv('green'), fontWeight: String(AppleFontWeight.semibold) }; }
function stateFailStyle() { return { color: cv('red'), fontWeight: String(AppleFontWeight.semibold) }; }

async function checkAndProceed() {
  loading.next = true;
  backendOnline.value = await isBackendHealthy();
  if (!backendOnline.value) {
    ui.showToast(t('vAuthUi.k18'), 'error');
    loading.next = false;
    return;
  }
  try {
    const state = await getInstallState();
    // system_version 保留原字符串值，其余字段转为布尔值
    installState.value = Object.fromEntries(
      Object.entries(state).map(([key, value]) => [key, key === 'system_version' ? value : Boolean(value)])
    );
    if (!installState.value.db_connected) {
      ui.showToast(t('vAuthUi.k19'), 'error');
    } else if (installState.value.admin_exists) {
      ui.showToast(t('vAuthUi.k20'), 'error');
    } else {
      currentStep.value = 1;
    }
  } catch (e) {
    ui.showToast(t('vAuthUi.k21') + (e instanceof Error ? e.message : String(e)), 'error');
  } finally {
    loading.next = false;
  }
}

async function initDatabaseFunc() {
  loading.db = true;
  msg.value = '';
  try {
    await initDatabase();
    msg.value = t('sentUi.k46');
    dbDone.value = true;
  } catch (e) {
    ui.showToast(e instanceof Error ? e.message : String(e), 'error');
  } finally {
    loading.db = false;
  }
}

async function initSystemFunc() {
  if (!adminForm.value.username || !adminForm.value.password) {
    ui.showToast(t('vAuthUi.k22'), 'error');
    return;
  }
  if (!validators.username(adminForm.value.username)) {
    ui.showToast(getValidationMessage('username', adminForm.value.username), 'error');
    return;
  }
  if (adminForm.value.given_name && adminForm.value.given_name.length > 100) {
    ui.showToast(t('vAuthUi.k23'), 'error');
    return;
  }
  if (adminForm.value.email && !validators.email(adminForm.value.email)) {
    ui.showToast(getValidationMessage('email', adminForm.value.email), 'error');
    return;
  }
  if (!validators.password(adminForm.value.password)) {
    ui.showToast(getValidationMessage('password', adminForm.value.password), 'error');
    return;
  }
  loading.admin = true;
  try {
    await initSystem(adminForm.value);
    currentStep.value = 3;
  } catch (e) {
    ui.showToast(e instanceof Error ? e.message : String(e), 'error');
  } finally {
    loading.admin = false;
  }
}

function goLogin() {
  router.push('/login');
}

const wrapperStyle = { flex: '1', display: 'flex', alignItems: 'center', justifyContent: 'center' };

const cardStyle = { ...AppleGlass.standard, borderRadius: AppleRadius.card, padding: authCardPadding, width: '100%', maxWidth: AppleLayout.cardWide, boxShadow: `${cv('glass-edge')}, ${cv('shadow-subtle')}` };
const logoAreaStyle = { textAlign: 'center' as const, marginBottom: AppleSpace.lg, display: 'flex', flexDirection: 'column' as const, alignItems: 'center', gap: AppleSpace.md };
const titleStyle = { fontFamily: AppleType.cardProductTitle.font, fontSize: AppleType.sectionTitle.size, fontWeight: String(AppleType.cardProductTitle.weight), color: cv('text-primary'), margin: '0' };
const subtitleStyle = { fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.controlLabel.size, color: cv('text-tertiary'), margin: '0' };
const stepsStyle = { display: 'flex', justifyContent: 'space-between' as const, marginBottom: AppleSpace.lg, position: 'relative' as const };
const stepItemStyle = { display: 'flex', flexDirection: 'column' as const, alignItems: 'center', gap: AppleSpace.sm, flex: '1', position: 'relative' as const, zIndex: String(AppleZIndex.base) };
const stepNumStyle = (i: number) => ({ width: AppleSpace['2xl'], height: AppleSpace['2xl'], borderRadius: AppleRadius.circle, background: i <= currentStep.value ? cv('blue') : cv('border-light'), color: i <= currentStep.value ? cv('on-accent') : cv('text-quaternary'), display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), fontFamily: AppleType.bodyEmphasis.font });
const stepLabelStyle = (i: number) => ({ fontFamily: AppleType.microUI.font, fontSize: AppleType.caption.size, letterSpacing: AppleType.caption.letterSpacing, color: i <= currentStep.value ? cv('text-primary') : cv('text-quaternary'), fontWeight: i <= currentStep.value ? String(AppleFontWeight.semibold) : String(AppleFontWeight.regular) });
const stepContentStyle = { display: 'flex' as const, flexDirection: 'column' as const, gap: AppleSpace.md };
const stateCardStyle = { background: cv('surface-panel'), border: `1px solid ${cv('border-light')}`, borderRadius: AppleRadius.card, padding: AppleSpace.md };
const stateRowStyle = { display: 'flex', justifyContent: 'space-between', padding: `${AppleSpace.sm} 0`, fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.controlLabel.size };
const stateKeyStyle = { color: cv('text-secondary') };
const stateHintStyle = { color: cv('text-quaternary'), fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.controlLabel.size, padding: `${AppleSpace.sm} 0` };
const stateVersionStyle = { color: cv('text-primary'), fontWeight: String(AppleFontWeight.semibold), fontFamily: AppleType.bodyEmphasis.font };
const descStyle = { fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.controlLabel.size, color: cv('text-secondary'), margin: '0', textAlign: 'center' as const };
const successStyle = { color: cv('green'), fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.microUI.size, margin: '0', textAlign: 'center' as const };
const successBoxStyle = { textAlign: 'center' as const, display: 'flex', flexDirection: 'column' as const, alignItems: 'center', gap: AppleSpace.md, padding: `${AppleSpace.xl} 0` };
const successIconStyle = { width: AppleSpace['4xl'], height: AppleSpace['4xl'], borderRadius: AppleRadius.xl, background: cv('green-light'), display: 'flex', alignItems: 'center', justifyContent: 'center' };
const successTitleStyle = { fontFamily: AppleType.cardProductTitle.font, fontSize: AppleType.pageTitle.size, letterSpacing: AppleType.pageTitle.letterSpacing, fontWeight: String(AppleType.cardProductTitle.weight), color: cv('text-primary'), margin: '0' };
const successDescStyle = { fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.controlLabel.size, color: cv('text-secondary'), margin: '0' };
</script>
