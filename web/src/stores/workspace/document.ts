import { defineStore } from 'pinia';
import { ref, computed, shallowRef, triggerRef } from 'vue';
import { getDocumentTree, getRootDocuments, getDocumentChildren, getDocumentByID, getDocumentContent, createDocument, updateDocument, deleteDocument, moveDocument, copyDocument, sortDocuments, rollbackDocument, getDocumentPath, getDocumentFull, lockEdit, unlockEdit, heartbeatEdit } from '@/api/modules/document/document';
import type { CreateDocumentRequest, UpdateDocumentRequest, MoveDocumentRequest, SortDocumentItem } from '@/api/modules/document/document';
import type { Document } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';
import { buildTree as _buildTree } from '@/utils/domain/build-tree';
import { parseContent } from '@/utils/domain/document';
import { useExpandedState } from '@/composables/tree/useExpandedState';
import type { CacheEntry } from '@/utils/core/cache';
import { isCacheValid } from '@/utils/core/cache';
import { logError, getErrorMessage } from '@/utils/core/error-logger';
import { parallelLimit } from '@/utils/core/parallel-limit';

export type TreeNode = Document;

export const useDocumentStore = defineStore('document', () => {
  const nodeIndexMap = shallowRef<Map<number, TreeNode>>(new Map());
  const contentCache = shallowRef<Map<number, CacheEntry<string>>>(new Map());
  const docCacheMap = shallowRef<Map<number, CacheEntry<Document>>>(new Map());
  const pathCache = shallowRef<Map<number, CacheEntry<{ id: number; name: string }[]>>>(new Map());
  // P1-1 修复：根文档列表缓存（与 docCacheMap/contentCache 同 TTL），避免每次返回空间都重新拉取
  const rootsCacheMap = shallowRef<Map<number, CacheEntry<Document[]>>>(new Map());

  function getCachedContent(docId: number): string | null {
    const entry = contentCache.value.get(docId);
    return isCacheValid(entry) ? entry.data : null;
  }

  function setCachedContent(docId: number, content: string) {
    contentCache.value.set(docId, { data: content, timestamp: Date.now() });
    triggerRef(contentCache);
  }

  function getCachedDoc(docId: number): Document | null {
    const entry = docCacheMap.value.get(docId);
    return isCacheValid(entry) ? entry.data : null;
  }

  function getCachedPath(docId: number): { id: number; name: string }[] | null {
    const entry = pathCache.value.get(docId);
    return isCacheValid(entry) ? entry.data : null;
  }

  function setCachedPath(docId: number, path: { id: number; name: string }[]) {
    pathCache.value.set(docId, { data: path, timestamp: Date.now() });
    triggerRef(pathCache);
  }

  function isDocCacheValid(docId: number): boolean {
    return isCacheValid(docCacheMap.value.get(docId));
  }

  function isContentCacheValid(docId: number): boolean {
    const docEntry = docCacheMap.value.get(docId);
    if (!isCacheValid(docEntry)) return false;
    const contentEntry = contentCache.value.get(docId);
    // 目录（type=2）本身也是 md 文档，其 README.md 内容同样应缓存，避免每次点击重复请求
    return isCacheValid(contentEntry);
  }

  function setCachedDoc(docId: number, doc: Document) {
    docCacheMap.value.set(docId, { data: doc, timestamp: Date.now() });
    triggerRef(docCacheMap);
  }

  function updateCachedDoc(docId: number, partial: Partial<Document>) {
    const cached = docCacheMap.value.get(docId);
    if (cached) {
      docCacheMap.value.set(docId, { data: { ...cached.data, ...partial }, timestamp: Date.now() });
      triggerRef(docCacheMap);
    }
  }

  function invalidateDocCache(docId?: number) {
    if (docId !== undefined) {
      docCacheMap.value.delete(docId);
      triggerRef(docCacheMap);
      contentCache.value.delete(docId);
      triggerRef(contentCache);
      pathCache.value.delete(docId);
      triggerRef(pathCache);
    } else {
      docCacheMap.value = new Map();
      contentCache.value = new Map();
      pathCache.value = new Map();
      // P1-1 修复：清空所有空间的 roots 缓存（create/move/copy/sort 后调用）
      rootsCacheMap.value = new Map();
      triggerRef(rootsCacheMap);
    }
  }

  // P1-1 修复：roots 缓存读写辅助
  function getCachedRoots(spaceId: number): Document[] | null {
    const entry = rootsCacheMap.value.get(spaceId);
    return isCacheValid(entry) ? entry.data : null;
  }
  function setCachedRoots(spaceId: number, roots: Document[]) {
    rootsCacheMap.value.set(spaceId, { data: roots, timestamp: Date.now() });
    triggerRef(rootsCacheMap);
  }

  function invalidateContentCache(docId: number) {
    contentCache.value.delete(docId);
    triggerRef(contentCache);
  }

  function buildNodeIndex(nodes: TreeNode[]) {
    const index = new Map<number, TreeNode>();
    const traverse = (list: TreeNode[]) => {
      for (const node of list) {
        index.set(node.id, node);
        if (node.children) {
          traverse(node.children);
        }
      }
    };
    traverse(nodes);
    nodeIndexMap.value = index;
  }

  const ui = useUIStore();
  const tree = ref<TreeNode[]>([]);
  const currentDocument = ref<Document | null>(null);
  const currentContent = shallowRef<string>('');
  const expandedNodes = ref<Set<number>>(new Set());
  const loadedNodes = ref<Set<number>>(new Set());
  const loadingChildren = ref<Set<number>>(new Set());
  // 搜索命中节点 ID（从全局搜索跳转时设置，触发 TreeNode 滚动定位）
  const searchMatchId = shallowRef<number | null>(null);

  const treeLoading = shallowRef(false);
  const contentLoading = shallowRef(false);
  const spaceInitializing = shallowRef(false);
  const loading = computed(() => treeLoading.value || contentLoading.value);
  const saving = shallowRef(false);
  let loadDocVersion = 0;
  let loadDocAbort: AbortController | null = null;

  const currentSpaceId = computed(() => currentDocument.value?.space_id);

  function rebuildIndex() {
    buildNodeIndex(tree.value);
  }

  const { persistExpanded, restoreExpanded } = useExpandedState(expandedNodes);

  function expandNode(id: number) {
    const newSet = new Set(expandedNodes.value);
    newSet.add(id);
    expandedNodes.value = newSet;
  }

  function setSearchMatchId(id: number | null) {
    searchMatchId.value = id;
  }

  function collapseNode(id: number) {
    const newSet = new Set(expandedNodes.value);
    newSet.delete(id);
    expandedNodes.value = newSet;
  }

  function expandNodes(ids: number[]) {
    const newSet = new Set(expandedNodes.value);
    ids.forEach(id => newSet.add(id));
    expandedNodes.value = newSet;
  }

  async function loadTree(spaceId?: number) {
    treeLoading.value = true;
    try {
      const res = await getDocumentTree(spaceId);
      tree.value = res ?? [];
      rebuildIndex();
      const level1Ids = tree.value
        .filter((n: Document) => n.children && n.children.length > 0)
        .map((n: Document) => n.id);
      const restored = currentSpaceId.value ? restoreExpanded(currentSpaceId.value) : null;
      expandedNodes.value = restored && restored.size > 0 ? restored : new Set(level1Ids);
      loadedNodes.value = new Set();
      const loaded = new Set<number>();
      const collect = (nodes: Document[]) => {
        nodes.forEach((n) => {
          if (n.children && n.children.length > 0) {
            loaded.add(n.id);
            collect(n.children);
          }
        });
      };
      collect(tree.value);
      loadedNodes.value = loaded;
    } finally {
      treeLoading.value = false;
    }
  }

  async function loadRootDocuments(spaceId: number, preserveExpanded = false, forceExpandFirstLevel = false) {
    treeLoading.value = true;
    try {
      const currentExpanded = preserveExpanded ? new Set(expandedNodes.value) : new Set<number>();
      // P1-1 修复：preserveExpanded=false 时检查 roots 缓存，避免重复拉取
      // （preserveExpanded=true 用于 move/copy 后强制刷新，不走缓存）
      let res: Document[] | null = null;
      if (!preserveExpanded) {
        const cached = getCachedRoots(spaceId);
        if (cached) {
          res = cached;
        }
      }
      if (!res) {
        res = await getRootDocuments(spaceId);
        if (!preserveExpanded) {
          setCachedRoots(spaceId, res ?? []);
        }
      }
      tree.value = (res ?? []).map((doc: Document) => {
        return {
          ...doc,
          has_children: doc.has_children ?? false,
          children: (doc.has_children ?? false) ? [] : undefined,
        };
      });
      rebuildIndex();
      
      // 先设置展开状态，让 UI 立即显示
      let nodesToExpand: number[] = [];
      if (preserveExpanded) {
        const validIds = new Set(getAllNodeIds(tree.value));
        const validExpanded = new Set([...currentExpanded].filter((id: number) => validIds.has(id)));
        expandedNodes.value = validExpanded;
        nodesToExpand = [...validExpanded];
      } else if (forceExpandFirstLevel) {
        const level1Ids = tree.value
          .filter((n: Document) => n.has_children ?? false)
          .map((n: Document) => n.id);
        expandedNodes.value = new Set(level1Ids);
        nodesToExpand = level1Ids;
      } else {
        const restored = restoreExpanded(spaceId);
        const validIds = new Set(getAllNodeIds(tree.value));
        const validRestored = new Set([...restored].filter(id => validIds.has(id)));
        if (validRestored.size > 0) {
          expandedNodes.value = validRestored;
          nodesToExpand = [...validRestored];
        } else {
          nodesToExpand = tree.value
            .filter((n: Document) => n.has_children ?? false)
            .map((n: Document) => n.id);
          expandedNodes.value = new Set(nodesToExpand);
        }
      }
      loadedNodes.value = new Set();
      
      // 收集需要加载的节点
      const nodesToLoad: number[] = [];
      for (const id of nodesToExpand) {
        const node = findNodeById(id);
        if (node && node.has_children && !loadedNodes.value.has(id)) {
          nodesToLoad.push(id);
        }
      }
      
      // 立即返回，让 UI 先显示
      treeLoading.value = false;
      
      // 后台异步加载子节点
      if (nodesToLoad.length > 0) {
        if (preserveExpanded) {
          await loadChildrenInBackground(nodesToLoad);
        } else {
          loadChildrenInBackground(nodesToLoad);
        }
      }
    } catch (error) {
      treeLoading.value = false;
      throw error;
    }
  }
  
  // 后台加载子节点，不阻塞 UI
  async function loadChildrenInBackground(nodeIds: number[]) {
    batchLoadingCount++;
    pendingRebuild = false;
    
    try {
      let changed = true;
      while (changed) {
        changed = false;
        const nextBatch: number[] = [];
        
        for (const id of nodeIds) {
          const node = findNodeById(id);
          if (node && node.has_children && (!node.children || node.children.length === 0) && !loadedNodes.value.has(id)) {
            nextBatch.push(id);
          }
        }
        
        if (nextBatch.length > 0) {
          changed = true;
          await parallelLimit(nextBatch.map((id: number) => () => loadChildren(id, true).catch((e) => {
            logError({ module: 'DocumentStore', operation: 'loadChildrenBatch' }, e);
          })), 3);
          // 每层加载后重建索引，让下次迭代 findNodeById 能找到新子节点
          rebuildIndex();
        }
      }
    } finally {
      batchLoadingCount--;
      // 所有批量加载完成后统一更新一次
      if (batchLoadingCount === 0 && pendingRebuild) {
        rebuildIndex();
        tree.value = [...tree.value];
      }
    }
  }

  // 批量加载标志，用于延迟响应式更新（计数器支持并发调用）
  let batchLoadingCount = 0;
  let pendingRebuild = false;
  
  async function loadChildren(parentId: number, skipRebuild = false) {
    const parent = findNodeById(parentId);
    if (!parent) {
      return;
    }

    if (loadedNodes.value.has(parentId)) {
      return;
    }

    // 批量加载时不立即更新 loadingChildren
    if (batchLoadingCount === 0) {
      const newLoading = new Set(loadingChildren.value);
      newLoading.add(parentId);
      loadingChildren.value = newLoading;
    }

    try {
      const res = await getDocumentChildren(parentId);

      const children = (res ?? []).map((doc) => {
        return {
          ...doc,
          has_children: doc.has_children ?? false,
          children: (doc.has_children ?? false) ? [] : undefined,
        };
      });

      parent.children = children;
      parent.has_children = children.length > 0;
      
      // 批量加载时延迟更新
      if (batchLoadingCount > 0) {
        pendingRebuild = true;
        const newLoaded = new Set(loadedNodes.value);
        newLoaded.add(parentId);
        loadedNodes.value = newLoaded;

      } else {
        loadedNodes.value = new Set([...loadedNodes.value, parentId]);
        if (!skipRebuild) {
          rebuildIndex();
          tree.value = [...tree.value];
        }
      }
    } catch (error) {
      logError({ module: 'DocumentStore', operation: 'loadChildren' }, error);
      ui.showToast(getErrorMessage(error), 'error');
      if (!parent.children) {
        parent.children = [];
      }
    } finally {
      if (batchLoadingCount === 0) {
        const newLoading = new Set(loadingChildren.value);
        newLoading.delete(parentId);
        loadingChildren.value = newLoading;
      }
    }
  }

  function findNodeById(id: number): Document | null {
    return nodeIndexMap.value.get(id) ?? null;
  }

  /** 按名称查找已加载的文档节点（wiki 双链 [[页面名]] 解析用，大小写不敏感） */
  function findNodeByName(name: string): Document | null {
    const target = name.trim().toLowerCase();
    if (!target) return null;
    for (const node of nodeIndexMap.value.values()) {
      if (node.name.trim().toLowerCase() === target) return node;
    }
    return null;
  }

  async function loadDocument(docId: number, opts?: { force?: boolean }) {
    // 取消上一次未完成的请求
    if (loadDocAbort) {
      loadDocAbort.abort();
    }
    loadDocAbort = new AbortController();
    const signal = loadDocAbort.signal;

    const loadVersion = ++loadDocVersion;
    const cachedDoc = getCachedDoc(docId);
    const cachedContent = getCachedContent(docId);
    // force=true：跳过文档/内容缓存判断，强制向后端校验最新元数据与内容（仍先用缓存即时显示）
    // 目录（type=2）此前内容恒为空时被缓存，若 force 不刷新内容，后端已支持目录内容后仍显示旧空值
    const docCacheValid = !opts?.force && isDocCacheValid(docId);
    const contentCacheValid = !opts?.force && isContentCacheValid(docId);

    // 如果有缓存，立即显示，不等待网络请求
    if (cachedDoc && (docCacheValid || opts?.force)) {
      currentDocument.value = cachedDoc;
      if (contentCacheValid && cachedContent !== null) {
        currentContent.value = cachedContent;
      }
      expandToDocument(docId);
    }

    // 即使没有文档缓存，如果树中有该节点，也先显示基本信息
    // 注意：若当前已持有该文档的完整数据（如保存后刷新同一文档），
    // 不能用树节点占位覆盖，否则会丢掉 update_time/editor_name 等元数据
    if (!docCacheValid && currentDocument.value?.id !== docId) {
      const treeNode = nodeIndexMap.value.get(docId);
      if (treeNode) {
        currentDocument.value = {
          id: treeNode.id,
          name: treeNode.name,
          type: treeNode.type,
          space_id: treeNode.space_id,
          parent_id: treeNode.parent_id,
          has_children: treeNode.has_children,
        } as Document;
      }
    }

    const needDoc = !docCacheValid;
    const needContent = !contentCacheValid;
    
    // 只有在完全没有任何显示数据时才显示 loading
    const hasDisplayData = docCacheValid || nodeIndexMap.value.has(docId);
    if (needDoc && !hasDisplayData) {
      contentLoading.value = true;
    }

    try {
      if (needDoc && needContent) {
        // 使用合并接口：1 次 HTTP 请求获取文档元数据+内容+路径（原为 2-3 次）
        const full = await getDocumentFull(docId, { signal });
        if (loadVersion !== loadDocVersion) return;
        setCachedDoc(docId, full.document);
        currentDocument.value = full.document;

        const content = full.content || '';
        currentContent.value = content;
        setCachedContent(docId, content);

        // 合并接口已返回 path，直接缓存
        if (full.path && full.path.length > 0) {
          setCachedPath(docId, full.path);
        }
      } else if (needDoc) {
        const doc = await getDocumentByID(docId, { signal });
        if (loadVersion !== loadDocVersion) return;
        setCachedDoc(docId, doc);
        currentDocument.value = doc;
      } else if (needContent) {
        const contentRes = await getDocumentContent(docId, { signal });
        if (loadVersion !== loadDocVersion) return;
        const content = parseContent(contentRes);
        currentContent.value = content;
        setCachedContent(docId, content);
      }

      if (!docCacheValid) {
        expandToDocument(docId);
      }
    } catch (error) {
      // 请求被取消（用户快速切换文档）不视为错误
      if ((error as Error)?.name === 'CanceledError' || (error as Error)?.name === 'AbortError') {
        return;
      }
      if (currentDocument.value?.id === docId) {
        currentDocument.value = null;
        currentContent.value = '';
      }
      throw error;
    } finally {
      if (contentLoading.value) {
        contentLoading.value = false;
      }
    }
  }

  async function toggleNode(docId: number) {
    const isExpanding = !expandedNodes.value.has(docId);
    if (isExpanding && loadingChildren.value.has(docId)) return;
    const newExpanded = new Set(expandedNodes.value);
    if (isExpanding) {
      newExpanded.add(docId);
    } else {
      newExpanded.delete(docId);
    }
    expandedNodes.value = newExpanded;
    if (currentSpaceId.value) {
      persistExpanded(currentSpaceId.value);
    }
    if (isExpanding && !loadedNodes.value.has(docId)) {
      await loadChildren(docId);
    }
  }

  async function expandToDocument(docId: number) {
    const ancestorIds: number[] = [];
    const findAncestors = (nodes: Document[]): boolean => {
      for (const node of nodes) {
        if (node.id === docId) return true;
        if (node.children?.length && findAncestors(node.children)) {
          ancestorIds.push(node.id);
          return true;
        }
      }
      return false;
    };
    findAncestors(tree.value);

    if (ancestorIds.length === 0) {
      const cachedPath = getCachedPath(docId);
      if (cachedPath && cachedPath.length > 0) {
        ancestorIds.push(...cachedPath.slice(0, -1).map(p => p.id));
      }
    }

    if (ancestorIds.length === 0) {
      try {
        const pathRes = await getDocumentPath(docId);
        if (pathRes && pathRes.length > 0) {
          setCachedPath(docId, pathRes);
          const pathIds = pathRes.slice(0, -1).map(p => p.id);
          ancestorIds.push(...pathIds);
        }
      } catch { /* ignore parse errors */ }
    }

    if (ancestorIds.length > 0) {
      for (let i = 0; i < ancestorIds.length; i++) {
        const id = ancestorIds[i]!;
        let node = findNodeById(id);

        if (!node) {
          if (i === 0) {
            // P0-1 修复：根节点 children 互相独立，并发加载（原为串行）
            const rootIdsToLoad = tree.value
              .filter(root => !loadedNodes.value.has(root.id) && root.has_children)
              .map(root => root.id);
            if (rootIdsToLoad.length > 0) {
              await parallelLimit(
                rootIdsToLoad.map(rid => () => loadChildren(rid).catch((e) => {
                  logError({ module: 'DocumentStore', operation: 'loadRootChildren' }, e);
                })),
                5,
              );
            }
          } else {
            const parentId = ancestorIds[i - 1]!;
            if (!loadedNodes.value.has(parentId)) {
              await loadChildren(parentId).catch((e) => {
                logError({ module: 'DocumentStore', operation: 'loadParentChildren' }, e);
              });
            }
          }
          node = findNodeById(id);
        }

        if (node && !loadedNodes.value.has(id) && node.has_children) {
          await loadChildren(id).catch((e) => {
            logError({ module: 'DocumentStore', operation: 'loadChildren' }, e);
          });
        }
      }
    }

    const merged = new Set(expandedNodes.value);
    ancestorIds.forEach(id => merged.add(id));
    expandedNodes.value = merged;
    if (currentSpaceId.value) {
      persistExpanded(currentSpaceId.value);
    }
  }

  function getAllNodeIds(nodes: Document[]): number[] {
    const ids: number[] = [];
    for (const node of nodes) {
      ids.push(node.id);
      if (node.children && node.children.length > 0) {
        ids.push(...getAllNodeIds(node.children));
      }
    }
    return ids;
  }

  async function create(data: CreateDocumentRequest) {
    saving.value = true;
    try {
      let siblings: Document[];
      if (data.parent_id > 0) {
        const parent = findNodeById(data.parent_id);
        siblings = parent?.children ?? [];
      } else {
        siblings = tree.value;
      }
      const maxSequence = siblings.reduce((max, n) => Math.max(max, n.sequence ?? 0), 0);
      const reqData = { ...data, sequence: data.sequence ?? (maxSequence + 1) };
      const doc = await createDocument(reqData);
      invalidateDocCache();
      const newNode: Document = {
        ...doc,
        has_children: doc.type === 2 ? (doc.has_children ?? false) : (doc.has_children ?? false),
        children: doc.type === 2 && (doc.has_children ?? false) ? [] : undefined,
      };
      if (data.parent_id > 0) {
        const parent = findNodeById(data.parent_id);
        if (parent) {
          if (!parent.children) parent.children = [];
          parent.children.push(newNode);
          parent.has_children = true;
        }
        expandNode(data.parent_id);
        loadedNodes.value = new Set([...loadedNodes.value, data.parent_id]);
      } else {
        tree.value = [...tree.value, newNode];
      }
      buildNodeIndex(tree.value);
      return doc;
    } finally {
      saving.value = false;
    }
  }

  async function update(docId: number, data: UpdateDocumentRequest) {
    saving.value = true;
    try {
      const doc = await updateDocument(docId, data);
      updateCachedDoc(docId, doc);
      if (data.content !== undefined) {
        invalidateContentCache(docId);
        setCachedContent(docId, data.content);
      }
      if (currentDocument.value?.id === docId) {
        currentDocument.value = { ...currentDocument.value, ...doc };
        if (data.content !== undefined) {
          currentContent.value = data.content;
        }
      }
      const nodeToUpdate = findNodeById( docId);
      if (nodeToUpdate) {
        Object.assign(nodeToUpdate, doc);
      }
      return doc;
    } finally {
      saving.value = false;
    }
  }

  function removeNodeFromTree(nodes: TreeNode[], id: number): TreeNode | null {
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i];
      if (!node) continue;
      if (node.id === id) {
        nodes.splice(i, 1);
        return node;
      }
      if (node.children) {
        const removed = removeNodeFromTree(node.children, id);
        if (removed !== null) {
          return removed;
        }
      }
    }
    return null;
  }

  async function remove(docId: number, _spaceId: number) {
    await deleteDocument(docId);
    const removedNode = removeNodeFromTree(tree.value, docId);
    if (removedNode) {
      const parentNode = removedNode.parent_id ? findNodeById(removedNode.parent_id) : null;
      if (parentNode) {
        if (!parentNode.children || parentNode.children.length === 0) {
          parentNode.has_children = false;
          parentNode.children = undefined;
        }
      }
    }
    invalidateDocCache(docId);
    // 不在此处清空 currentDocument，由调用方在导航后再清理，
    // 避免视图层无法判断"被删除的就是当前文档"而误跳转到空间根
    collapseNode(docId);
    const newLoaded = new Set(loadedNodes.value);
    newLoaded.delete(docId);
    const removeDescendantIds = (nodes: Document[], id: number) => {
      const node = findNodeById(id);
      if (node?.children) {
        const collectIds = (list: Document[]) => {
          for (const n of list) {
            newLoaded.delete(n.id);
            if (n.children) collectIds(n.children);
          }
        };
        collectIds(node.children);
      }
    };
    removeDescendantIds(tree.value, docId);
    loadedNodes.value = newLoaded;
    buildNodeIndex(tree.value);
    if (currentSpaceId.value) {
      persistExpanded(currentSpaceId.value);
    }
  }

  async function relocateDoc(
    docId: number,
    data: MoveDocumentRequest,
    apiFn: (id: number, data: MoveDocumentRequest) => Promise<void>
  ) {
    await apiFn(docId, data);
    invalidateDocCache();
    await loadRootDocuments(data.space_id, true);
    if (data.parent_id > 0) {
      expandNode(data.parent_id);
      const targetNode = findNodeById(data.parent_id);
      if (targetNode && !loadedNodes.value.has(data.parent_id)) {
        await loadChildren(data.parent_id);
      }
    }
    if (currentDocument.value?.id === docId) {
      await expandToDocument(docId);
    }
  }

  async function move(docId: number, data: MoveDocumentRequest) {
    await relocateDoc(docId, data, moveDocument);
  }

  async function copy(docId: number, data: MoveDocumentRequest) {
    await relocateDoc(docId, data, copyDocument);
  }

  async function sort(orders: SortDocumentItem[]) {
    await sortDocuments(orders);
    invalidateDocCache();
    for (const order of orders) {
      const node = findNodeById( order.id);
      if (node) node.sequence = order.sequence;
    }
    const sortNodes = (nodes: TreeNode[]) => {
      nodes.sort((a, b) => (a.sequence ?? 0) - (b.sequence ?? 0));
      for (const node of nodes) {
        if (node.children) sortNodes(node.children);
      }
    };
    sortNodes(tree.value);
    buildNodeIndex(tree.value);
  }

  async function rollback(docId: number, version: number) {
    saving.value = true;
    try {
      const doc = await rollbackDocument(docId, version);
      invalidateDocCache(docId);

      if (currentDocument.value?.id === docId) {
        await loadDocument(docId);
      }

      return doc;
    } finally {
      saving.value = false;
    }
  }

  // 编辑锁：进入编辑模式时获取，30 分钟 TTL，需心跳续期，离开时释放
  async function acquireLock(docId: number): Promise<boolean> {
    try {
      await lockEdit(docId);
      return true;
    } catch {
      return false;
    }
  }

  async function releaseLock(docId: number): Promise<void> {
    try {
      await unlockEdit(docId);
    } catch {
      // 静默：锁可能已过期或被强制释放
    }
  }

  async function heartbeat(docId: number): Promise<void> {
    try {
      await heartbeatEdit(docId);
    } catch {
      // 静默：心跳失败不阻塞编辑
    }
  }


  async function expandAll() {
    const BATCH_SIZE = 3;
    let changed = true;
    while (changed) {
      changed = false;
      const allIds = new Set<number>();
      const idsToLoad: number[] = [];
      const collect = (nodes: TreeNode[]) => {
        nodes.forEach((n) => {
          if (n.has_children || (n.children && n.children.length > 0)) {
            allIds.add(n.id);
            if (!loadedNodes.value.has(n.id) && n.has_children) {
              idsToLoad.push(n.id);
            }
            if (n.children && n.children.length > 0) {
              collect(n.children);
            }
          }
        });
      };
      collect(tree.value);
      expandedNodes.value = allIds;
      if (idsToLoad.length > 0) {
        changed = true;
        await parallelLimit(idsToLoad.map(id => () => loadChildren(id)), BATCH_SIZE);
      }
    }
    if (currentSpaceId.value) persistExpanded(currentSpaceId.value);
  }

  function collapseAll() {
    expandedNodes.value = new Set();
    if (currentSpaceId.value) persistExpanded(currentSpaceId.value);
  }

  function reset() {
    tree.value = [];
    currentDocument.value = null;
    currentContent.value = '';
    expandedNodes.value = new Set();
    loadedNodes.value = new Set();
    loadingChildren.value = new Set();
    contentLoading.value = false;
    treeLoading.value = false;
  }

  function clearCurrentDocument() {
    currentDocument.value = null;
    currentContent.value = '';
    contentLoading.value = false;
  }

  return {
    tree,
    currentDocument,
    currentContent,
    expandedNodes,
    loadedNodes,
    loadingChildren,

    loading,
    treeLoading,
    contentLoading,
    spaceInitializing,
    saving,
    currentSpaceId,
    loadTree,
    loadRootDocuments,
    loadChildren,
    findNodeById,
    findNodeByName,
    loadDocument,
    toggleNode,
    expandToDocument,
    getCachedContent,
    getCachedDoc,
    getCachedPath,
    setCachedPath,
    setCachedContent,
    invalidateDocCache,
    create,
    update,
    remove,
    move,
    copy,
    sort,
    rollback,
    acquireLock,
    releaseLock,
    heartbeat,

    expandAll,
    collapseAll,
    expandNode,
    collapseNode,
    expandNodes,
    searchMatchId,
    setSearchMatchId,
    persistExpanded,
    restoreExpanded,
    getAllNodeIds,
    reset,
    clearCurrentDocument,
  };
});
