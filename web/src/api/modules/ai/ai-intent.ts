import { post } from '../../client';

export type AIIntent = 'chat' | 'data_query' | 'agent' | 'research' | 'wiki_query';

export interface DetectIntentRequest {
  question: string;
}

export interface DetectIntentResponse {
  intent: AIIntent;
  confidence: number;
  hints: string[];
}

/**
 * 检测用户输入的意图，用于统一自然语言入口路由
 */
export function detectIntent(data: DetectIntentRequest): Promise<DetectIntentResponse> {
  return post<DetectIntentResponse>('/ai/intent/detect', data, { timeout: 30000 });
}
