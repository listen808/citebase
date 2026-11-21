import type { IconName } from '@/utils/icons';

export interface DocTemplate {
  key: string;
  name: string;
  category: 'project' | 'meeting' | 'tech' | 'report' | 'personal' | 'other';
  /** 共享图标注册表里的名字（曾是 emoji：emoji 没有描边/尺寸标尺，等于第三套图标族） */
  icon: IconName;
  desc: string;
  content: string;
}

export const templateCategories = [
  { key: 'all', label: '全部' },
  { key: 'project', label: '项目管理' },
  { key: 'meeting', label: '会议' },
  { key: 'tech', label: '技术' },
  { key: 'report', label: '报告' },
  { key: 'personal', label: '个人' },
  { key: 'other', label: '其他' },
] as const;

export const docTemplates: DocTemplate[] = [
  // 项目管理
  { key: 'project-plan', name: '项目计划', category: 'project', icon: 'ClipboardList', desc: '项目目标、里程碑、资源需求', content: `# 项目计划\n\n**项目名称**：\n**项目经理**：\n**开始日期**：\n**预计完成**：\n\n## 项目目标\n\n## 里程碑\n\n| 阶段 | 目标 | 开始 | 结束 | 负责人 |\n| --- | --- | --- | --- | --- |\n| | | | | |\n\n## 资源需求\n\n- 人力：\n- 设备：\n\n## 风险登记\n\n| 风险 | 影响 | 概率 | 应对措施 |\n| --- | --- | --- | --- |\n| | | | |` },
  { key: 'okr', name: 'OKR', category: 'project', icon: 'Target', desc: '目标与关键结果', content: `# OKR\n\n**周期**：\n**姓名**：\n\n## Objective 1\n\n**目标**：\n\n### Key Results\n\n- [ ] KR1: \n- [ ] KR2: \n- [ ] KR3: \n\n## Objective 2\n\n**目标**：\n\n### Key Results\n\n- [ ] KR1: \n- [ ] KR2: \n\n## 总结\n` },
  { key: 'prd', name: '需求文档', category: 'project', icon: 'FileText', desc: '产品需求文档模板', content: `# 需求文档\n\n## 需求背景\n\n## 用户故事\n\n- 作为 **[角色]**，我希望 **[功能]**，以便 **[价值]**\n\n## 功能描述\n\n### 功能点 1\n\n**描述**：\n**优先级**：P0 / P1 / P2\n**验收标准**：\n\n## 非功能需求\n\n- 性能要求：\n- 安全要求：\n\n## 相关文档\n\n- ` },
  { key: 'review', name: '复盘报告', category: 'project', icon: 'Search', desc: '项目复盘与改进', content: `# 复盘报告\n\n**项目/事件**：\n**日期**：\n**参与人**：\n\n## 目标回顾\n\n## 结果评估\n\n| 指标 | 目标 | 实际 | 达成率 |\n| --- | --- | --- | --- |\n| | | | |\n\n## 做得好的\n\n1. \n\n## 待改进的\n\n1. \n\n## 行动计划\n\n| 事项 | 负责人 | 截止日期 |\n| --- | --- | --- |\n| | | |` },

  // 会议
  { key: 'meeting', name: '会议记录', category: 'meeting', icon: 'Users', desc: '会议议题与决议', content: `# 会议记录\n\n**日期**：\n**参会人**：\n**主持人**：\n\n## 议题\n\n1. \n\n## 决议\n\n- \n\n## 待办\n\n| 负责人 | 任务 | 截止日期 | 状态 |\n| --- | --- | --- | --- |\n| | | | |` },

  // 技术
  { key: 'tech-design', name: '技术方案', category: 'tech', icon: 'Layers', desc: '技术架构与设计', content: `# 技术方案\n\n## 背景\n\n## 目标\n\n## 方案设计\n\n### 架构\n\n### 数据模型\n\n## 接口设计\n\n| 方法 | 路径 | 说明 |\n| --- | --- | --- |\n| | | |\n\n## 风险与应对\n\n## 排期` },
  { key: 'api-doc', name: 'API 文档', category: 'tech', icon: 'Code2', desc: '接口文档模板', content: `# API 文档\n\n## 接口名称\n\n**URL**：\n**方法**：\n\n### 请求参数\n\n| 字段 | 类型 | 必填 | 说明 |\n| --- | --- | --- | --- |\n| | | | |\n\n### 响应\n\n\`\`\`json\n{\n  "code": 0,\n  "data": {}\n}\n\`\`\`\n\n### 示例\n\n\`\`\`bash\ncurl -X GET "http://localhost:3001/api/v1/example"\n\`\`\`` },
  { key: 'architecture', name: '架构文档', category: 'tech', icon: 'GitBranch', desc: '系统架构说明', content: `# 架构文档\n\n## 概述\n\n## 系统架构图\n\n\`\`\`mermaid\ngraph TD\n    A[客户端] --> B[API 网关]\n    B --> C[服务层]\n    C --> D[数据库]\n\`\`\`\n\n## 技术选型\n\n| 组件 | 技术 | 说明 |\n| --- | --- | --- |\n| | | |\n\n## 模块划分\n\n### 模块 1\n\n**职责**：\n**依赖**：\n\n## 部署架构\n\n## 安全设计` },
  { key: 'coding-standard', name: '编码规范', category: 'tech', icon: 'Code', desc: '代码规范与约定', content: `# 编码规范\n\n## 命名规范\n\n- 文件命名：\n- 变量命名：\n- 函数命名：\n\n## 代码风格\n\n\`\`\`\n// 示例代码\n\`\`\`\n\n## Git 提交规范\n\n\`\`\`\n<type>(<scope>): <subject>\n\`\`\`\n\n## 目录结构\n\n\`\`\`\n.\n├── src/\n├── tests/\n└── docs/\n\`\`\`\n\n## Code Review 检查项\n\n- [ ] 代码逻辑正确\n- [ ] 测试覆盖\n- [ ] 无明显性能问题` },

  // 报告
  { key: 'weekly', name: '周报', category: 'report', icon: 'BarChart3', desc: '每周工作总结', content: `# 周报\n\n**周期**：\n\n## 本周完成\n\n1. \n\n## 下周计划\n\n1. \n\n## 风险与问题\n\n- ` },
  { key: 'daily', name: '日报', category: 'report', icon: 'Calendar', desc: '每日工作记录', content: `# 日报\n\n**日期**：\n**姓名**：\n\n## 今日完成\n\n1. \n\n## 明日计划\n\n1. \n\n## 遇到的问题\n\n- \n\n## 需要协助\n\n- ` },
  { key: 'release-note', name: '发布说明', category: 'report', icon: 'Rocket', desc: '版本发布说明', content: `# 发布说明\n\n**版本号**：\n**发布日期**：\n\n## 新功能\n\n- \n\n## 改进\n\n- \n\n## 修复\n\n- \n\n## 已知问题\n\n- \n\n## 升级指南\n\n### 数据库变更\n\n### 配置变更\n\n### 依赖更新` },
  { key: 'troubleshoot', name: '故障复盘', category: 'report', icon: 'AlertTriangle', desc: '故障分析与改进', content: `# 故障复盘\n\n**故障时间**：\n**发现人**：\n**影响范围**：\n\n## 时间线\n\n| 时间 | 事件 |\n| --- | --- |\n| | 故障发生 |\n| | 发现告警 |\n| | 开始处理 |\n| | 恢复服务 |\n\n## 根因分析\n\n## 改进措施\n\n| 措施 | 负责人 | 截止日期 |\n| --- | --- | --- |\n| | | |` },

  // 个人
  { key: 'onboarding', name: '新人入职', category: 'personal', icon: 'User', desc: '入职指南与计划', content: `# 新人入职指南\n\n**欢迎**：\n**入职日期**：\n\n## 团队介绍\n\n## 环境准备\n\n- [ ] 开发环境搭建\n- [ ] 代码仓库权限\n- [ ] 内部系统账号\n\n## 必读文档\n\n1. \n\n## 第一周计划\n\n| 日期 | 任务 | 导师 |\n| --- | --- | --- |\n| | | |\n\n## 常见问题\n\n- Q: \n- A: ` },

  // 其他
  { key: 'decision', name: '决策记录', category: 'other', icon: 'Scale', desc: 'ADR 架构决策记录', content: `# 决策记录\n\n**编号**：\n**日期**：\n**决策者**：\n\n## 背景\n\n## 决策\n\n## 备选方案\n\n| 方案 | 优点 | 缺点 |\n| --- | --- | --- |\n| A | | |\n| B | | |\n\n## 影响\n\n## 后续行动` },
  { key: 'blank', name: '空白文档', category: 'other', icon: 'File', desc: '从零开始', content: '' },
];
