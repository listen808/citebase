<template>
  <div :style="aiSubPageStyle">
    <!-- MCP 服务器列表 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Wrench" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiView2.k15') }}</span>
        </div>
      </template>

      <ApSkeleton v-if="loading" :lines="3" />
      <!-- P2-17：无数据统一用 Inbox，功能符号（扳手/指纹）留给导航与标题 -->
      <ApEmpty v-else-if="servers.length === 0" icon="Inbox" :title="$t('aiView2.k26')" :description="$t('vAiUi.k35')" />

      <div v-else :style="compactServerListStyle">
        <div v-for="server in servers" :key="server.name" :style="aiSubCardStyle">
          <div :style="compactServerHeaderStyle">
            <div :style="compactServerNameRowStyle">
              <span :style="statusDotStyle(server.connected)" />
              <span :style="serverNameStyle">{{ server.name }}</span>
              <span :style="transportBadgeStyle">{{ server.transport }}</span>
            </div>
            <ApButton variant="ghost" size="small" :tooltip="$t('aiView2.k27')" :aria-label="$t('aiView2.k27')" icon-only @click="onRemove(server.name)">
              <ApIcon name="Trash2" :size="16" />
            </ApButton>
          </div>
          <div v-if="server.tool_count > 0" :style="compactServerInfoStyle">
            <ApIcon name="Wrench" :size="12" :color="cv('text-secondary')" />
            <span>{{ server.tool_count }} {{ $t('vAiUi.k38') }}</span>
            <ApButton
              variant="ghost"
              size="small"
              :aria-expanded="expandedServers[server.name] ? 'true' : 'false'"
              @click="toggleTools(server.name)"
            >
              {{ expandedServers[server.name] ? $t('vAiUi.collapse') : $t('vAiUi.k37') }}
            </ApButton>
          </div>
          <!-- 工具清单此前只在后端有 /ai/mcp/tools，页面上只报个数：
               配好 MCP 却看不见 Agent 到底能调用什么，出问题时只能猜 -->
          <div v-if="expandedServers[server.name]" :style="toolListStyle">
            <span v-if="toolsLoading" :style="toolHintStyle">{{ $t('aiView2.k16') }}</span>
            <span v-else-if="tools === null" :style="toolHintStyle">{{ $t('aiView2.k17') }}</span>
            <span v-else-if="toolsOf(server.name).length === 0" :style="toolHintStyle">
              {{ $t('aiView2.k18') }}
            </span>
            <template v-else>
              <div v-for="tool in toolsOf(server.name)" :key="server.name + '/' + tool.name" :style="toolItemStyle">
                <span :style="toolNameStyle">{{ tool.displayName }}</span>
                <span :style="toolDescStyle">{{ tool.description || $t('fallback.k69') }}</span>
              </div>
            </template>
          </div>
          <div v-if="server.error" :style="compactErrorStyle">
            <ApIcon name="AlertTriangle" :size="12" :color="cv('red')" />
            <span>{{ server.error }}</span>
          </div>
        </div>
      </div>
    </ApCard>

    <ApConfirmModal
      v-model="showRemoveConfirm"
      :title="$t('aiView2.k27')"
      :message="$t('uiCopy.confirmRemoveMcp', { p1: pendingRemoveName })"
      :confirm-text="$t('aiView2.k28')"
      type="danger"
      @confirm="confirmRemove"
    />

    <!-- 添加 MCP 服务器 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Plus" :size="16" :color="cv('green')" />
          <span :style="sectionTitleStyle">{{ $t('aiView2.k19') }}</span>
        </div>
      </template>
      <div :style="compactFormGridStyle">
        <ApInput v-model="form.name" :label="$t('aiView2.name')" placeholder="my-mcp-server" />
        <ApSelect
          :model-value="form.transport"
          :options="transportOptions"
          :label="$t('aiView2.k22')"
          @update:model-value="(v) => form.transport = v as string"
        />
        <template v-if="form.transport === 'stdio'">
          <ApInput v-model="form.command" :label="$t('aiView2.k23')" placeholder="npx -y @modelcontextprotocol/server-filesystem" />
          <ApInput v-model="form.argsText" :label="$t('aiView2.k24')" placeholder="/path/to/dir" />
        </template>
        <template v-if="form.transport === 'sse' || form.transport === 'streamable_http'">
          <ApInput
            v-model="form.endpoint"
            :label="$t('aiView2.k25')"
            :placeholder="form.transport === 'sse' ? 'http://localhost:3000/sse' : 'http://localhost:3000/mcp'"
          />
        </template>
      </div>
      <div :style="compactActionRowStyle">
        <ApButton variant="primary" size="small" :loading="adding" @click="handleAdd">
          <ApIcon name="Plus" :size="16" /> {{ $t('aiView2.k20') }}
        </ApButton>
        <div v-if="addResult" :style="resultStyle(addResult.success)">
          <ApIcon v-if="addResult.success" name="Check" :size="14" :color="cv('green')" />
          <ApIcon v-else name="AlertTriangle" :size="14" :color="cv('red')" />
          {{ addResult.message }}
        </div>
      </div>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, computed, onMounted } from 'vue';
