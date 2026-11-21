import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { getAIConfig, getVectorStats } from '@/api/modules/ai/ai-config';
import { logger } from '@/utils/logger';

export const useAIStore = defineStore('ai', () => {
  const enabled = ref(false);
  const provider = ref('');
  const model = ref('');
  const temperature = ref(0);
  const maxTokens = ref(0);
  const baseUrl = ref('');
  const configLoaded = ref(false);
  const configLoading = ref(false);
  /** 配置加载失败标记：让「没启用」和「没加载到」在守卫里分得开（原先两者都报「AI 功能未启用」） */
  const configError = ref(false);

  const vectorEnabled = ref(false);
  const vectorDocCount = ref(0);

  const isAIReady = computed(() => enabled.value && !!provider.value && !!model.value);

  async function loadConfig() {
    if (configLoading.value) return;
    configLoading.value = true;
    try {
      const res = await getAIConfig();
      enabled.value = res.enabled;
      provider.value = res.provider || '';
      model.value = res.model || '';
      temperature.value = res.temperature ?? 0;
      maxTokens.value = res.max_tokens ?? 0;
      baseUrl.value = res.base_url || '';
      configLoaded.value = true;
      configError.value = false;
    } catch (e) {
      // 失败要留痕：enabled=false 会让守卫把「加载失败」误报成「AI 功能未启用」，重进 /ai 也拿不回原因。
      logger.error('Failed to load AI config:', e);
      configError.value = true;
    } finally {
      configLoading.value = false;
    }
  }

  async function loadVectorStats() {
    try {
      const res = await getVectorStats();
      vectorEnabled.value = res.enabled;
      vectorDocCount.value = res.doc_count;
    } catch (e) {
      // 被动统计位：失败只记日志、保留上一次值，不弹 toast（同 loadUnreadCount 口径）。
      logger.error('Failed to load vector stats:', e);
    }
  }

  function $reset() {
    enabled.value = false;
    provider.value = '';
    model.value = '';
    temperature.value = 0;
    maxTokens.value = 0;
    baseUrl.value = '';
    configLoaded.value = false;
    configError.value = false;
    vectorEnabled.value = false;
    vectorDocCount.value = 0;
  }

  return {
    enabled,
    provider,
    model,
    temperature,
    maxTokens,
    baseUrl,
    configLoaded,
    configLoading,
    configError,
    vectorEnabled,
    vectorDocCount,
    isAIReady,
    loadConfig,
    loadVectorStats,
    $reset,
  };
});
