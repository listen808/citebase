import { computed, ref, shallowRef, watch, onMounted, nextTick } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useDocumentStore } from '@/stores/workspace/document';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import { useHistoryStore } from '@/stores/workspace/history';
import { useAuthStore } from '@/stores/auth/auth';
import { useBackNavigation } from '@/composables/ui/useBackNavigation';
import { getDocumentPath } from '@/api/modules/document/document';
import { ApiError } from '@/api/client';
import { logger } from '@/utils/logger';
import type { Document } from '@/types/models';
import i18n from '@/i18n';

const { t } = i18n.global;

function useAbortController() {
  let controller: AbortController | null = null;

  function create(): AbortSignal {
    if (controller) controller.abort();
    controller = new AbortController();
    return controller.signal;
  }

  function isAborted(signal: AbortSignal | undefined): boolean {
    return signal?.aborted ?? false;
  }

  function clear(signal: AbortSignal): void {
    if (controller?.signal === signal) {
      controller = null;
    }
  }

  function abort(): void {
    if (controller) {
      controller.abort();
      controller = null;
    }
  }

  return { create, isAborted, clear, abort };
}

function buildBreadcrumbs(pathRes: { id: number; name: string }[] | null, spaceName?: string): { id: number; name: string }[] {
  if (!pathRes || pathRes.length === 0) return [];
  let items = pathRes.map((p) => ({ id: p.id, name: p.name }));
  if (spaceName && items.length > 0 && items[0]!.name === spaceName) {
    items = items.slice(1);
  }
  return items;
}

function findSiblingDoc(tree: Document[], deletedId: number): Document | null {
  function search(nodes: Document[], parent: Document | null): { parent: Document | null; siblings: Document[]; idx: number } | null {
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i];
      if (!node) continue;
      if (node.id === deletedId) return { parent, siblings: nodes, idx: i };
      if (node.children?.length) {
        const found = search(node.children, node);
        if (found) return found;
      }
    }
    return null;
  }
  const result = search(tree, null);
  if (!result) return null;
  const { siblings, idx } = result;
  if (siblings.length > 1) return siblings[idx + 1] ?? siblings[idx - 1] ?? null;
  return result.parent;
}

interface UseSpaceInitOptions {
  forceExpandFirstLevel?: Ref<boolean>;
}

