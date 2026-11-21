<template>
  <aside data-glass-surface="sidebar" :style="sidebarStyle" role="navigation" :aria-label="$t('shell.k5')" @click.self="closeMenu">
    <div :class="collapsed ? 'ap-sidebar-collapsed-scroll' : ''" :style="navScrollStyle">

      <!-- 搜索框（2026-10-26 回滚：开关不放搜索行内 —— 用户明确否决，开关落顶栏最左恒定位） -->
      <div :style="searchBoxStyle" :title="collapsed ? $t('shell.k1') : undefined" @click="router.push('/search')">
        <Search :size="16" :style="{ color: cv('text-tertiary'), flexShrink: '0' }" aria-hidden="true" />
        <span v-if="!collapsed" :style="searchTextStyle">{{ $t('shell.k1') }}</span>
        <span v-if="!collapsed" :style="searchKbdStyle">{{ SEARCH_SHORTCUT_HINT }}</span>
      </div>

      <!-- 扁平导航 -->
      <nav :style="navSectionStyle" :aria-label="$t('shell.k6')">
        <ApButton
          v-for="item in mainNav"
          :key="item.nameKey"
          :variant="item.active ? 'tint' : 'ghost'"
          size="small"
          :aria-current="item.active ? 'page' : undefined"
          :style="navBtnStyle"
          :title="collapsed ? $t(item.nameKey) : undefined"
          @click="item.onClick"
        >
          <component :is="item.icon" :size="16" :aria-hidden="true" />
          <span v-if="!collapsed" :style="navLabelStyle" :title="$t(item.nameKey)">{{ $t(item.nameKey) }}</span>
        </ApButton>
      </nav>

      <!-- 工作区菜单组（收藏/协作/模板/同步等） -->
      <div :style="dividerStyle" />
      <nav :style="navSectionStyle" :aria-label="$t('shell.k7')">
        <ApButton
          :variant="socialGroupActive ? 'tint' : 'ghost'"
          size="small"
          :style="navBtnStyle"
          :title="collapsed ? (socialGroup?.label ?? socialGroupName) : undefined"
          @click="toggleAdminGroup(socialGroupName)"
        >
          <Users :size="16" aria-hidden="true" />
          <span v-if="!collapsed" :style="navLabelStyle" :title="socialGroup?.label ?? socialGroupName">{{ socialGroup?.label ?? socialGroupName }}</span>
          <ChevronDown
            v-if="!collapsed"
            :style="{ marginLeft: 'auto', transform: adminExpanded[socialGroupName] ? 'rotate(180deg)' : 'rotate(0)', transition: AppleTransition.fast, flexShrink: '0' }"
            :size="16"
          />
        </ApButton>
        <div v-if="!collapsed && adminExpanded[socialGroupName]" :style="subMenuStyle">
          <ApButton
            v-for="child in socialChildren"
            :key="child.name"
            :variant="child.active ? 'tint' : 'ghost'"
            size="small"
            :style="{ width: '100%', justifyContent: 'flex-start' }"
            @click="child.onClick"
          >
            <component :is="child.icon" :size="16" aria-hidden="true" />
            <span :style="navLabelStyle" :title="child.label">{{ child.label }}</span>
          </ApButton>
        </div>
      </nav>

      <!-- AI 菜单组（AI 启用时显示） -->
      <template v-if="aiStore.enabled">
      <div :style="dividerStyle" />
      <nav :style="navSectionStyle" :aria-label="$t('shell.k8')">
        <ApButton
          :variant="aiGroupActive ? 'tint' : 'ghost'"
          size="small"
          :style="navBtnStyle"
          title="AI"
          @click="toggleAdminGroup(aiGroupName)"
        >
          <Zap :size="16" aria-hidden="true" />
          <span v-if="!collapsed" :style="navLabelStyle" title="AI">AI</span>
          <ChevronDown
            v-if="!collapsed"
            :style="{ marginLeft: 'auto', transform: adminExpanded[aiGroupName] ? 'rotate(180deg)' : 'rotate(0)', transition: AppleTransition.fast, flexShrink: '0' }"
            :size="16"
          />
        </ApButton>
        <div v-if="!collapsed && adminExpanded[aiGroupName]" :style="subMenuStyle">
          <ApButton
            v-for="child in aiChildren"
            :key="child.name"
            :variant="child.active ? 'tint' : 'ghost'"
            size="small"
            :style="{ width: '100%', justifyContent: 'flex-start' }"
            @click="child.onClick"
          >
            <component :is="child.icon" :size="16" aria-hidden="true" />
            <span :style="navLabelStyle" :title="child.label">{{ child.label }}</span>
          </ApButton>
        </div>
      </nav>
      </template>

      <!-- 管理区（仅管理员） -->
      <template v-if="isAdmin">
        <div :style="dividerStyle" />
        <div v-if="!collapsed" :style="sectionLabelStyle">{{ $t('shell.k2') }}</div>
        <nav :style="navSectionStyle" :aria-label="$t('shell.k9')">
          <template v-for="group in adminGroups" :key="group.name">
            <ApButton
              :variant="group.active ? 'tint' : 'ghost'"
              size="small"
              :style="navBtnStyle"
              :title="collapsed ? group.label : undefined"
              @click="toggleAdminGroup(group.name)"
            >
              <component :is="group.icon" :size="16" aria-hidden="true" />
              <span v-if="!collapsed" :style="navLabelStyle" :title="group.label">{{ group.label }}</span>
              <ChevronDown
                v-if="!collapsed"
                :style="{ marginLeft: 'auto', transform: adminExpanded[group.name] ? 'rotate(180deg)' : 'rotate(0)', transition: AppleTransition.fast, flexShrink: '0' }"
                :size="16"
              />
            </ApButton>
            <div v-if="!collapsed && adminExpanded[group.name]" :style="subMenuStyle">
              <ApButton
                v-for="child in group.children"
                :key="child.name"
                :variant="child.active ? 'tint' : 'ghost'"
                size="small"
                :style="{ width: '100%', justifyContent: 'flex-start' }"
                @click="child.onClick"
              >
                <component :is="child.icon" :size="16" aria-hidden="true" />
                <span :style="navLabelStyle" :title="child.label">{{ child.label }}</span>
              </ApButton>
            </div>
          </template>
        </nav>
      </template>
    </div>

    <!-- 2026-10-26：收起/展开开关已挪到顶栏品牌右侧（ApNavBar）——这里原先是一行专属按钮：
         展开态拉满整行、收起态居中，两态位置跳且占一整块（用户报"不应该放在这里，占据太多空间"）。
         挪到顶栏后任何状态位置恒定，本块整体删除。开关状态与持久化仍在 useUIStore。 -->

    <!-- 底部用户区：data-ap-shell 是给运行时巡检的锚点。
         main.ts 的 mixin 只给组件根节点打 data-ap，这个 div 是裸元素，得自己标一个。 -->
    <div
      ref="userSectionRef"
      data-ap-shell="user-row"
      :style="userSectionStyle"
      role="button"
      tabindex="0"
      aria-haspopup="menu"
      :aria-expanded="showMenu"
      @click.stop="toggleMenu"
      @keydown.enter.prevent="toggleMenu"
      @keydown.space.prevent="toggleMenu"
      @keydown.esc="showMenu = false"
    >
      <ApAvatar
        :text="getAvatarText(auth.user)"
        size="small"
        :online="true"
      />
      <div v-if="!collapsed" :style="userInfoStyle">
        <div :style="userNameStyle">{{ getUserDisplayName(auth.user) }}</div>
        <div :style="userRoleStyle">{{ getRoleName(auth.user?.role_id) }}</div>
      </div>
      <MoreVertical v-if="!collapsed" :size="16" :style="{ color: cv('text-quaternary'), cursor: 'pointer' }" :aria-label="$t('shell.k10')" />

      <!-- 用户菜单 Teleport 到 body + fixed 定位：侧栏壳是 overflow:hidden（收起过渡要裁内容），
           原位绝对定位的菜单在收起态（64px）会被剪成竖条 —— 这正是"弹窗被截断"的根因。 -->
      <Teleport to="body">
        <div v-if="showMenu" ref="dropdownRef" class="apple-glass-layer" :style="dropdownStyle" role="menu">
          <div v-bind="keyboardClick(goToProfile, 'menuitem')" :style="dropdownItemStyle">
            <UserCircle :size="16" aria-hidden="true" />
            <span>{{ $t('shell.k3') }}</span>
          </div>
          <div :style="dropdownDividerStyle" />
          <div v-bind="keyboardClick(handleLogout, 'menuitem')" :style="dropdownItemStyle">
            <LogOut :size="16" aria-hidden="true" />
            <span>{{ $t('shell.k4') }}</span>
          </div>
        </div>
      </Teleport>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch, onMounted } from 'vue';
