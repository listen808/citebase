
/**
 * 智能搜索增强工具
 * 拼音/模糊匹配、同义词扩展、搜索结果排序
 */

import type { SearchResult } from '@/types/models';
import { getSynonyms } from '@/api/modules/document/search';

/**
 * 拼音字母映射表（简化版，覆盖常用拼音）
 * 注意: 当前硬编码映射已覆盖大部分常用场景，如需完整拼音支持可引入 pinyin-pro 库
 */
const PINYIN_MAP: Record<string, string[]> = {
  'a': ['阿', '啊', '爱', '安', '暗', '按', '案', '昂', '奥'],
  'b': ['不', '把', '比', '被', '白', '百', '本', '部', '别', '办', '半', '包', '保', '报', '北', '备', '必', '变', '表', '并', '步'],
  'c': ['从', '出', '此', '才', '长', '成', '除', '处', '次', '参', '草', '测', '层', '查', '产', '常', '场', '超', '车', '称', '程', '持', '重', '初', '传', '创', '春', '纯', '存', '错'],
  'd': ['的', '大', '到', '得', '都', '对', '点', '地', '但', '当', '道', '等', '定', '动', '多', '打', '代', '单', '党', '导', '低', '第', '调', '东', '度', '端', '段', '断', '队', '吨'],
  'e': ['而', '二', '儿', '额', '恩', '恶'],
  'f': ['发', '非', '方', '分', '反', '服', '副', '法', '范', '放', '飞', '费', '风', '否', '夫', '复', '负', '富'],
  'g': ['个', '过', '给', '高', '更', '关', '国', '工', '公', '改', '感', '刚', '根', '共', '够', '古', '故', '官', '广', '规', '果'],
  'h': ['和', '好', '会', '还', '后', '很', '化', '回', '或', '活', '海', '含', '行', '号', '合', '河', '核', '红', '护', '花', '划', '环', '换', '黄', '婚', '混'],
  'j': ['就', '家', '将', '机', '叫', '几', '间', '见', '解', '进', '经', '加', '件', '建', '教', '接', '结', '今', '金', '近', '九', '酒', '据', '军', '举', '决', '绝', '均'],
  'k': ['可', '看', '开', '快', '空', '科', '口', '苦', '困', '扩'],
  'l': ['了', '来', '里', '老', '路', '力', '理', '两', '量', '领', '流', '六', '龙', '落', '律', '绿', '论', '罗', '落'],
  'm': ['吗', '没', '么', '每', '满', '面', '民', '明', '名', '目', '马', '买', '毛', '美', '门', '密', '免', '秒', '命', '模', '某', '母', '木'],
  'n': ['你', '那', '能', '年', '内', '南', '难', '脑', '呢', '女', '努', '暖'],
  'o': ['哦', '噢', '欧'],
  'p': ['平', '派', '配', '跑', '片', '品', '破', '普'],
  'q': ['全', '去', '起', '前', '其', '且', '求', '确', '群', '千', '强', '青', '轻', '清', '请', '区', '取', '权', '缺', '却'],
  'r': ['如', '人', '日', '让', '若', '然', '认', '任', '容', '入', '软', '弱'],
  's': ['是', '说', '上', '三', '所', '时', '事', '生', '使', '十', '四', '水', '社', '身', '深', '神', '省', '师', '实', '石', '识', '市', '示', '世', '式', '收', '手', '首', '书', '数', '思', '死', '算', '虽', '岁', '速'],
  't': ['他', '她', '它', '同', '天', '太', '特', '提', '题', '体', '通', '头', '图', '土', '团', '推'],
  'w': ['我', '完', '为', '外', '无', '文', '问', '五', '万', '王', '往', '网', '望', '位', '温', '物', '务', '误'],
  'x': ['想', '下', '新', '现', '小', '性', '行', '学', '需', '许', '系', '西', '喜', '细', '先', '相', '向', '消', '效', '些', '写', '信', '星', '型', '形', '修', '选', '需', '宣'],
  'y': ['一', '也', '有', '要', '用', '又', '于', '与', '以', '已', '因', '应', '月', '元', '原', '员', '远', '运', '约', '越', '云'],
  'z': ['在', '这', '最', '再', '做', '着', '子', '自', '知', '只', '之', '至', '正', '整', '政', '制', '中', '主', '住', '专', '转', '装', '准', '资', '总', '走', '组', '族', '足', '作', '座'],
};

