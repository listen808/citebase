import { shallowRef } from 'vue';
import { writeAssist, generateTable, generateReport } from '@/api/modules/ai/ai-write';
import type { AIWriteAction } from '@/types/ai';
import { useUIStore } from '@/stores/app/ui';
import i18n from '@/i18n';

const { t } = i18n.global;

// `table` / `report` 走独立 API，不进入 writeAssist（对应 R2：类型层与运行时均分流）
export type WriteAssistAction = Exclude<AIWriteAction, 'table' | 'report'>;
export type EditorAIAction = WriteAssistAction | 'table' | 'report';

interface RunOptions {
  tableDescription?: string;
  reportTitle?: string;
  reportSources?: string[];
}

// 模块级并发锁：所有 AI 入口共享单一 loading，避免重复触发 LLM
const running = shallowRef(false);

export function useAIWrite() {
  const ui = useUIStore();

  async function run(action: EditorAIAction, text: string, options?: RunOptions): Promise<string> {
    if (running.value) return '';
    running.value = true;
    try {
      if (action === 'table') {
        const res = await generateTable({ description: options?.tableDescription || text || '请生成一个3列的数据表格' });
        return res?.markdown || '';
      }
      if (action === 'report') {
        const res = await generateReport({
          type: 'weekly',
          title: options?.reportTitle || text || '工作汇报',
          sources: options?.reportSources,
        });
        return res?.content || '';
      }
      const res = await writeAssist({ text, action });
      return res?.result || '';
    } catch (e) {
      ui.showToast(e instanceof Error ? e.message: t('tsUi.k1'), 'error');
      return '';
    } finally {
      running.value = false;
    }
  }

  function isRunning(): boolean {
    return running.value;
  }

  return { run, isRunning, running };
}