import { useRouter, useRoute } from 'vue-router';
import { Home, Users, Zap, Search, MoreVertical, UserCircle, LogOut, ChevronDown, PanelLeftClose, PanelLeftOpen } from 'lucide-vue-next';
import { AppleEasing, AppleFont, AppleFontWeight, AppleLayout, AppleLineHeight, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, durVar, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { useAuthStore } from '@/stores/auth/auth';
import { keyboardClick } from '@/composables/ui/useKeyboardClick';
import { useClickOutsideClose } from '@/composables/ui/useClickOutsideClose';
import { useAIStore } from '@/stores/ai/ai';
import { useUIStore } from '@/stores/app/ui';
import { useCollectionStore } from '@/stores/workspace/collection';
import { listStyle, scrollSingleAxis } from '@/styles/layout/page-styles';
import { getRoleName, SEARCH_SHORTCUT_HINT } from '@/utils/core/constants';
import { getUserDisplayName } from '@/types/user-helpers';
import { collectNavGroups } from '@/router/navigation';
import ApAvatar from '@/components/apple/ApAvatar.vue';
import ApButton from '@/components/apple/ApButton.vue';

const props = defineProps<{
  /** 收起态：64px 图标列（桌面端；移动端抽屉里不传，恒为展开） */
  collapsed?: boolean;
}>();

const collapsed = computed(() => props.collapsed === true);

const router = useRouter();
const route = useRoute();
const auth = useAuthStore();
const aiStore = useAIStore();
const uiStore = useUIStore();
const collectionStore = useCollectionStore();

const showMenu = shallowRef(false);
const userSectionRef = ref<HTMLElement | null>(null);
const dropdownRef = ref<HTMLElement | null>(null);
/** 打开菜单瞬间用户区的视口几何（Teleport 到 body 后按它做 fixed 定位；bottomGap = 视口高 - 用户区顶边） */
const menuAnchor = shallowRef<{ left: number; width: number; bottomGap: number } | null>(null);
const adminExpanded = ref<Record<string, boolean>>({});

// 菜单 Teleport 到 body 了，"自己人"除了用户区还要算上浮层本身，
// 否则 pointerdown capture 会在 click 处理前把菜单卸载掉（菜单点不动）。
useClickOutsideClose(
  () => showMenu.value,
  () => { showMenu.value = false; },
  () => [userSectionRef.value, dropdownRef.value],
);

const currentPath = computed(() => route.path);
const isAdmin = computed(() => auth.isAdmin);

onMounted(() => {
  try {
    collectionStore.loadCollections({ page: 1, page_size: 20 });
  } catch {
    // 静默处理，避免侧边栏加载失败影响整体使用
  }
});

const mainNav = computed(() => [
  { nameKey: 'nav.home', icon: Home, active: currentPath.value === '/', onClick: () => router.push('/') },
]);

// ─── 导航单一事实源 ─────────────────────────────────────────────
// 菜单树从路由 meta（group/icon/title）派生，见 router/navigation.ts。
// 此前这里硬编码整套菜单，与路由表、面包屑三头维护，出现过
// "/ai/knowledge-graph 死链"和面包屑父级错挂兄弟页面的问题。
const navGroups = computed(() => collectNavGroups(router, currentPath.value, isAdmin.value));

const socialGroupName = '工作区'; // 原名"社交"名不副实：收藏/模板/协作/Git 同步都是工作区能力
const socialGroup = computed(() => navGroups.value.get(socialGroupName));
const socialGroupActive = computed(() => socialGroup.value?.active ?? false);
const socialChildren = computed(() => socialGroup.value?.children ?? []);

const aiGroupName = 'AI';
const aiGroup = computed(() => navGroups.value.get(aiGroupName));
const aiGroupActive = computed(() => currentPath.value.startsWith('/ai'));
const aiChildren = computed(() => aiGroup.value?.children ?? []);

const adminGroupNames = ['数据', '用户', '内容', '配置'] as const;
const adminGroups = computed(() =>
  adminGroupNames
    .map((name) => navGroups.value.get(name))
    .filter((group): group is NonNullable<typeof group> => Boolean(group))
);

function toggleAdminGroup(name: string) {
  adminExpanded.value[name] = !adminExpanded.value[name];
  if (adminExpanded.value[name]) {
    if (name === socialGroupName) {
      if (socialChildren.value[0]) socialChildren.value[0].onClick();
      return;
    }
    if (name === aiGroupName) {
      if (aiChildren.value[0]) aiChildren.value[0].onClick();
      return;
    }
    const group = adminGroups.value.find(g => g.name === name);
    if (group?.children?.[0]) group.children[0].onClick();
  }
}

function autoExpandMenu() {
  if (socialGroupActive.value) adminExpanded.value[socialGroupName] = true;
  // P1-2：AI 设置页自带 31 项二级导航，外层侧栏再展开 3 个子项会让两列导航同时高亮、
  // 并排吃掉 440px 横向空间（页头已有「知识图谱 / 返回对话」快捷入口）。
  // 设置页内把外层 AI 组收成一行；AI 对话 / 图谱页仍照常展开。
  if (aiGroupActive.value) adminExpanded.value[aiGroupName] = !currentPath.value.startsWith('/ai/settings');
  if (!isAdmin.value) return;
  for (const group of adminGroups.value) {
    if (group.active) adminExpanded.value[group.name] = true;
  }
}

watch(() => route.path, () => { autoExpandMenu(); }, { immediate: true });

function getAvatarText(user: { username: string; given_name?: string } | null) {
  if (!user) return '?';
  return (user.given_name || user.username).slice(0, 1).toUpperCase();
}

function toggleMenu() {
  if (!showMenu.value) {
    const rect = userSectionRef.value?.getBoundingClientRect();
    if (rect) {
      menuAnchor.value = {
        left: rect.left,
        width: rect.width,
        bottomGap: window.innerHeight - rect.top,
      };
    }
  }
  showMenu.value = !showMenu.value;
}
function closeMenu() { showMenu.value = false; }
function goToProfile() { closeMenu(); router.push('/profile'); }
function handleLogout() { closeMenu(); auth.logout(); router.push('/login'); }

const sidebarStyle = computed(() => ({
  width: collapsed.value ? AppleLayout.sidebarCollapsed : AppleLayout.sidebarWidth,
  minWidth: collapsed.value ? AppleLayout.sidebarCollapsed : AppleLayout.sidebarWidth,
  // 刻意不加 width 过渡：G17/G27 只准动 transform/opacity，收起是低频操作，瞬时切换即可
  height: '100%',
  // 侧栏**不再**从 y=0 起（旧注释那句"顶栏背后那 48px 是侧栏自己的壳面，这正是 macOS 工具栏
  // 压住侧栏顶端的样子"是错的）：顶栏是一整条浮层壳，它自己也铺 chrome，两者叠起来就是
  // 左上角两份玻璃 —— 0.28 压 0.28，品牌区比右侧亮 30+ 级，x=240 还留一条竖缝。
  // 现在让位写在布局壳上（MainLayout 的 sidebarShellStyle 给 padding-top 一个壳高），
  // 这里只留内容自己的呼吸间距：两处相加仍是原来的 64px，侧栏内容位置不变，
  // 但顶栏那 48px 只透环境层（一份材质）。移动端抽屉里侧栏在抽屉标题之下，同样不该再让。
  boxSizing: 'border-box' as const,
  backgroundColor: cv('surface-chrome'),
  // **不铺 `glass-specular`**（2026-11-01 七轮，用户判「不自然」）。
  // 那枚令牌是"贴着盒子上缘的斜切面"：顶栏/底栏的盒子顶端就是外壳的边，受光合理；
  // 侧栏的顶端是**与顶栏的接缝**（见 MainLayout 的 sidebarShellStyle），光从上方照下来时
  // 那里是被顶栏挡住的暗缝，不是受光边。铺上去等于在顶栏下沿又画一条 32px 亮带，
  // 实测侧栏顶端比它自己中段高 25~29 级 —— 读起来是"光从下面打上来"。
  // 侧栏的玻璃信号改由**透过来的环境明暗**承担（环境层顶部有光、往下变暗），
  // 加上边上的亮棱线与侧向软影；这比一层画上去的高光更接近"同一份材质"。
  // 侧栏身后永远没有内容滚过去，读成玻璃只能靠**边**：亮棱线 + 一圈内高光。
  // 之前这里是一条 1px 中性 `border`，深色下和内容区的发丝边长得一样，材质读不出来。
  borderRight: `0.5px solid ${cv('glass-rim')}`,
  // 棱线（内）+ 侧向软影（外）：侧栏与内容之间那条明度台阶靠阴影摊开才有前后关系。
  // 两枚令牌拼列表，非玻璃材质下 side-shadow 解析成零尺寸投影（不是 none，否则整条失效）。
  boxShadow: `${cv('glass-edge')}, ${cv('glass-side-shadow')}`,
  // 静止档与顶栏同深度（blur-panel）：侧栏身后只有那张固定的环境渐变，没有内容从它底下滚过，
  // 16px 与 8px 在静态底上肉眼分不出，却白付一倍半径 —— 壳层的玻璃语言由单边亮段与侧向软影承担。
  backdropFilter: cv('blur-panel'), WebkitBackdropFilter: cv('blur-panel'),
  display: 'flex', flexDirection: 'column' as const,
  // 镜面环是绝对定位的 ::after（见 apple-tokens.ts 的镜面环规则），需要侧栏自己是
  // 定位上下文 —— 否则伪元素会挂到更外层的壳上。菜单本身是 fixed，不受影响。
  position: 'relative' as const,
  // 收起态去掉左右 padding：64px 列里所有条目自己居中，否则 sm/md 的不对称
  // 内边距会把放大镜、图标推离中线（用户报的"不对齐，尤其搜索放大镜"）
  padding: collapsed.value
    ? `${AppleSpace.md} 0 0 0`
    : `${AppleSpace.md} ${AppleSpace.sm} 0 ${AppleSpace.sm}`,
  overflow: 'hidden' as const,
}));

/** 顶部导航条目的按钮样式：展开时左对齐（icon+文字），收起时图标居中成 64px 列 */
const navBtnStyle = computed(() => ({
  width: '100%',
  justifyContent: collapsed.value ? ('center' as const) : ('flex-start' as const),
  // 收起时把 ApButton 自身的水平 padding 归零，图标几何中心才与列中线重合
  ...(collapsed.value ? { paddingLeft: 0, paddingRight: 0 } : {}),
}));

/** 导航滚动区：收起态去掉 scrollbarGutter 并隐藏滚动条 —— stable 槽 + 8px 滚动条
 * 会把 64px 列的内容区压到 56px，width:100% 的搜索框在偏窄区里居中，整列视觉左偏
 * （用户报的"搜索框左右占据不一致"）。滚轮滚动不受影响。 */
const navScrollStyle = computed(() => ({
  flex: '1 1 auto',
  ...(collapsed.value
    ? { overflowY: 'auto' as const, overflowX: 'hidden' as const, scrollbarGutter: 'auto' as const, scrollbarWidth: 'none' as const }
    : scrollSingleAxis),
  minHeight: '0', paddingBottom: AppleSpace.sm,
}));

/**
 * 导航条目里那一行文字。三个属性是一件事：`minWidth: 0` 让它在 flex 行里真的能收缩
 * （flex 项默认 min-width:auto，收缩到内容最小宽度就顶住了），后两个把溢出的部分收成省略号。
 * 缺了它 = 条目溢出到滚动容器外面 = 上面那条横向条。
 * 全站同一写法已由 ChatSidebar 的 infoStyle + titleStyle 立过，这里是把侧栏并过去。
 * 截断必须配 `:title`（全名要可达）—— check:geom 有一条"截断且无 title/tooltip"的断言。
 */
const navLabelStyle = {
  flex: '1',
  minWidth: 0,
  textAlign: 'left' as const,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const searchBoxStyle = computed(() => ({
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm,
  // 收起态：胶囊只包住放大镜（自适应宽 + alignSelf 居中），与下方图标按钮同一视觉宽度档；
  // 全宽底色块在 64px 列里会顶满左右边缘
  ...(collapsed.value
    ? { justifyContent: 'center' as const, padding: `${AppleSpace.sm}`, alignSelf: 'center' as const }
    : { padding: `${AppleSpace.sm} ${AppleSpace.md}` }),
  borderRadius: AppleRadius.card,
  background: cv('bg-tertiary'), cursor: 'pointer',
  marginBottom: AppleSpace.md, transition: AppleTransition.fast,
}));

const searchTextStyle = {
  flex: '1', fontFamily: AppleType.bodyPrimary.font, fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'), textAlign: 'left' as const,
};

const searchKbdStyle = {
  fontFamily: AppleType.microUI.font, fontSize: AppleType.legalMeta.size,
  color: cv('text-quaternary'), padding: `2px ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small, background: cv('bg-secondary'),
  border: `1px solid ${cv('border-light')}`,
};

const navSectionStyle = listStyle(AppleSpace.xs);

const subMenuStyle = {
  paddingLeft: AppleSpace.lg, marginTop: AppleSpace.xs, marginBottom: AppleSpace.xs,
  display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs,
};

const sectionLabelStyle = {
  fontFamily: AppleType.microUI.font, fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleType.microUI.weight),
  color: cv('text-tertiary'), padding: `${AppleSpace.md} ${AppleSpace.md} ${AppleSpace.sm}`,
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
};

const dividerStyle = {
  height: '1px', background: cv('border-light'), margin: `${AppleSpace.sm} 0`,
};

const userSectionStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  // 与顶栏、页脚同高（壳层横条标尺）。原先用 padding 凑出来是 49px，三条边线对不齐。
  height: AppleLayout.shellBarHeight,
  flexShrink: 0,
  // 收起态：头像在 64px 列里居中，左右 padding 归零
  ...(collapsed.value
    ? { justifyContent: 'center' as const, padding: '0' }
    : { padding: `0 ${AppleSpace.sm}` }),
  borderTop: `1px solid ${cv('border-light')}`,
  marginTop: 'auto',
  cursor: 'pointer',
  position: 'relative' as const,
}));

const userInfoStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  justifyContent: 'center',
  height: AppleLayout.avatarSizeSmall,
  flex: '1',
  minWidth: 0,
  overflow: 'hidden' as const,
};

const userNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
  lineHeight: AppleLineHeight.display,
};

const userRoleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
  lineHeight: AppleLineHeight.display,
};

const dropdownStyle = computed(() => {
  const anchor = menuAnchor.value;
  if (!anchor) return { display: 'none' as const };
  return {
    position: 'fixed' as const,
    left: `${anchor.left}px`,
    // 贴用户区上沿：fixed bottom = 视口高 - 用户区顶边（G25 不认 translate(±100%) 的浮层定位）
    bottom: `${anchor.bottomGap}px`,
    // 收起态下侧栏只有 64px，跟着收缩会把菜单压成竖条 —— 宽度对齐展开态侧栏宽（令牌）
    width: collapsed.value ? AppleLayout.sidebarWidth : `${anchor.width}px`,
    ...menuSurfaceStyle(),
    padding: `${AppleSpace.xs} 0`,
  };
});

const dropdownItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  cursor: 'pointer',
  transition: AppleTransition.smooth,
};

const dropdownDividerStyle = {
  height: '1px',
  background: cv('border-light'),
  margin: `${AppleSpace.xs} 0`,
};
</script>

<style scoped>
/* 收起态 64px 列隐藏滚动条（Chromium 侧；Firefox 走内联 scrollbarWidth:'none'）。
   全局滚动条 8px 会把图标列往左挤——见 navScrollStyle 注释。滚轮滚动不受影响。 */
.ap-sidebar-collapsed-scroll::-webkit-scrollbar {
  width: 0;
  height: 0;
}
</style>