// Wrench 与二级导航「MCP 服务器」同符号：卡片头由 Server、空态由 Settings 统一到 Wrench
// （Server 已从 import 移除，避免留下未用的 lucide 组件）。
import ApIcon from '@/components/apple/ApIcon.vue';






import { cv, AppleFontWeight, AppleSpace, AppleType, cssType } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, aiSubSectionHeaderGapStyle, aiSubSectionTitleStyle, aiSubCardStyle, aiSubCardHeaderStyle, aiSubCardNameRowStyle, aiSubCardNameStyle, aiSubCardInfoStyle, aiSubBadgeStyle, aiSubServerErrorStyle, aiSubFormGridStyle, aiSubActionRowStyle, aiSubResultStyle, aiSubStatusDotStyle } from '@/styles/ai/ai-page-styles';
import { listMCPServers, addMCPServer, removeMCPServer, listMCPTools, type MCPServerInfo } from '@/api/modules/ai/ai-config';
import { useUIStore } from '@/stores/app/ui';

interface MCPTool {
  name: string;
  description: string;
  parameters: unknown;
  /** 后端把工具名统一加 `server:tool` 前缀以避免跨服务器重名，展示时剥掉前缀 */
  displayName: string;
}

const tools = ref<MCPTool[] | null>(null);
const toolsLoading = ref(false);
const expandedServers = ref<Record<string, boolean>>({});

async function loadTools() {
  if (tools.value !== null || toolsLoading.value) return;
  toolsLoading.value = true;
  try {
    const res = await listMCPTools();
    tools.value = (res?.tools ?? []).map((t) => ({
      ...t,
      displayName: t.name.includes(':') ? t.name.slice(t.name.indexOf(':') + 1) : t.name,
    }));
  } catch (e) {
    logger.error('Failed to fetch MCP tools:', e);
    tools.value = null;
  } finally {
    toolsLoading.value = false;
  }
}

function toolsOf(serverName: string) {
  return (tools.value ?? []).filter((t) => t.name.startsWith(serverName + ':'));
}

function toggleTools(serverName: string) {
  expandedServers.value = { ...expandedServers.value, [serverName]: !expandedServers.value[serverName] };
  if (expandedServers.value[serverName]) void loadTools();
}

const ui = useUIStore();
const { isMobile } = useBreakpoint();
const loading = ref(true);
const servers = ref<MCPServerInfo[]>([]);
const adding = ref(false);
const addResult = ref<{ success: boolean; message: string } | null>(null);

const form = ref({
  name: '',
  transport: 'stdio',
  command: '',
  argsText: '',
  endpoint: '',
});

