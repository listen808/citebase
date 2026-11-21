import type { ChatMessage, AIAskSource } from './ai';
import type { IconName } from '@/utils/icons';

export type AIWriterTab = 'chat' | 'templates' | 'history';

export interface AITemplate {
  id: string;
  name: string;
  description: string;
  variables: string[];
  icon?: IconName;
}

export interface AIChatMessage extends ChatMessage {
  id: string;
  timestamp: number;
  sources?: AIAskSource[];
}

export interface AIWriterState {
  open: boolean;
  activeTab: AIWriterTab;
  messages: AIChatMessage[];
  loading: boolean;
  /** 最近一次请求失败的原因；空串表示没有失败。发送失败原先只 log，界面停在「没有回复」 */
  error: string;
  streamingContent: string;
  selectedText: string;
  templates: AITemplate[];
}

export interface ApplyTemplateRequest {
  templateId: string;
  variables: Record<string, string>;
  docContext?: {
    docId?: number;
    spaceId?: number;
    currentContent?: string;
  };
}

export interface ChatWithDocRequest {
  messages: ChatMessage[];
  spaceId?: number;
  docId?: number;
  selectedText?: string;
  stream?: boolean;
}

// Re-export to use from existing types
export type { AIAskSource } from './ai';
