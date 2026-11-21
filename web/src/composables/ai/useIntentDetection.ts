import { ref, computed } from 'vue';
import { detectIntent, type AIIntent } from '@/api/modules/ai/ai-intent';
import { logger } from '@/utils/logger';

export function useIntentDetection() {
  const detectedIntent = ref<AIIntent | null>(null);
  const manualOverrideIntent = ref<AIIntent | null>(null);
  const detecting = ref(false);

  const effectiveIntent = computed<AIIntent>(() => {
    return manualOverrideIntent.value || detectedIntent.value || 'chat';
  });

  async function detect(question: string) {
    if (manualOverrideIntent.value) {
      return manualOverrideIntent.value;
    }
    // 已有检测意图则复用，避免每次发送都重新识别导致按钮显示与实际执行不一致
    if (detectedIntent.value) {
      return detectedIntent.value;
    }
    detecting.value = true;
    try {
      const result = await detectIntent({ question });
      detectedIntent.value = result.intent || 'chat';
      return detectedIntent.value;
    } catch (e) {
      logger.error('Intent detection failed:', e);
      detectedIntent.value = 'chat';
      return 'chat' as AIIntent;
    } finally {
      detecting.value = false;
    }
  }

  function overrideIntent(intent: AIIntent) {
    manualOverrideIntent.value = intent;
  }

  function clearOverride() {
    manualOverrideIntent.value = null;
  }

  function reset() {
    detectedIntent.value = null;
    manualOverrideIntent.value = null;
  }

  return {
    detectedIntent,
    manualOverrideIntent,
    effectiveIntent,
    detecting,
    detect,
    overrideIntent,
    clearOverride,
    reset,
  };
}
