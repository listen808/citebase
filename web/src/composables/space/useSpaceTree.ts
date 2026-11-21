import { computed, onBeforeUnmount, shallowRef, watch } from 'vue';
import { useDocumentStore } from '@/stores/workspace/document';
import { getDocumentTree } from '@/api/modules/document/document';
import type { Document } from '@/types/models';

/**
 * 目录树内搜索（对齐主流方案：语雀 / Notion / VS Code 式树过滤）
 *
 * 语义：
 * 1. 只按「名称」过滤（导航用途）；全文内容搜索请使用全局搜索页 /search。
 * 2. 名称命中的节点保留，并显示其全部子树（找到目录即可浏览其内容）。
 * 3. 名称不匹配、但存在匹配后代的节点保留为「导航路径」，且仅保留匹配后代。
 * 4. 其余节点剪掉；命中的祖先自动展开加载，命中关键词在节点上高亮。
 */
export function useSpaceTree() {
  const docStore = useDocumentStore();
  const treeSearchText = shallowRef('');
  const treeFocusedId = shallowRef<number | null>(null);
  const isSearching = shallowRef(false);
  // searchMatchId 提升到 store，使全局搜索跳转时也能触发 TreeNode 滚动定位
  const searchMatchId = computed(() => docStore.searchMatchId);
  // 搜索完成后（含懒加载展开）强制 filteredTree 重新计算
  const searchVersion = shallowRef(0);

  // 搜索用的完整树：docStore.tree 是懒加载的，只挂已展开过的分支，
  // 直接过滤它就会出现「没展开的目录搜不到」（用户实测）。
  // /documents/tree 一次就返回整棵嵌套树，按空间缓存一份专供过滤。
  const searchTree = shallowRef<Document[] | null>(null);
  let searchTreeSpaceId: number | null = null;

  const filteredTree = computed(() => {
    const keyword = treeSearchText.value.trim().toLowerCase();
    // 依赖 searchVersion，确保整树就绪后强制重新计算
    void searchVersion.value;
    if (!keyword) return docStore.tree;
    const source = searchTreeSpaceId !== null && searchTreeSpaceId === docStore.currentSpaceId && searchTree.value
      ? searchTree.value
      : docStore.tree;
    return filterTree(source, keyword);
  });

  async function ensureSearchTree(spaceId: number) {
    if (searchTreeSpaceId === spaceId && searchTree.value) return;
    const res = await getDocumentTree(spaceId).catch(() => null);
    searchTree.value = res ?? null;
    searchTreeSpaceId = spaceId;
    searchVersion.value++;
  }

  function filterTree(nodes: Document[], keyword: string): Document[] {
    const result: Document[] = [];
    for (const node of nodes) {
      const selfMatch = node.name.toLowerCase().includes(keyword);
      const children = node.children || [];
      const filteredChildren = children.length > 0 ? filterTree(children, keyword) : [];
      if (selfMatch) {
        // 命中节点：严格过滤，仅保留其中同样命中的子节点（不带出未命中的内容）
        result.push({ ...node, children: filteredChildren, has_children: node.has_children ?? filteredChildren.length > 0 });
      } else if (filteredChildren.length > 0) {
        // 未命中但有命中后代：作为导航路径保留，仅保留匹配后代
        result.push({ ...node, children: filteredChildren, has_children: true });
      }
      // 既不命中也无命中后代：剪掉
    }
    return result;
  }


  let preSearchExpandedNodes: Set<number> | null = null;
  let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

  async function searchAndExpand(keyword: string) {
    isSearching.value = true;
    docStore.setSearchMatchId(null);
    try {
      const spaceId = docStore.currentSpaceId;
      if (!spaceId) return;

      // 数据源改成整棵树，不再依赖全文搜索接口：
      // 后台 fulltext_search_open=0 时接口直接 503，旧实现拿不到 path_ids
      // 就等于"只能搜到已经展开过的分支"。
      await ensureSearchTree(spaceId);
      const matched = filterTree(searchTree.value ?? docStore.tree, keyword.toLowerCase());

      // 命中的祖先必须展开才看得见：树只渲染 expandedNodes 里的分支
      const expandIds = new Set(docStore.expandedNodes);
      const expand = (nodes: Document[]) => {
        for (const node of nodes) {
          if (node.children && node.children.length > 0) {
            expandIds.add(node.id);
            expand(node.children);
          }
        }
      };
      expand(matched);
      docStore.expandedNodes = expandIds;

      // 自动滚动定位到第一个命中的文档（目录优先让位给具体文档）
      let firstDoc: number | null = null;
      let firstAny: number | null = null;
      const pick = (nodes: Document[]) => {
        for (const node of nodes) {
          if (firstAny === null) firstAny = node.id;
          if (firstDoc === null && node.type === 1) firstDoc = node.id;
          if (node.children) pick(node.children);
        }
      };
      pick(matched);
      const target = firstDoc ?? firstAny;
      if (target !== null) docStore.setSearchMatchId(target);
    } finally {
      isSearching.value = false;
      // 搜索完成后强制 filteredTree 重新计算
      searchVersion.value++;
    }
  }

  watch(treeSearchText, (newVal) => {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(async () => {
      if (newVal.trim()) {
        if (!preSearchExpandedNodes) {
          preSearchExpandedNodes = new Set(docStore.expandedNodes);
        }
        await searchAndExpand(newVal.trim());
      } else if (preSearchExpandedNodes) {
        docStore.setSearchMatchId(null);
        docStore.expandedNodes = preSearchExpandedNodes;
        preSearchExpandedNodes = null;
        if (docStore.currentSpaceId) {
          docStore.persistExpanded(docStore.currentSpaceId as number);
        }
      }
    }, 250);
  });

  onBeforeUnmount(() => {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
  });

  async function expandAllNodes() {
    await docStore.expandAll();
  }

  function collapseAllNodes() {
    docStore.collapseAll();
  }

  async function handleToggleNode(id: number) {
    await docStore.toggleNode(id);
  }

  return {
    treeSearchText,
    treeFocusedId,
    filteredTree,
    isSearching,
    searchMatchId,
    expandAllNodes,
    collapseAllNodes,
    handleToggleNode,
  };
}
