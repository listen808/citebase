/**
 * 文档草稿管理
 * 使用特殊的键前缀来存储草稿数据，不使用统一的storage前缀
 */
import { storageGetNumber, storageSetNumber } from '@/utils/core/storage';
import { useAuthStore } from '@/stores/auth/auth';

const DRAFT_KEY_PREFIX = 'citebase_doc_draft_';
const DRAFT_TIME_KEY_PREFIX = 'citebase_doc_draft_time_';
const SERVER_TIME_KEY_PREFIX = 'citebase_doc_server_time_';
const DRAFT_EXPIRY_MS = 24 * 60 * 60 * 1000;
const MAX_DRAFT_COUNT = 50;

function getUserId(): number {
  return useAuthStore().user?.id ?? 0;
}

function draftKey(id: number): string {
  return `${DRAFT_KEY_PREFIX}${getUserId()}_${id}`;
}

function draftTimeKey(id: number): string {
  return `${DRAFT_TIME_KEY_PREFIX}${getUserId()}_${id}`;
}

function serverTimeKey(id: number): string {
  return `${SERVER_TIME_KEY_PREFIX}${getUserId()}_${id}`;
}

export function useDraft() {
  /**
   * 获取当前草稿数量
   */
  function getDraftCount(): number {
    let count = 0;
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.startsWith(DRAFT_KEY_PREFIX) && !key.includes('_time_')) {
        count++;
      }
    }
    return count;
  }

  /**
   * 删除最旧的草稿
   */
  function removeOldestDraft(): void {
    const draftEntries: { key: string; timeKey: string; timestamp: number }[] = [];

    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (key && key.startsWith(DRAFT_KEY_PREFIX) && !key.includes('_time_')) {
        const id = key.replace(DRAFT_KEY_PREFIX, '');
        const timeKey = `${DRAFT_TIME_KEY_PREFIX}${id}`;
        const timestamp = storageGetNumber(timeKey, 0);
        draftEntries.push({ key, timeKey, timestamp });
      }
    }

    if (draftEntries.length > 0) {
      draftEntries.sort((a, b) => a.timestamp - b.timestamp);
      const oldest = draftEntries[0];
      if (oldest) {
        localStorage.removeItem(oldest.key);
        localStorage.removeItem(oldest.timeKey);
      }
    }
  }

  /**
   * 保存草稿内容
   */
  function saveDraft(documentId: number, content: string): void {
    // 检查是否已存在该文档的草稿
    const existingDraft = loadDraft(documentId);
    if (!existingDraft && getDraftCount() >= MAX_DRAFT_COUNT) {
      // 如果是新草稿且数量已达上限，删除最旧的
      removeOldestDraft();
    }
    localStorage.setItem(draftKey(documentId), content);
    storageSetNumber(draftTimeKey(documentId), Date.now());
  }

  /**
   * 加载草稿内容
   */
  function loadDraft(documentId: number): string | null {
    return localStorage.getItem(draftKey(documentId));
  }

  /**
   * 加载草稿时间戳
   */
  function loadDraftTimestamp(documentId: number): number {
    return storageGetNumber(draftTimeKey(documentId), 0);
  }

  /**
   * 保存服务器时间戳
   */
  function saveServerTimestamp(documentId: number, timestamp: number): void {
    storageSetNumber(serverTimeKey(documentId), timestamp);
  }

  /**
   * 加载服务器时间戳
   */
  function loadServerTimestamp(documentId: number): number {
    return storageGetNumber(serverTimeKey(documentId), 0);
  }

  /**
   * 清除草稿
   */
  function clearDraft(documentId: number): void {
    localStorage.removeItem(draftKey(documentId));
    localStorage.removeItem(draftTimeKey(documentId));
  }

  /**
   * 检查草稿状态
   */
  function hasDraft(documentId: number, serverUpdatedAt: number): { exists: boolean; expired: boolean; serverUpdated: boolean } {
    const draft = loadDraft(documentId);
    if (!draft) return { exists: false, expired: false, serverUpdated: false };

    const draftTimestamp = loadDraftTimestamp(documentId);
    const lastServerTime = loadServerTimestamp(documentId);

    const expired = Date.now() - draftTimestamp > DRAFT_EXPIRY_MS;
    const serverUpdated = serverUpdatedAt > lastServerTime;

    return { exists: true, expired, serverUpdated };
  }

  /**
   * 清理过期草稿
   */
  function cleanExpiredDrafts(): void {
    const doClean = () => {
      const draftEntries: { key: string; timeKey: string; timestamp: number }[] = [];

      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        if (key && key.startsWith(DRAFT_KEY_PREFIX) && !key.includes('_time_')) {
          const id = key.replace(DRAFT_KEY_PREFIX, '');
          const timeKey = `${DRAFT_TIME_KEY_PREFIX}${id}`;
          const timestamp = storageGetNumber(timeKey, 0);
          draftEntries.push({ key, timeKey, timestamp });
        }
      }

      const now = Date.now();

      // 删除过期草稿
      draftEntries.forEach((entry) => {
        if (now - entry.timestamp > DRAFT_EXPIRY_MS) {
          localStorage.removeItem(entry.key);
          localStorage.removeItem(entry.timeKey);
        }
      });

      // 如果草稿数量超过限制，删除最旧的
      const validDrafts = draftEntries.filter((d) => now - d.timestamp <= DRAFT_EXPIRY_MS);
      if (validDrafts.length > MAX_DRAFT_COUNT) {
        validDrafts
          .sort((a, b) => a.timestamp - b.timestamp)
          .slice(0, validDrafts.length - MAX_DRAFT_COUNT)
          .forEach((entry) => {
            localStorage.removeItem(entry.key);
            localStorage.removeItem(entry.timeKey);
          });
      }
    };

    // 使用requestIdleCallback在浏览器空闲时执行清理
    if (typeof requestIdleCallback !== 'undefined') {
      requestIdleCallback(doClean);
    } else {
      doClean();
    }
  }

  return {
    saveDraft,
    loadDraft,
    loadDraftTimestamp,
    saveServerTimestamp,
    loadServerTimestamp,
    clearDraft,
    hasDraft,
    cleanExpiredDrafts,
    DRAFT_EXPIRY_MS,
  };
}
