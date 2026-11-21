import { get } from '../../client';

export interface KGNode {
  id: string;
  label: string;
  type: string;
  group: string;
}

export interface KGEdge {
  source: string;
  target: string;
  label: string;
  confidence: number;
}

export interface KnowledgeGraphData {
  nodes: KGNode[];
  edges: KGEdge[];
  metadata?: { entity_count: number; relation_count: number };
}

export function getKnowledgeGraph(spaceId?: number): Promise<KnowledgeGraphData> {
  const url = spaceId ? `/ai/knowledge-graph?space_id=${spaceId}` : '/ai/knowledge-graph';
  return get<KnowledgeGraphData>(url);
}