const transportOptions = [
  { label: t('aiView2.k29'), value: 'stdio' },
  { label: 'SSE (HTTP)', value: 'sse' },
  { label: 'Streamable HTTP', value: 'streamable_http' },
];

async function fetchServers() {
  loading.value = true;
  try {
    const res = await listMCPServers();
    servers.value = res.servers || [];
  } catch (e) {
    logger.error('Failed to fetch MCP servers:', e);
    ui.showToast(t('aiView2.k30'), 'error');
  } finally {
    loading.value = false;
  }
}

async function handleAdd() {
  if (!form.value.name) return;
  adding.value = true;
  addResult.value = null;
  try {
    const res = await addMCPServer({
      name: form.value.name,
      transport: form.value.transport,
      command: form.value.command,
      args: form.value.argsText ? form.value.argsText.split(',').map(s => s.trim()).filter(Boolean) : [],
      endpoint: form.value.endpoint || undefined,
    });
    addResult.value = { success: true, message: t('sentUi.k38', { p1: res.tool_count }) };
    ui.showToast(t('aiView2.k31'), 'success');
    form.value = { name: '', transport: 'stdio', command: '', argsText: '', endpoint: '' };
    await fetchServers();
  } catch (e: unknown) {
    addResult.value = { success: false, message: (e as { message?: string }).message || t('fallback.k104') };
    ui.showToast(t('aiView2.k32'), 'error');
  } finally { adding.value = false; }
}

const showRemoveConfirm = ref(false);
const pendingRemoveName = ref('');
const removing = ref(false);

function onRemove(name: string) {
  pendingRemoveName.value = name;
  showRemoveConfirm.value = true;
}

async function confirmRemove() {
  const name = pendingRemoveName.value;
  if (!name || removing.value) return;
  removing.value = true;
  try {
    await removeMCPServer(name);
    ui.showToast(t('aiView2.k33'), 'success');
    await fetchServers();
  } catch (e) {
    logger.error('Failed to remove MCP server:', e);
    ui.showToast(t('aiView2.k34'), 'error');
  } finally {
    removing.value = false;
  }
}

onMounted(() => fetchServers());

const sectionTitleStyle = aiSubSectionTitleStyle;

const statusDotStyle = aiSubStatusDotStyle;
const serverNameStyle = aiSubCardNameStyle;
const transportBadgeStyle = aiSubBadgeStyle;
const resultStyle = aiSubResultStyle;

const compactServerListStyle = {
  display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm,
};
const compactServerHeaderStyle = { ...aiSubCardHeaderStyle, marginBottom: 0 };
const compactServerNameRowStyle = { ...aiSubCardNameRowStyle, gap: AppleSpace.xs };
const compactServerInfoStyle = { ...aiSubCardInfoStyle, marginTop: AppleSpace.xs, gap: AppleSpace.xxs };
const compactErrorStyle = { ...aiSubServerErrorStyle, marginTop: AppleSpace.xs, gap: AppleSpace.xxs };
const compactFormGridStyle = computed(() => isMobile.value ? { ...aiSubFormGridStyle, gridTemplateColumns: '1fr', gap: AppleSpace.sm, marginBottom: AppleSpace.sm } : { ...aiSubFormGridStyle, gap: AppleSpace.sm, marginBottom: AppleSpace.sm });
const compactActionRowStyle = { ...aiSubActionRowStyle, gap: AppleSpace.sm };

const toolListStyle = {
  marginTop: AppleSpace.xxs,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};
const toolHintStyle = { color: cv('text-tertiary') };
const toolItemStyle = { display: 'flex', flexDirection: 'column' as const };
const toolNameStyle = { ...cssType(AppleType.microUI), fontWeight: String(AppleFontWeight.medium), color: cv('text-primary') };
const toolDescStyle = { ...cssType(AppleType.microUI), color: cv('text-secondary') };
</script>