let synonymMapCache: Record<string, string[]> | null = null;
let synonymLoadPromise: Promise<Record<string, string[]>> | null = null;

async function loadSynonymMap(): Promise<Record<string, string[]>> {
  if (synonymMapCache) return synonymMapCache;
  if (synonymLoadPromise) return synonymLoadPromise;

  synonymLoadPromise = (async () => {
    try {
      const data = await getSynonyms();
      synonymMapCache = data.synonym_map || {};
      return synonymMapCache;
    } catch {
      synonymMapCache = getDefaultSynonymMap();
      return synonymMapCache;
    }
  })();

  return synonymLoadPromise;
}

function getDefaultSynonymMap(): Record<string, string[]> {
  return {
    '文档': ['文件', '记录', '内容'],
    '搜索': ['查找', '检索', '查询'],
    '创建': ['新建', '添加', '新增'],
    '删除': ['移除', '去掉', '清除'],
    '编辑': ['修改', '更新', '调整'],
    '空间': ['目录', '文件夹', '区域'],
    '用户': ['成员', '人员', '使用者'],
    '设置': ['配置', '偏好', '选项'],
    '管理': ['维护', '处理', '控制'],
    '帮助': ['支持', '协助', '辅助'],
    '指南': ['教程', '手册', '指引'],
    '常见问题': ['FAQ', '问答', '疑问'],
  };
}

function getSynonymMapSync(): Record<string, string[]> {
  return synonymMapCache || getDefaultSynonymMap();
}

/**
 * 占位提示词轮换库 —— 存 i18n 键而不是文案：util 层不碰 i18n 实例，
 * 消费方（SearchHeader）在展示时 t() 翻译。原先硬编码中文，英文界面也显示中文。
 */
const SEARCH_PLACEHOLDERS = [
  'searchUi.ph1',
  'searchUi.ph2',
  'searchUi.ph3',
  'searchUi.ph4',
  'searchUi.ph5',
];

/**
 * 拼音匹配：检查输入是否可能是中文拼音首字母
 */
export function pinyinMatch(text: string, query: string): boolean {
  const cleanQuery = query.toLowerCase().replace(/\s+/g, '');
  const cleanText = text.toLowerCase();

  if (cleanText.includes(cleanQuery)) return true;

  if (/^[a-z]+$/.test(cleanQuery)) {
    let matchCount = 0;
    let textIndex = 0;

    for (const char of cleanQuery) {
      const possibleChars = PINYIN_MAP[char] || [];
      let found = false;

      for (let i = textIndex; i < cleanText.length; i++) {
        const currentChar = cleanText[i];
        if (currentChar && possibleChars.includes(currentChar)) {
          matchCount++;
          textIndex = i + 1;
          found = true;
          break;
        }
      }

      if (!found) break;
    }

    if (matchCount >= Math.ceil(cleanQuery.length * 0.8)) return true;
  }

  return false;
}

/**
 * 模糊匹配：计算文本与查询的相似度
 */
export function fuzzyMatch(text: string, query: string): number {
  const cleanQuery = query.toLowerCase();
  const cleanText = text.toLowerCase();

  if (cleanText.includes(cleanQuery)) return 100;
  if (pinyinMatch(cleanText, cleanQuery)) return 80;

  const synonymMap = getSynonymMapSync();
  let synonymScore = 0;
  for (const [word, synonyms] of Object.entries(synonymMap)) {
    if (cleanQuery.includes(word) || synonyms.some(s => cleanQuery.includes(s))) {
      if (cleanText.includes(word) || synonyms.some(s => cleanText.includes(s))) {
        synonymScore = 70;
      }
    }
  }

  if (synonymScore > 0) return synonymScore;

  const queryWords = cleanQuery.split(/\s+/);
  let matchCount = 0;
  for (const word of queryWords) {
    if (word && cleanText.includes(word)) {
      matchCount++;
    }
  }

  if (matchCount > 0) {
    return 30 + matchCount * 15;
  }

  return 0;
}

/**
 * 扩展查询：添加同义词（异步，从后端获取）
 */
export async function expandQueryAsync(query: string): Promise<string[]> {
  const synonymMap = await loadSynonymMap();
  return expandQueryWithMap(query, synonymMap);
}