export function useSpaceInit(options?: UseSpaceInitOptions) {
  const route = useRoute();
  const router = useRouter();
  const docStore = useDocumentStore();
  const spaceStore = useSpaceStore();
  const ui = useUIStore();
  const historyStore = useHistoryStore();
  const auth = useAuthStore();
  const { goBack } = useBackNavigation({
    onRestoreState: (state) => {
      if (state.expandedNodes) {
        docStore.expandedNodes = new Set(state.expandedNodes);
      }
      if (state.sidebarCollapsed !== undefined) {
        ui.sidebarCollapsed = state.sidebarCollapsed;
      }
    },
  });
  const abortCtrl = useAbortController();

  const canEditInSpace = computed(() => auth.isEditor(spaceStore.currentSpace?.privilege));

  let pendingLoadDocumentId: number | null = null;
  let loadDocVersion = 0;

  const spaceId = computed(() => Number(route.params.spaceId));
  const docId = computed(() => Number(route.params.docId));
  const selectedDocumentId = shallowRef<number | null>(null);

  const breadcrumbs = ref<{ id: number; name: string }[]>([]);
  const contentRef = ref<HTMLElement | null>(null);
  // P2-7：加载失败只弹一次 toast，toast 消失后页面只剩空树且没有重试入口
  const initError = ref<'' | 'tree' | 'space' | 'both'>('');

  function handleBack() {
    goBack();
  }

  async function initSpace(sid: number) {
    if (!sid) return;
    const signal = abortCtrl.create();

    const historyState = historyStore.getCurrentState();
    const hasDocInUrl = !!route.params.docId;
    const hasHistoryDoc = !!historyState?.selectedDocumentId;

    // 设置空间初始化状态，避免显示空间根页面
    docStore.spaceInitializing = true;
    initError.value = '';

    // 只有在确定不会立即加载文档时才清空当前文档
    // 避免进入空间时短暂显示空间根页面（SpaceRootContent）
    if (!hasDocInUrl && !hasHistoryDoc) {
      docStore.currentDocument = null;
      docStore.currentContent = '';
    }
    breadcrumbs.value = [];

    const forceExpand = options?.forceExpandFirstLevel?.value ?? false;

    try {
      const results = await Promise.allSettled([
        docStore.loadRootDocuments(sid, false, forceExpand),
        spaceStore.loadSpace(sid),
      ]);
      const [treeRes, spaceRes] = results;
      if (treeRes?.status === 'rejected') {
        logger.error('Failed to load root documents', treeRes.reason);
        ui.showToast(t('tsUi.k32'), 'warning');
      }
      if (spaceRes?.status === 'rejected') {
        logger.error('Failed to load space', spaceRes.reason);
        ui.showToast(t('tsUi.k33'), 'warning');
      }
      const treeFailed = treeRes?.status === 'rejected';
      const spaceFailed = spaceRes?.status === 'rejected';
      initError.value = treeFailed && spaceFailed ? 'both' : treeFailed ? 'tree' : spaceFailed ? 'space' : '';
      if (abortCtrl.isAborted(signal)) return;

      // 先确定 targetDocId 并预填 currentDocument（基于树节点）
      // 必须在 spaceInitializing=false 之前完成，避免 await 期间 currentDocument=null 触发 SpaceRootContent 闪现
      let targetDocId: number | null = null;
      if (route.params.docId) {
        targetDocId = Number(route.params.docId);
      } else if (route.query.root === '1') {
        targetDocId = null;
      } else if (historyState?.selectedDocumentId) {
        targetDocId = historyState.selectedDocumentId;
      } else if (docStore.tree.length > 0 && docStore.tree[0]) {
        targetDocId = docStore.tree[0].id;
      }

      if (targetDocId) {
        selectedDocumentId.value = targetDocId;
        const treeNode = docStore.findNodeById(targetDocId);
        // 已持有该文档完整数据（如从编辑器保存后跳回）时不要覆盖，
        // 否则树节点占位数据会丢掉 update_time/editor_name 等元数据
        if (treeNode && docStore.currentDocument?.id !== targetDocId) {
          docStore.currentDocument = {
            id: treeNode.id, name: treeNode.name, type: treeNode.type,
            space_id: treeNode.space_id, parent_id: treeNode.parent_id,
            has_children: treeNode.has_children,
          } as Document;
        }
      }

      // currentDocument 已预填（若有 targetDocId），可安全关闭 spaceInitializing
      // 不会触发 SpaceRootContent 闪现，后续 loadDocument 仍由 contentLoading 控制
      docStore.spaceInitializing = false;

      if (historyState?.expandedNodes?.length) {
        const validIds = new Set(docStore.getAllNodeIds(docStore.tree));
        const validExpanded = historyState.expandedNodes.filter((id: number) => validIds.has(id));
        if (validExpanded.length > 0) {
          const currentExpanded = new Set(docStore.expandedNodes);
          validExpanded.forEach((id: number) => currentExpanded.add(id));
          docStore.expandedNodes = currentExpanded;
          const toLoad = validExpanded.filter((id: number) => !docStore.loadedNodes.has(id));
          await Promise.allSettled(toLoad.map((id: number) => docStore.loadChildren(id)));
        }
      }
      if (abortCtrl.isAborted(signal)) return;

      if (targetDocId) {
        if (abortCtrl.isAborted(signal)) return;
        await Promise.all([loadDocument(targetDocId), docStore.expandToDocument(targetDocId)]);
      }

      if (!abortCtrl.isAborted(signal) && pendingLoadDocumentId !== null) {
        const pendingId = pendingLoadDocumentId;
        pendingLoadDocumentId = null;
        if (pendingId !== targetDocId) await loadDocument(pendingId);
      }
    } finally {
      // spaceInitializing 已在文档树加载完成后提前设为 false
      // 这里作为安全兜底，防止异常情况遗漏
      if (docStore.spaceInitializing) {
        docStore.spaceInitializing = false;
      }
      abortCtrl.clear(signal);
    }
  }

  /** 失败横幅上的「重试」：直接重跑同一次初始化 */
  function retryInit() {
    return initSpace(spaceId.value);
  }

  function mountInit(openCreateModalWrapper: (parentId: number, sId: number, type: 1 | 2) => void) {
    onMounted(() => {
      initSpace(spaceId.value);
      if (route.query.createDoc === 'true') {
        nextTick(() => {
          openCreateModalWrapper(0, spaceId.value, 1);
          router.replace({ path: route.path, query: {} });
        });
      }
    });

    watch(spaceId, (sid) => { if (sid) initSpace(sid); });

    watch(() => route.query.createDoc, (val) => {
      if (val === 'true') {
        openCreateModalWrapper(0, spaceId.value, 1);
        router.replace({ path: route.path, query: {} });
      }
    });
  }

  async function loadDocument(id: number) {
    const version = ++loadDocVersion;
    selectedDocumentId.value = id;
    try {
      // 优先使用缓存路径立即更新面包屑
      const cachedPath = docStore.getCachedPath(id);
      if (cachedPath) {
        breadcrumbs.value = buildBreadcrumbs(cachedPath, spaceStore.currentSpace?.name);
      }

      // 文档加载（无缓存时用合并接口一次性获取 doc+content+path）
      // force：跳过缓存强制向后端校验，确保顶部“修改者/更新于”等元数据始终与后端一致
      await docStore.loadDocument(id, { force: true });

      if (version !== loadDocVersion) return;

      // 合并接口可能已返回 path 并缓存，更新面包屑
      const updatedPath = docStore.getCachedPath(id);
      if (updatedPath && updatedPath !== cachedPath) {
        breadcrumbs.value = buildBreadcrumbs(updatedPath, spaceStore.currentSpace?.name);
      }

      // 仅当 docStore 没有缓存 path（doc 缓存有效但 path 缓存无效）时单独请求
      if (!updatedPath) {
        const pathRes = await getDocumentPath(id).catch(() => null);
        if (version !== loadDocVersion) return;
        if (pathRes) {
          docStore.setCachedPath(id, pathRes);
          breadcrumbs.value = buildBreadcrumbs(pathRes, spaceStore.currentSpace?.name);
        }
      }
    } catch (e: unknown) {
      if (version !== loadDocVersion) return;
      selectedDocumentId.value = null;
      docStore.currentDocument = null;
      docStore.currentContent = '';
      breadcrumbs.value = [];
      const msg = e instanceof Error ? e.message : String(e);
      // 认状态码，不认消息里的标记串：消息是要印给用户看的（以前这里靠 `[rate_limit]` 前缀，
      // 于是那个内部判别串会顺着 ErrorBoundary 的"原因"格子漏到界面上）。
      const status = e instanceof ApiError ? e.status : undefined;
      const isNotFound = status === 404 || msg?.includes('not found') || msg?.includes('不存在');
      const isRateLimited = status === 429;
      if (isNotFound) {
        router.replace(`/space/${spaceId.value}`);
        ui.showToast(t('tsUi.k34'), 'error');
      } else if (isRateLimited) {
        ui.showToast(t('tsUi.k35'), 'warning');
      } else {
        ui.showToast(t('tsUi.k36'), 'error');
      }
      return;
    }
    if (contentRef.value) contentRef.value.scrollTop = 0;
  }

  function navigateToSpaceRoot() {
    selectedDocumentId.value = null;
    docStore.currentDocument = null;
    docStore.currentContent = '';
    breadcrumbs.value = [];
    router.push({ path: `/space/${spaceId.value}`, query: { root: '1' } });
  }

  function pushHistory(docId: number) {
    historyStore.push({
      path: route.fullPath,
      scrollTop: contentRef.value?.scrollTop || 0,
      expandedNodes: [...docStore.expandedNodes],
      selectedDocumentId: docId,
    });
  }

  function navigateToDoc(id: number) {
    selectedDocumentId.value = id;
    pushHistory(id);
    router.push(`/space/${spaceId.value}/doc/${id}`);
  }

  function handleSelectDoc(doc: { id: number; space_id: number; type: number }) {
    selectedDocumentId.value = doc.id;
    pushHistory(doc.id);
    router.push(`/space/${spaceId.value}/doc/${doc.id}`);
  }

  async function handleLocateDoc(docId: number) {
    selectedDocumentId.value = docId;
    await docStore.expandToDocument(docId);
    pushHistory(docId);
    router.push(`/space/${spaceId.value}/doc/${docId}`);
  }

  function findSiblingOrParent(deletedId: number | null): Document | null {
    if (!deletedId) return null;
    return findSiblingDoc(docStore.tree, deletedId);
  }

  function cleanup() {
    loadDocVersion++;
    abortCtrl.abort();
    docStore.clearCurrentDocument();
  }

  return {
    route, router, docStore, spaceStore, ui, canEditInSpace,
    spaceId, docId, selectedDocumentId, breadcrumbs, contentRef, initError,
    initSpace, retryInit, mountInit, loadDocument,
    navigateToSpaceRoot, navigateToDoc, handleSelectDoc, handleLocateDoc, handleBack,
    findSiblingOrParent, cleanup,
  };
}