/**
 * 扩展查询：添加同义词（同步，使用缓存或默认值）
 */
export function expandQuery(query: string): string[] {
  return expandQueryWithMap(query, getSynonymMapSync());
}

function expandQueryWithMap(query: string, synonymMap: Record<string, string[]>): string[] {
  const queries: string[] = [query];
  const cleanQuery = query.toLowerCase();

  for (const [word, synonyms] of Object.entries(synonymMap)) {
    if (cleanQuery.includes(word)) {
      for (const syn of synonyms) {
        queries.push(query.replace(word, syn));
      }
    }

    for (const syn of synonyms) {
      if (cleanQuery.includes(syn)) {
        queries.push(query.replace(syn, word));
      }
    }
  }

  return [...new Set(queries)];
}

/**
 * 排序类型
 */
export type SortType = 'relevance' | 'time' | 'hot';

/**
 * 搜索结果排序
 */
export function sortResults(
  results: SearchResult[],
  sortBy: SortType = 'relevance',
  query?: string
): SearchResult[] {
  if (!results.length) return results;

  return [...results].sort((a, b) => {
    switch (sortBy) {
      case 'relevance': {
        const scoreA = a.score || 0;
        const scoreB = b.score || 0;
        if (query) {
          const fuzzyA = fuzzyMatch(a.name || '', query);
          const fuzzyB = fuzzyMatch(b.name || '', query);
          return (scoreB + fuzzyB * 0.01) - (scoreA + fuzzyA * 0.01);
        }
        return scoreB - scoreA;
      }

      case 'time': {
        const timeA = a.updated_at || a.create_time || 0;
        const timeB = b.updated_at || b.create_time || 0;
        return timeB - timeA;
      }

      case 'hot': {
        const hotA = a.view_count || a.score || 0;
        const hotB = b.view_count || b.score || 0;
        return hotB - hotA;
      }

      default:
        return 0;
    }
  });
}

/**
 * 占位提示词轮换
 */
export class PlaceholderRotator {
  private placeholders: string[];
  private currentIndex = 0;
  private timer: ReturnType<typeof setInterval> | null = null;
  private callback: ((text: string) => void) | null = null;

  constructor(placeholders: string[] = SEARCH_PLACEHOLDERS) {
    this.placeholders = placeholders;
  }

  get current(): string {
    return this.placeholders[this.currentIndex] || '';
  }

  setCallback(callback: (text: string) => void): void {
    this.callback = callback;
  }

  start(intervalMs: number = 3000): void {
    this.stop();
    this.timer = setInterval(() => {
      this.currentIndex = (this.currentIndex + 1) % this.placeholders.length;
      if (this.callback) {
        this.callback(this.current);
      }
    }, intervalMs);
  }

  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }
}

/**
 * S15: 编辑距离计算（Levenshtein Distance）
 */
function levenshteinDistance(a: string, b: string): number {
  const m = a.length;
  const n = b.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0) as number[]);
  for (let i = 0; i <= m; i++) dp[i]![0] = i;
  for (let j = 0; j <= n; j++) dp[0]![j] = j;
  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (a[i - 1] === b[j - 1]) {
        dp[i]![j] = dp[i - 1]![j - 1]!;
      } else {
        dp[i]![j] = 1 + Math.min(dp[i - 1]![j]!, dp[i]![j - 1]!, dp[i - 1]![j - 1]!);
      }
    }
  }
  return dp[m]![n]!;
}

/**
 * S15: 搜索纠错建议
 * 基于文档名称字典提供"您是否要搜..."建议
 */
export function spellCheck(query: string, dictionary: string[], maxDistance: number = 2): string[] {
  if (!query || query.length < 2 || !dictionary.length) return [];

  const cleanQuery = query.toLowerCase();
  const suggestions: { word: string; distance: number }[] = [];

  for (const word of dictionary) {
    const cleanWord = word.toLowerCase();
    if (cleanWord === cleanQuery) continue;
    if (cleanWord.includes(cleanQuery) || cleanQuery.includes(cleanWord)) continue;

    const distance = levenshteinDistance(cleanQuery, cleanWord);
    if (distance <= maxDistance && distance < cleanQuery.length * 0.5) {
      suggestions.push({ word, distance });
    }
  }

  suggestions.sort((a, b) => a.distance - b.distance);
  return suggestions.slice(0, 3).map(s => s.word);
}
