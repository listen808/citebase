-- ============================================================
-- Citebase 数据库表结构 DDL
-- 由 tools/genddl 自动生成，模型清单取自 internal/repository 的 AutoMigrate 注册表，
-- 与启动时真实建表结果同源；请勿手工编辑本文件。
-- ⚠️ 每张表都带 DROP TABLE，仅适用于空库初始化；已有库请依赖启动时的 AutoMigrate 升级。
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- aw_ai_usage_daily
DROP TABLE IF EXISTS `aw_ai_usage_daily`;
CREATE TABLE `aw_ai_usage_daily` (
  `id` int NOT NULL AUTO_INCREMENT,
  `date` date NOT NULL,
  `user_id` int NOT NULL DEFAULT 0,
  `provider` varchar(64) DEFAULT NULL,
  `model` varchar(128) DEFAULT NULL,
  `action` varchar(32) DEFAULT NULL,
  `request_count` int NOT NULL DEFAULT 1,
  `token_usage` int NOT NULL DEFAULT 0,
  `duration_ms` bigint NOT NULL DEFAULT 0,
  `failed_count` int NOT NULL DEFAULT 0,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_usage_day` (`date`, `user_id`, `provider`, `model`, `action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='aw_ai_usage_daily';


-- aw_ai_task
DROP TABLE IF EXISTS `aw_ai_task`;
CREATE TABLE `aw_ai_task` (
  `id` int NOT NULL AUTO_INCREMENT,
  `task_id` varchar(64) NOT NULL DEFAULT '',
  `type` varchar(32) NOT NULL DEFAULT '',
  `status` varchar(32) NOT NULL DEFAULT '',
  `progress` decimal(5,4) DEFAULT 0,
  `target_space_ids` text DEFAULT NULL,
  `target_doc_id` int DEFAULT 0,
  `total_docs` int DEFAULT 0,
  `processed_docs` int DEFAULT 0,
  `result` longtext DEFAULT NULL,
  `error_message` text DEFAULT NULL,
  `token_usage` int DEFAULT 0,
  `checkpoint` text DEFAULT NULL,
  `started_at` datetime DEFAULT NULL,
  `completed_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_id` (`task_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI分析任务';


-- aw_ai_doc_mapping
DROP TABLE IF EXISTS `aw_ai_doc_mapping`;
CREATE TABLE `aw_ai_doc_mapping` (
  `id` int NOT NULL AUTO_INCREMENT,
  `ai_doc_id` int NOT NULL DEFAULT 0,
  `source_doc_id` int NOT NULL DEFAULT 0,
  `source_space_id` int NOT NULL DEFAULT 0,
  `mapping_type` varchar(32) NOT NULL DEFAULT '',
  `review_status` varchar(32) DEFAULT 'pending',
  `last_synced_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_ai_doc_id` (`ai_doc_id`),
  KEY `idx_mapping_type` (`mapping_type`),
  KEY `idx_source_doc_id` (`source_doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI文档映射';


-- aw_ai_extracted_entities
DROP TABLE IF EXISTS `aw_ai_extracted_entities`;
CREATE TABLE `aw_ai_extracted_entities` (
  `id` int NOT NULL AUTO_INCREMENT,
  `doc_id` int NOT NULL DEFAULT 0,
  `urls` json DEFAULT NULL,
  `config_items` json DEFAULT NULL,
  `aliases` json DEFAULT NULL,
  `action_items` json DEFAULT NULL,
  `url_groups` json DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_doc_id` (`doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI提取实体';


-- aw_ai_doc_cluster
DROP TABLE IF EXISTS `aw_ai_doc_cluster`;
CREATE TABLE `aw_ai_doc_cluster` (
  `id` int NOT NULL AUTO_INCREMENT,
  `cluster_id` varchar(64) NOT NULL DEFAULT '',
  `doc_id` int NOT NULL DEFAULT 0,
  `space_id` int NOT NULL DEFAULT 0,
  `doc_name` varchar(255) NOT NULL DEFAULT '',
  `similarity` decimal(5,4) DEFAULT NULL,
  `status` varchar(32) NOT NULL DEFAULT '',
  `reason` text DEFAULT NULL,
  `merge_plan` json DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_cluster_id` (`cluster_id`),
  KEY `idx_doc_id` (`doc_id`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI文档聚类';


-- aw_vector_sync_task
DROP TABLE IF EXISTS `aw_vector_sync_task`;
CREATE TABLE `aw_vector_sync_task` (
  `id` int NOT NULL AUTO_INCREMENT,
  `doc_id` int NOT NULL DEFAULT 0,
  `space_id` int NOT NULL DEFAULT 0,
  `doc_name` varchar(500) DEFAULT NULL,
  `content` longtext DEFAULT NULL,
  `op_type` varchar(20) NOT NULL DEFAULT '',
  `status` varchar(20) NOT NULL DEFAULT 'pending',
  `retry_count` tinyint DEFAULT 0,
  `last_error` varchar(500) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_doc_id` (`doc_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='向量同步任务';


-- aw_vector_sync_state
DROP TABLE IF EXISTS `aw_vector_sync_state`;
CREATE TABLE `aw_vector_sync_state` (
  `id` int NOT NULL AUTO_INCREMENT,
  `doc_id` int NOT NULL DEFAULT 0,
  `content_sha256` char(64) NOT NULL,
  `emb_identity` varchar(255) NOT NULL DEFAULT '',
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_id` (`doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='aw_vector_sync_state';


-- aw_knowledge_entity
DROP TABLE IF EXISTS `aw_knowledge_entity`;
CREATE TABLE `aw_knowledge_entity` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL DEFAULT '',
  `type` varchar(50) NOT NULL DEFAULT '',
  `description` text DEFAULT NULL,
  `doc_id` int NOT NULL DEFAULT 0,
  `space_id` int NOT NULL DEFAULT 0,
  `aliases` text DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_doc_id` (`doc_id`),
  KEY `idx_ke_name` (`name`(100)),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识图谱实体';


-- aw_knowledge_relation
DROP TABLE IF EXISTS `aw_knowledge_relation`;
CREATE TABLE `aw_knowledge_relation` (
  `id` int NOT NULL AUTO_INCREMENT,
  `source_entity_id` int NOT NULL DEFAULT 0,
  `target_entity_id` int NOT NULL DEFAULT 0,
  `relation_type` varchar(50) NOT NULL DEFAULT '',
  `description` text DEFAULT NULL,
  `confidence` decimal(3,2) DEFAULT 0.8,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_source_entity_id` (`source_entity_id`),
  KEY `idx_target_entity_id` (`target_entity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='知识图谱关系';


-- aw_ai_feedback
DROP TABLE IF EXISTS `aw_ai_feedback`;
CREATE TABLE `aw_ai_feedback` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL DEFAULT 0,
  `conversation_id` int DEFAULT NULL,
  `question` text DEFAULT NULL,
  `answer` text DEFAULT NULL,
  `rating` tinyint NOT NULL DEFAULT 0,
  `reason` varchar(500) DEFAULT NULL,
  `source_doc_ids` text DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_conversation_id` (`conversation_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI反馈表';


-- aw_ai_ragas_eval
DROP TABLE IF EXISTS `aw_ai_ragas_eval`;
CREATE TABLE `aw_ai_ragas_eval` (
  `id` int NOT NULL AUTO_INCREMENT,
  `question` text NOT NULL,
  `answer` text NOT NULL,
  `expected_answer` text DEFAULT NULL,
  `contexts` text DEFAULT NULL,
  `ground_truth` text DEFAULT NULL,
  `faithfulness` decimal(5,4) DEFAULT NULL,
  `answer_relevancy` decimal(5,4) DEFAULT NULL,
  `context_precision` decimal(5,4) DEFAULT NULL,
  `context_recall` decimal(5,4) DEFAULT NULL,
  `avg_score` decimal(5,4) DEFAULT NULL,
  `duration` varchar(32) DEFAULT '',
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='RAGAS评估表';


-- aw_ai_analysis_result
DROP TABLE IF EXISTS `aw_ai_analysis_result`;
CREATE TABLE `aw_ai_analysis_result` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `doc_id` int NOT NULL DEFAULT 0,
  `summary` text DEFAULT NULL,
  `key_points` json DEFAULT NULL,
  `category` varchar(100) DEFAULT NULL,
  `tags` json DEFAULT NULL,
  `quality_score` decimal(5,2) DEFAULT NULL,
  `quality_issues` json DEFAULT NULL,
  `entities` json DEFAULT NULL,
  `content_hash` varchar(64) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_category` (`category`),
  KEY `idx_doc_id` (`doc_id`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI分析结果';


-- aw_wiki_page
DROP TABLE IF EXISTS `aw_wiki_page`;
CREATE TABLE `aw_wiki_page` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `slug` varchar(190) NOT NULL DEFAULT '',
  `type` varchar(32) NOT NULL DEFAULT 'concept',
  `title` varchar(255) NOT NULL DEFAULT '',
  `summary` text DEFAULT NULL,
  `content` longtext DEFAULT NULL,
  `sources` text DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'planned',
  `version` int NOT NULL DEFAULT 0,
  `base_version` int NOT NULL DEFAULT 0,
  `source_count` int NOT NULL DEFAULT 0,
  `provenance` varchar(16) NOT NULL DEFAULT 'llm',
  `confidence` decimal(4,2) NOT NULL DEFAULT 0.50,
  `superseded_by` varchar(190) NOT NULL DEFAULT '',
  `content_hash` varchar(64) NOT NULL DEFAULT '',
  `compiled_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_wiki_space_slug` (`space_id`, `slug`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_wiki_space_status` (`space_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Wiki页面表';


-- aw_wiki_link
DROP TABLE IF EXISTS `aw_wiki_link`;
CREATE TABLE `aw_wiki_link` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `from_slug` varchar(190) NOT NULL DEFAULT '',
  `to_slug` varchar(190) NOT NULL DEFAULT '',
  `link_type` varchar(32) NOT NULL DEFAULT 'ref',
  `weight` decimal(6,3) NOT NULL DEFAULT 1,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_wiki_link` (`space_id`, `from_slug`, `to_slug`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_to_slug` (`to_slug`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Wiki链接表';


-- aw_wiki_pin
DROP TABLE IF EXISTS `aw_wiki_pin`;
CREATE TABLE `aw_wiki_pin` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `page_slug` varchar(190) NOT NULL DEFAULT '',
  `kind` varchar(32) NOT NULL DEFAULT 'correction',
  `claim` text NOT NULL,
  `anchor` varchar(255) NOT NULL DEFAULT '',
  `provenance` varchar(16) NOT NULL DEFAULT 'human',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_by` int NOT NULL DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_page_slug` (`page_slug`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_wiki_pin_page` (`space_id`, `page_slug`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Wiki置顶表';


-- aw_wiki_review
DROP TABLE IF EXISTS `aw_wiki_review`;
CREATE TABLE `aw_wiki_review` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `page_slug` varchar(190) NOT NULL DEFAULT '',
  `kind` varchar(32) NOT NULL DEFAULT 'draft',
  `action` varchar(32) NOT NULL DEFAULT '',
  `reason` text DEFAULT NULL,
  `base_version` int NOT NULL DEFAULT 0,
  `status` varchar(20) NOT NULL DEFAULT 'pending',
  `resolved_by` int NOT NULL DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_page_slug` (`page_slug`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_wiki_review_status` (`space_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Wiki评审表';


-- aw_wiki_lint_issue
DROP TABLE IF EXISTS `aw_wiki_lint_issue`;
CREATE TABLE `aw_wiki_lint_issue` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `page_slug` varchar(190) NOT NULL DEFAULT '',
  `category` varchar(32) NOT NULL DEFAULT '',
  `severity` varchar(16) NOT NULL DEFAULT 'warning',
  `detail` text DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'open',
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_wiki_lint_space` (`space_id`, `status`, `category`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Wiki治理问题表';


-- aw_wiki_page_history
DROP TABLE IF EXISTS `aw_wiki_page_history`;
CREATE TABLE `aw_wiki_page_history` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `slug` varchar(190) NOT NULL DEFAULT '',
  `version` int NOT NULL DEFAULT 0,
  `title` varchar(255) NOT NULL DEFAULT '',
  `summary` text DEFAULT NULL,
  `content` longtext DEFAULT NULL,
  `sources` text DEFAULT NULL,
  `provenance` varchar(16) NOT NULL DEFAULT 'llm',
  `content_hash` varchar(64) NOT NULL DEFAULT '',
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_wiki_hist_space_slug` (`space_id`, `slug`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='aw_wiki_page_history';


-- aw_ai_conversation
DROP TABLE IF EXISTS `aw_ai_conversation`;
CREATE TABLE `aw_ai_conversation` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL DEFAULT 0,
  `space_id` int DEFAULT 0,
  `type` varchar(20) DEFAULT 'ai',
  `title` varchar(255) DEFAULT NULL,
  `messages` json NOT NULL,
  `token_usage` int DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_type` (`type`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI对话表';


-- aw_ai_audit_log
DROP TABLE IF EXISTS `aw_ai_audit_log`;
CREATE TABLE `aw_ai_audit_log` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL DEFAULT 0,
  `action` varchar(64) NOT NULL DEFAULT '',
  `target_id` varchar(64) DEFAULT '',
  `question` text DEFAULT NULL,
  `answer` text DEFAULT NULL,
  `detail` text DEFAULT NULL,
  `token_usage` int DEFAULT 0,
  `source_count` int DEFAULT 0,
  `model` varchar(128) DEFAULT '',
  `provider` varchar(64) DEFAULT '',
  `result` varchar(32) DEFAULT 'success',
  `reason` varchar(512) DEFAULT '',
  `ip_address` varchar(64) DEFAULT '',
  `user_agent` varchar(255) DEFAULT '',
  `duration_ms` bigint DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_action` (`action`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI审计日志';


-- aw_ai_business_rule
DROP TABLE IF EXISTS `aw_ai_business_rule`;
CREATE TABLE `aw_ai_business_rule` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `name` varchar(200) NOT NULL DEFAULT '',
  `type` varchar(50) NOT NULL DEFAULT '',
  `table_name` varchar(100) NOT NULL DEFAULT '',
  `field_name` varchar(100) DEFAULT NULL,
  `content` text NOT NULL,
  `description` text DEFAULT NULL,
  `status` tinyint DEFAULT 1,
  `schema_space_id` bigint unsigned NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_schema_space_id` (`schema_space_id`),
  KEY `idx_table_name` (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='业务规则';


-- aw_ai_table_cache
DROP TABLE IF EXISTS `aw_ai_table_cache`;
CREATE TABLE `aw_ai_table_cache` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `table_name` varchar(100) DEFAULT NULL,
  `table_comment` varchar(500) DEFAULT NULL,
  `ddl_content` text NOT NULL,
  `columns_json` json DEFAULT NULL,
  `constraints_json` json DEFAULT NULL,
  `space_id` bigint unsigned DEFAULT NULL,
  `schema_space_id` bigint unsigned DEFAULT NULL,
  `doc_id` bigint unsigned DEFAULT NULL,
  `db_type` varchar(50) DEFAULT NULL,
  `row_count_estimate` bigint DEFAULT 0,
  `version` int DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_table_cache_space_name` (`table_name`, `space_id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='表结构缓存';


-- aw_ai_table_relation
DROP TABLE IF EXISTS `aw_ai_table_relation`;
CREATE TABLE `aw_ai_table_relation` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `source_table` varchar(100) NOT NULL DEFAULT '',
  `source_field` varchar(100) NOT NULL DEFAULT '',
  `target_table` varchar(100) NOT NULL DEFAULT '',
  `target_field` varchar(100) NOT NULL DEFAULT '',
  `relation_type` varchar(50) DEFAULT 'one_to_many',
  `confidence` float DEFAULT 1,
  `space_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='表关系';


-- aw_ai_datasource
DROP TABLE IF EXISTS `aw_ai_datasource`;
CREATE TABLE `aw_ai_datasource` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `name` varchar(200) NOT NULL DEFAULT '',
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `db_type` varchar(50) NOT NULL DEFAULT '',
  `host` varchar(200) NOT NULL DEFAULT '',
  `port` int DEFAULT 3306,
  `db_name` varchar(200) NOT NULL DEFAULT '',
  `username` varchar(200) NOT NULL DEFAULT '',
  `password` varchar(500) NOT NULL DEFAULT '',
  `sync_cron` varchar(100) DEFAULT NULL,
  `last_sync_at` datetime DEFAULT NULL,
  `is_active` tinyint DEFAULT 1,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI数据源';


-- aw_ai_sql_history
DROP TABLE IF EXISTS `aw_ai_sql_history`;
CREATE TABLE `aw_ai_sql_history` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `query_text` text NOT NULL,
  `generated_sql` text DEFAULT NULL,
  `executed_sql` text DEFAULT NULL,
  `execution_result` longtext DEFAULT NULL,
  `execution_error` text DEFAULT NULL,
  `is_favorite` tinyint DEFAULT 0,
  `rating` tinyint DEFAULT 0,
  `feedback` text DEFAULT NULL,
  `tables_used` text DEFAULT NULL,
  `execution_time_ms` int DEFAULT 0,
  `datasource_id` bigint unsigned DEFAULT NULL,
  `duration_ms` int DEFAULT 0,
  `status` varchar(20) DEFAULT 'success',
  `explanation_json` longtext DEFAULT NULL,
  `trace_id` varchar(64) DEFAULT NULL,
  `generator_tier` varchar(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_trace_id` (`trace_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SQL执行历史';


-- aw_ai_sql_security_policy
DROP TABLE IF EXISTS `aw_ai_sql_security_policy`;
CREATE TABLE `aw_ai_sql_security_policy` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `max_rows` int DEFAULT 200,
  `timeout_sec` int DEFAULT 15,
  `max_cost_tokens` int DEFAULT 0,
  `allow_patterns` text DEFAULT NULL,
  `deny_patterns` text DEFAULT NULL,
  `allow_dml` tinyint DEFAULT 0,
  `allow_ddl` tinyint DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_space_policy` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SQL安全策略';


-- aw_ai_text2sql_feedback
DROP TABLE IF EXISTS `aw_ai_text2sql_feedback`;
CREATE TABLE `aw_ai_text2sql_feedback` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_query` text NOT NULL,
  `generated_sql` longtext DEFAULT NULL,
  `execution_ok` tinyint NOT NULL DEFAULT 0,
  `error_type` varchar(64) DEFAULT '',
  `error_message` text DEFAULT NULL,
  `corrected_sql` longtext DEFAULT NULL,
  `correction_src` varchar(16) DEFAULT '',
  `duration_ms` int DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_t2s_feedback_space_created` (`space_id`),
  KEY `idx_t2sf_space_created` (`space_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Text2SQL反馈';


-- aw_ai_prompt_template
DROP TABLE IF EXISTS `aw_ai_prompt_template`;
CREATE TABLE `aw_ai_prompt_template` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `name` varchar(100) NOT NULL DEFAULT '',
  `type` varchar(20) NOT NULL DEFAULT 'sql',
  `query_type` varchar(50) NOT NULL DEFAULT 'basic',
  `system_prompt` text DEFAULT NULL,
  `user_template` text NOT NULL,
  `description` varchar(500) DEFAULT NULL,
  `is_active` tinyint DEFAULT 1,
  `is_default` tinyint DEFAULT 0,
  `sort` int DEFAULT 0,
  `space_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='提示词模板';


-- aw_ai_sql_example
DROP TABLE IF EXISTS `aw_ai_sql_example`;
CREATE TABLE `aw_ai_sql_example` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int DEFAULT NULL,
  `question` varchar(500) NOT NULL DEFAULT '',
  `sql` text NOT NULL,
  `db_type` varchar(20) DEFAULT 'mysql',
  `verified` int DEFAULT 0,
  `created_by` int DEFAULT 0,
  `created_at` longtext DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_space_verified` (`space_id`, `verified`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SQL示例';


-- aw_ai_business_glossary
DROP TABLE IF EXISTS `aw_ai_business_glossary`;
CREATE TABLE `aw_ai_business_glossary` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int DEFAULT NULL,
  `term` varchar(200) NOT NULL DEFAULT '' COMMENT '''业务术语/用户可能使用的名称''',
  `table_name` varchar(100) NOT NULL DEFAULT '' COMMENT '''DDL表名''',
  `column_name` varchar(100) NOT NULL DEFAULT '' COMMENT '''DDL字段名''',
  `description` varchar(500) DEFAULT NULL COMMENT '''映射说明''',
  `created_by` int DEFAULT 0,
  `created_at` longtext DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_space_term` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='业务术语表';


-- aw_ai_data_sensitivity_rules
DROP TABLE IF EXISTS `aw_ai_data_sensitivity_rules`;
CREATE TABLE `aw_ai_data_sensitivity_rules` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `space_id` bigint unsigned DEFAULT NULL,
  `table_name` varchar(128) DEFAULT NULL,
  `field_name` varchar(128) DEFAULT NULL,
  `level` varchar(32) DEFAULT NULL,
  `mask_type` varchar(32) DEFAULT NULL,
  `mask_rule` text DEFAULT NULL,
  `description` text DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_space_table_field` (`space_id`, `table_name`, `field_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='数据敏感性规则';


-- aw_ai_memory
DROP TABLE IF EXISTS `aw_ai_memory`;
CREATE TABLE `aw_ai_memory` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int unsigned NOT NULL DEFAULT 0,
  `memory_type` varchar(20) NOT NULL DEFAULT '',
  `content` text NOT NULL,
  `summary` varchar(500) DEFAULT NULL,
  `source_conv_id` int unsigned DEFAULT 0,
  `tags` json DEFAULT NULL,
  `importance` float DEFAULT 0.5,
  `access_count` int unsigned DEFAULT 0,
  `expires_at` datetime DEFAULT NULL,
  `memory_tier` varchar(10) DEFAULT 'short',
  `status` varchar(10) DEFAULT 'confirmed',
  `last_accessed_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_mem_status` (`status`),
  KEY `idx_mem_tier` (`memory_tier`),
  KEY `idx_memory_type` (`memory_type`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_user_importance` (`user_id`, `importance`),
  KEY `idx_user_tier` (`user_id`, `memory_tier`),
  KEY `idx_user_type` (`user_id`, `memory_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI记忆片段';


-- aw_ai_memory_summary
DROP TABLE IF EXISTS `aw_ai_memory_summary`;
CREATE TABLE `aw_ai_memory_summary` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int unsigned NOT NULL DEFAULT 0,
  `conv_id` int unsigned DEFAULT NULL,
  `summary` text NOT NULL,
  `key_points` json DEFAULT NULL,
  `entities` json DEFAULT NULL,
  `token_count` int unsigned DEFAULT 0,
  `message_count` int unsigned DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_conv_id` (`conv_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AI记忆摘要';


-- aw_text2sql_conversation_turn
DROP TABLE IF EXISTS `aw_text2sql_conversation_turn`;
CREATE TABLE `aw_text2sql_conversation_turn` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `conversation_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `space_id` bigint unsigned DEFAULT 0,
  `turn_number` int NOT NULL DEFAULT 0,
  `user_query` text NOT NULL,
  `resolved_query` text DEFAULT NULL,
  `intent_type` varchar(20) DEFAULT 'new_query',
  `generated_sql` text DEFAULT NULL,
  `sql_explanation` text DEFAULT NULL,
  `used_tables` json DEFAULT NULL,
  `used_fields` json DEFAULT NULL,
  `execution_result` text DEFAULT NULL,
  `execution_error` text DEFAULT NULL,
  `is_success` tinyint DEFAULT 0,
  `feedback_rating` tinyint DEFAULT 0,
  `embedding_id` varchar(100) DEFAULT NULL,
  `token_usage` int DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_sql_conv_id` (`conversation_id`),
  KEY `idx_sql_space_id` (`space_id`),
  KEY `idx_sql_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Text2SQL对话轮次';


-- aw_text2sql_conversation_summary
DROP TABLE IF EXISTS `aw_text2sql_conversation_summary`;
CREATE TABLE `aw_text2sql_conversation_summary` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `conversation_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `space_id` bigint unsigned DEFAULT 0,
  `summary` text DEFAULT NULL,
  `key_tables` json DEFAULT NULL,
  `key_fields` json DEFAULT NULL,
  `current_sql` text DEFAULT NULL,
  `turn_count` int DEFAULT 0,
  `success_count` int DEFAULT 0,
  `last_query_pattern` varchar(200) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_conversation_id` (`conversation_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Text2SQL对话摘要';


-- aw_text2sql_turn_embedding
DROP TABLE IF EXISTS `aw_text2sql_turn_embedding`;
CREATE TABLE `aw_text2sql_turn_embedding` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `turn_id` bigint unsigned NOT NULL DEFAULT 0,
  `conversation_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `query_embedding` text DEFAULT NULL,
  `sql_embedding` text DEFAULT NULL,
  `combined_text` text DEFAULT NULL,
  `vector_store_id` varchar(100) DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_turn_id` (`turn_id`),
  KEY `idx_conversation_id` (`conversation_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Text2SQL轮次嵌入';


-- aw_text2sql_session
DROP TABLE IF EXISTS `aw_text2sql_session`;
CREATE TABLE `aw_text2sql_session` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `session_key` varchar(100) NOT NULL DEFAULT '',
  `user_id` bigint unsigned NOT NULL DEFAULT 0,
  `conversation_id` bigint unsigned NOT NULL DEFAULT 0,
  `space_id` bigint unsigned DEFAULT 0,
  `state_json` longtext DEFAULT NULL,
  `is_active` tinyint DEFAULT 1,
  `last_access_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_key` (`session_key`),
  KEY `idx_conversation_id` (`conversation_id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Text2SQL会话';


-- aw_ai_semantic_metric
DROP TABLE IF EXISTS `aw_ai_semantic_metric`;
CREATE TABLE `aw_ai_semantic_metric` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `name` varchar(200) NOT NULL DEFAULT '',
  `code` varchar(100) NOT NULL DEFAULT '',
  `description` text DEFAULT NULL,
  `formula` text NOT NULL,
  `default_table` varchar(100) NOT NULL DEFAULT '',
  `dimensions` text DEFAULT NULL,
  `time_field` varchar(100) DEFAULT NULL,
  `time_granularity` varchar(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sm_space_code` (`space_id`, `code`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='语义指标';


-- aw_ai_semantic_dimension
DROP TABLE IF EXISTS `aw_ai_semantic_dimension`;
CREATE TABLE `aw_ai_semantic_dimension` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `name` varchar(200) NOT NULL DEFAULT '',
  `code` varchar(100) NOT NULL DEFAULT '',
  `field` varchar(100) NOT NULL DEFAULT '',
  `table` varchar(100) NOT NULL DEFAULT '',
  `value_type` varchar(20) DEFAULT 'string',
  `description` text DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sd_space_code` (`space_id`, `code`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='语义维度';


-- aw_ai_column_stats
DROP TABLE IF EXISTS `aw_ai_column_stats`;
CREATE TABLE `aw_ai_column_stats` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `table_name` varchar(100) NOT NULL DEFAULT '',
  `column_name` varchar(100) NOT NULL DEFAULT '',
  `data_type` varchar(50) DEFAULT NULL,
  `row_count` bigint DEFAULT 0,
  `null_count` bigint DEFAULT 0,
  `distinct_count` bigint DEFAULT 0,
  `min_value` varchar(500) DEFAULT NULL,
  `max_value` varchar(500) DEFAULT NULL,
  `avg_value` varchar(500) DEFAULT NULL,
  `sample_values` text DEFAULT NULL,
  `distribution` text DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_cs_table_col` (`space_id`, `table_name`, `column_name`),
  KEY `idx_cs_space_table` (`space_id`, `table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='列统计信息';


-- aw_ai_sql_trace
DROP TABLE IF EXISTS `aw_ai_sql_trace`;
CREATE TABLE `aw_ai_sql_trace` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `trace_id` varchar(64) NOT NULL DEFAULT '',
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `conversation_id` int DEFAULT 0,
  `user_query` text NOT NULL,
  `intent` varchar(50) DEFAULT NULL,
  `steps` longtext DEFAULT NULL,
  `final_sql` longtext DEFAULT NULL,
  `execution_ok` tinyint DEFAULT 0,
  `duration_ms` int DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_trace_id` (`trace_id`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_sql_trace_space_created` (`space_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='SQL追踪';


-- aw_ai_prompt_version
DROP TABLE IF EXISTS `aw_ai_prompt_version`;
CREATE TABLE `aw_ai_prompt_version` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `name` varchar(200) NOT NULL DEFAULT '',
  `query_type` varchar(50) NOT NULL DEFAULT '',
  `version` int NOT NULL DEFAULT 0,
  `system_prompt` text DEFAULT NULL,
  `user_template` text DEFAULT NULL,
  `active` tinyint DEFAULT 0,
  `experiment_id` varchar(64) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_pv_space_query` (`space_id`, `query_type`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='提示词版本';


-- aw_ai_ab_experiment
DROP TABLE IF EXISTS `aw_ai_ab_experiment`;
CREATE TABLE `aw_ai_ab_experiment` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `space_id` bigint unsigned NOT NULL DEFAULT 0,
  `name` varchar(200) NOT NULL DEFAULT '',
  `experiment_id` varchar(64) NOT NULL DEFAULT '',
  `description` text DEFAULT NULL,
  `traffic_a` int DEFAULT 50,
  `traffic_b` int DEFAULT 50,
  `metric_a` int DEFAULT 0,
  `metric_b` int DEFAULT 0,
  `status` tinyint DEFAULT 1,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_experiment_id` (`experiment_id`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='AB实验';


-- aw_document_collab_state
DROP TABLE IF EXISTS `aw_document_collab_state`;
CREATE TABLE `aw_document_collab_state` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `document_id` int unsigned NOT NULL DEFAULT 0,
  `state` mediumblob DEFAULT NULL,
  `version` int NOT NULL DEFAULT 0,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_document_id` (`document_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Document collaborative editing state';


-- aw_document_collab_session
DROP TABLE IF EXISTS `aw_document_collab_session`;
CREATE TABLE `aw_document_collab_session` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `document_id` int unsigned NOT NULL DEFAULT 0,
  `user_id` int unsigned NOT NULL DEFAULT 0,
  `user_name` varchar(100) DEFAULT NULL,
  `color` varchar(7) DEFAULT NULL,
  `connected_at` datetime DEFAULT NULL,
  `last_active_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Document collaborative editing sessions';


-- aw_user
DROP TABLE IF EXISTS `aw_user`;
CREATE TABLE `aw_user` (
  `user_id` int NOT NULL AUTO_INCREMENT,
  `username` varchar(100) NOT NULL DEFAULT '',
  `password` varchar(60) NOT NULL DEFAULT '',
  `given_name` varchar(50) DEFAULT NULL,
  `mobile` varchar(13) DEFAULT NULL,
  `phone` varchar(13) DEFAULT NULL,
  `email` varchar(50) DEFAULT NULL,
  `department` varchar(50) DEFAULT NULL,
  `position` varchar(50) DEFAULT NULL,
  `location` varchar(50) DEFAULT NULL,
  `im` varchar(50) DEFAULT NULL,
  `last_ip` varchar(15) DEFAULT NULL,
  `last_time` int DEFAULT NULL,
  `role_id` int DEFAULT NULL,
  `is_forbidden` int DEFAULT NULL,
  `is_delete` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  `preferences` text DEFAULT NULL,
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';


-- aw_role
DROP TABLE IF EXISTS `aw_role`;
CREATE TABLE `aw_role` (
  `role_id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) DEFAULT NULL,
  `type` int DEFAULT NULL,
  `description` varchar(255) DEFAULT NULL,
  `list_order` int DEFAULT NULL,
  `is_delete` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色表';


-- aw_privilege
DROP TABLE IF EXISTS `aw_privilege`;
CREATE TABLE `aw_privilege` (
  `privilege_id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) DEFAULT NULL,
  `parent_id` int DEFAULT NULL,
  `type` varchar(20) DEFAULT NULL,
  `controller` varchar(100) DEFAULT NULL,
  `action` varchar(100) DEFAULT NULL,
  `icon` varchar(100) DEFAULT NULL,
  `target` varchar(200) DEFAULT NULL,
  `is_display` int DEFAULT NULL,
  `is_menu` int DEFAULT NULL,
  `sequence` int DEFAULT NULL,
  `list_order` int DEFAULT NULL,
  `is_delete` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`privilege_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='权限表';


-- aw_role_privilege
DROP TABLE IF EXISTS `aw_role_privilege`;
CREATE TABLE `aw_role_privilege` (
  `role_privilege_id` int NOT NULL AUTO_INCREMENT,
  `role_id` int DEFAULT NULL,
  `privilege_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`role_privilege_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色权限关联表';


-- aw_space
DROP TABLE IF EXISTS `aw_space`;
CREATE TABLE `aw_space` (
  `space_id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `description` varchar(500) DEFAULT NULL,
  `tags` varchar(255) DEFAULT NULL,
  `status` int DEFAULT NULL,
  `visit_level` varchar(20) DEFAULT NULL,
  `is_share` int DEFAULT NULL,
  `is_export` int DEFAULT NULL,
  `share_password` varchar(100) DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  `create_user_id` int DEFAULT NULL,
  `edit_user_id` int DEFAULT NULL,
  `is_delete` int DEFAULT NULL,
  `sequence` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='空间表';


-- aw_space_user
DROP TABLE IF EXISTS `aw_space_user`;
CREATE TABLE `aw_space_user` (
  `space_user_id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL,
  `space_id` int DEFAULT NULL,
  `privilege` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`space_user_id`),
  UNIQUE KEY `idx_user_space` (`user_id`, `space_id`),
  KEY `idx_space_user` (`space_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='空间成员表';


-- aw_document
DROP TABLE IF EXISTS `aw_document`;
CREATE TABLE `aw_document` (
  `document_id` int NOT NULL AUTO_INCREMENT,
  `parent_id` int DEFAULT NULL,
  `space_id` int DEFAULT NULL,
  `name` varchar(255) NOT NULL DEFAULT '',
  `type` int DEFAULT NULL,
  `path` varchar(500) DEFAULT NULL,
  `sequence` int DEFAULT NULL,
  `icon` varchar(50) DEFAULT '',
  `cover_image` varchar(500) DEFAULT '',
  `create_user_id` int DEFAULT NULL,
  `edit_user_id` int DEFAULT NULL,
  `is_delete` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  `status` varchar(20) DEFAULT 'published',
  `published_at` int DEFAULT NULL,
  `is_immutable` int DEFAULT 0,
  PRIMARY KEY (`document_id`),
  UNIQUE KEY `uk_space_parent_name` (`space_id`, `parent_id`, `name`),
  KEY `idx_create_user_id` (`create_user_id`),
  KEY `idx_edit_user_id` (`edit_user_id`),
  KEY `idx_space_parent_delete` (`space_id`, `parent_id`, `is_delete`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档表';


-- aw_document_content
DROP TABLE IF EXISTS `aw_document_content`;
CREATE TABLE `aw_document_content` (
  `document_id` int NOT NULL,
  `content` mediumtext NOT NULL,
  `content_hash` varchar(64) NOT NULL DEFAULT '',
  `content_size` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`document_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='aw_document_content';


-- aw_document_version
DROP TABLE IF EXISTS `aw_document_version`;
CREATE TABLE `aw_document_version` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `version` int DEFAULT NULL,
  `content` mediumtext DEFAULT NULL,
  `change_log` varchar(500) DEFAULT NULL,
  `created_by` int DEFAULT NULL,
  `created_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_doc_version` (`document_id`, `version`),
  KEY `idx_document_id` (`document_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档版本表';


-- aw_document_view_log
DROP TABLE IF EXISTS `aw_document_view_log`;
CREATE TABLE `aw_document_view_log` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `space_id` int DEFAULT NULL,
  `user_id` int DEFAULT NULL,
  `ip` varchar(45) DEFAULT NULL,
  `user_agent` varchar(500) DEFAULT NULL,
  `viewed_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_viewed_at` (`viewed_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档视图模式记录';


-- aw_document_translation
DROP TABLE IF EXISTS `aw_document_translation`;
CREATE TABLE `aw_document_translation` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `lang` varchar(10) NOT NULL DEFAULT '',
  `translated_document_id` int DEFAULT NULL,
  `translation_status` varchar(20) DEFAULT 'draft',
  `created_at` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_translated_document_id` (`translated_document_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档翻译表';


-- aw_document_link
DROP TABLE IF EXISTS `aw_document_link`;
CREATE TABLE `aw_document_link` (
  `id` int NOT NULL AUTO_INCREMENT,
  `source_doc_id` int DEFAULT NULL,
  `target_doc_id` int DEFAULT NULL,
  `link_text` varchar(500) DEFAULT NULL,
  `space_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_source` (`source_doc_id`),
  KEY `idx_target` (`target_doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档链接表';


-- aw_document_edit_lock
DROP TABLE IF EXISTS `aw_document_edit_lock`;
CREATE TABLE `aw_document_edit_lock` (
  `document_id` int NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL DEFAULT 0,
  `username` varchar(100) DEFAULT NULL,
  `locked_at` datetime NOT NULL,
  `permanent` int DEFAULT 0,
  PRIMARY KEY (`document_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档编辑锁';


-- aw_document_trash
DROP TABLE IF EXISTS `aw_document_trash`;
CREATE TABLE `aw_document_trash` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int NOT NULL DEFAULT 0,
  `parent_id` int DEFAULT NULL,
  `space_id` int NOT NULL DEFAULT 0,
  `name` varchar(255) NOT NULL DEFAULT '',
  `type` int DEFAULT NULL,
  `path` varchar(500) DEFAULT NULL,
  `sequence` int DEFAULT NULL,
  `content_path` varchar(500) DEFAULT NULL,
  `deleted_by` int DEFAULT NULL,
  `deleted_at` int DEFAULT NULL,
  `delete_log` varchar(500) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_deleted_at` (`deleted_at`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档回收站';


-- aw_document_tag
DROP TABLE IF EXISTS `aw_document_tag`;
CREATE TABLE `aw_document_tag` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `tag_name` varchar(100) DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_tag` (`document_id`, `tag_name`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_tag_name` (`tag_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档标签关联表';


-- aw_log_document
DROP TABLE IF EXISTS `aw_log_document`;
CREATE TABLE `aw_log_document` (
  `log_document_id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `space_id` int DEFAULT NULL,
  `user_id` int DEFAULT NULL,
  `action` int DEFAULT NULL,
  `comment` varchar(255) DEFAULT NULL,
  `content` varchar(1000) DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`log_document_id`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_space_id` (`space_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档操作日志表';


-- aw_attachment
DROP TABLE IF EXISTS `aw_attachment`;
CREATE TABLE `aw_attachment` (
  `attachment_id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL,
  `document_id` int DEFAULT NULL,
  `name` varchar(255) DEFAULT NULL,
  `path` varchar(100) DEFAULT NULL,
  `source` int DEFAULT NULL,
  `size` bigint DEFAULT 0,
  `media_type` varchar(32) DEFAULT '',
  `duration` decimal(10,2) DEFAULT 0,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  `chunk_count` int NOT NULL DEFAULT 0,
  `sha256` varchar(64) NOT NULL DEFAULT '',
  `storage_kind` int NOT NULL DEFAULT 2,
  PRIMARY KEY (`attachment_id`),
  UNIQUE KEY `uk_attachment_path` (`path`),
  KEY `idx_document_source` (`document_id`, `source`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='附件表';


-- aw_attachment_chunk
DROP TABLE IF EXISTS `aw_attachment_chunk`;
CREATE TABLE `aw_attachment_chunk` (
  `attachment_id` int NOT NULL,
  `seq` int NOT NULL,
  `len` int NOT NULL DEFAULT 0,
  `data` mediumblob NOT NULL,
  PRIMARY KEY (`attachment_id`, `seq`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='aw_attachment_chunk';


-- aw_collection
DROP TABLE IF EXISTS `aw_collection`;
CREATE TABLE `aw_collection` (
  `collection_id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL,
  `type` int DEFAULT NULL,
  `resource_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`collection_id`),
  UNIQUE KEY `uk_user_resource` (`user_id`, `type`, `resource_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='收藏表';


-- aw_follow
DROP TABLE IF EXISTS `aw_follow`;
CREATE TABLE `aw_follow` (
  `follow_id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL,
  `type` int DEFAULT NULL,
  `object_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`follow_id`),
  UNIQUE KEY `idx_user_object_type` (`user_id`, `type`, `object_id`),
  KEY `idx_object_id` (`object_id`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='关注表';


-- aw_notification
DROP TABLE IF EXISTS `aw_notification`;
CREATE TABLE `aw_notification` (
  `id` int NOT NULL AUTO_INCREMENT,
  `user_id` int DEFAULT NULL,
  `type` varchar(50) DEFAULT NULL,
  `title` varchar(255) DEFAULT NULL,
  `content` varchar(1000) DEFAULT NULL,
  `is_read` int DEFAULT 0,
  `related_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_user_read` (`user_id`, `is_read`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='通知表';


-- aw_config
DROP TABLE IF EXISTS `aw_config`;
CREATE TABLE `aw_config` (
  `config_id` int NOT NULL AUTO_INCREMENT,
  `key` varchar(50) NOT NULL DEFAULT '',
  `value` text DEFAULT NULL,
  `label` varchar(100) DEFAULT NULL,
  `group_key` varchar(50) DEFAULT NULL,
  `type` varchar(20) DEFAULT 'text',
  `options` text DEFAULT NULL,
  `sort` int DEFAULT 0,
  `is_private` int DEFAULT 0,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`config_id`),
  UNIQUE KEY `uk_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统配置表';


-- aw_email
DROP TABLE IF EXISTS `aw_email`;
CREATE TABLE `aw_email` (
  `email_id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `sender_address` varchar(100) NOT NULL DEFAULT '',
  `sender_name` varchar(50) DEFAULT NULL,
  `sender_title_prefix` varchar(50) DEFAULT NULL,
  `host` varchar(100) NOT NULL DEFAULT '',
  `port` varchar(10) NOT NULL DEFAULT 25,
  `username` varchar(100) NOT NULL DEFAULT '',
  `password` varchar(100) NOT NULL DEFAULT '',
  `is_ssl` int DEFAULT 0,
  `is_used` int DEFAULT 0,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`email_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='邮件配置表';


-- aw_login_auth
DROP TABLE IF EXISTS `aw_login_auth`;
CREATE TABLE `aw_login_auth` (
  `login_auth_id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `url` varchar(255) NOT NULL DEFAULT '',
  `username_prefix` varchar(20) NOT NULL DEFAULT '',
  `ext_data` text DEFAULT NULL,
  `is_used` int DEFAULT 0,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`login_auth_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='第三方登录配置表';


-- aw_contact
DROP TABLE IF EXISTS `aw_contact`;
CREATE TABLE `aw_contact` (
  `contact_id` int unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `mobile` varchar(13) NOT NULL DEFAULT '',
  `email` varchar(50) NOT NULL DEFAULT '',
  `position` varchar(100) DEFAULT NULL,
  `create_time` bigint NOT NULL DEFAULT 0,
  `update_time` bigint NOT NULL DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`contact_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='联系人表';


-- aw_link
DROP TABLE IF EXISTS `aw_link`;
CREATE TABLE `aw_link` (
  `link_id` int unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `url` varchar(500) NOT NULL DEFAULT '',
  `sort` int DEFAULT 0,
  `click_count` int DEFAULT 0,
  `is_delete` int DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  `update_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`link_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='快捷链接表';


-- aw_plugin
DROP TABLE IF EXISTS `aw_plugin`;
CREATE TABLE `aw_plugin` (
  `plugin_id` int unsigned NOT NULL AUTO_INCREMENT,
  `plugin_key` varchar(50) DEFAULT NULL,
  `title` longtext DEFAULT NULL,
  `description` longtext DEFAULT NULL,
  `cover_url` longtext DEFAULT NULL,
  `conf_value` longtext DEFAULT NULL,
  `is_used` int DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  `update_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`plugin_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='插件表';


-- aw_log
DROP TABLE IF EXISTS `aw_log`;
CREATE TABLE `aw_log` (
  `log_id` bigint NOT NULL AUTO_INCREMENT,
  `level` int DEFAULT NULL,
  `url` varchar(500) DEFAULT NULL,
  `method` varchar(10) DEFAULT NULL,
  `param` text DEFAULT NULL,
  `path` varchar(100) DEFAULT NULL,
  `get` text DEFAULT NULL,
  `post` text DEFAULT NULL,
  `message` varchar(255) DEFAULT NULL,
  `ip` varchar(100) DEFAULT NULL,
  `user_agent` varchar(200) DEFAULT NULL,
  `referer` varchar(100) DEFAULT NULL,
  `user_id` int DEFAULT NULL,
  `username` varchar(100) DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`log_id`),
  KEY `idx_create_time` (`create_time`),
  KEY `idx_level_create_time` (`level`, `create_time`),
  KEY `idx_level_username` (`level`, `username`),
  KEY `idx_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统日志表';


-- aw_comment
DROP TABLE IF EXISTS `aw_comment`;
CREATE TABLE `aw_comment` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `parent_id` int DEFAULT 0,
  `resolved` int DEFAULT 0,
  `content` text DEFAULT NULL,
  `selection_text` text DEFAULT NULL,
  `selection_range` varchar(100) DEFAULT NULL,
  `creator_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_creator_id` (`creator_id`),
  KEY `idx_document_id` (`document_id`),
  KEY `idx_parent_id` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='评论表';


-- aw_document_mention
DROP TABLE IF EXISTS `aw_document_mention`;
CREATE TABLE `aw_document_mention` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `user_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_user` (`document_id`, `user_id`),
  KEY `idx_mention_doc` (`document_id`),
  KEY `idx_mention_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档@提及表';


-- aw_document_approval
DROP TABLE IF EXISTS `aw_document_approval`;
CREATE TABLE `aw_document_approval` (
  `id` int NOT NULL AUTO_INCREMENT,
  `document_id` int DEFAULT NULL,
  `space_id` int DEFAULT NULL,
  `submitter` int DEFAULT NULL,
  `approver` int DEFAULT NULL,
  `status` varchar(20) DEFAULT NULL,
  `chain_type` varchar(20) DEFAULT 'sequential',
  `comment` varchar(1000) DEFAULT NULL,
  `submit_time` int DEFAULT NULL,
  `handle_time` int DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_approval_doc` (`document_id`),
  KEY `idx_approval_space` (`space_id`),
  KEY `idx_approval_status` (`status`, `approver`),
  KEY `idx_approver` (`approver`),
  KEY `idx_submitter` (`submitter`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档审批单';


-- aw_document_approval_step
DROP TABLE IF EXISTS `aw_document_approval_step`;
CREATE TABLE `aw_document_approval_step` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `approval_id` bigint NOT NULL DEFAULT 0,
  `step_order` int NOT NULL DEFAULT 0,
  `approver_id` bigint NOT NULL DEFAULT 0,
  `status` varchar(20) NOT NULL DEFAULT 'pending',
  `comment` varchar(1000) DEFAULT '',
  `handle_time` bigint NOT NULL DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_step_approval` (`approval_id`),
  KEY `idx_step_approver` (`approver_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='审批步骤表';


-- aw_document_template
DROP TABLE IF EXISTS `aw_document_template`;
CREATE TABLE `aw_document_template` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int DEFAULT NULL,
  `name` varchar(100) DEFAULT NULL,
  `description` varchar(255) DEFAULT NULL,
  `content` longtext DEFAULT NULL,
  `create_user_id` int DEFAULT NULL,
  `create_time` int NOT NULL DEFAULT 0,
  `update_time` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tpl_space_name` (`name`),
  KEY `idx_create_user_id` (`create_user_id`),
  KEY `idx_tpl_space` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档模板表';


-- aw_document_version_tag
DROP TABLE IF EXISTS `aw_document_version_tag`;
CREATE TABLE `aw_document_version_tag` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `document_id` bigint NOT NULL DEFAULT 0,
  `version` int NOT NULL DEFAULT 0,
  `tag_name` varchar(100) NOT NULL DEFAULT '',
  `color` varchar(20) DEFAULT '',
  `note` varchar(500) DEFAULT '',
  `created_by` bigint NOT NULL DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_vtag_doc` (`document_id`),
  KEY `idx_vtag_name` (`tag_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档版本标签/里程碑表';


-- aw_tag_hierarchy
DROP TABLE IF EXISTS `aw_tag_hierarchy`;
CREATE TABLE `aw_tag_hierarchy` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `parent_tag` varchar(100) NOT NULL DEFAULT '',
  `child_tag` varchar(100) NOT NULL DEFAULT '',
  `space_id` bigint NOT NULL DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tag_child` (`child_tag`),
  KEY `idx_tag_parent` (`parent_tag`),
  KEY `idx_tag_space` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='标签层级表';


-- aw_saved_search
DROP TABLE IF EXISTS `aw_saved_search`;
CREATE TABLE `aw_saved_search` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL DEFAULT 0,
  `name` varchar(100) NOT NULL DEFAULT '',
  `query` varchar(500) NOT NULL DEFAULT '',
  `type` varchar(20) DEFAULT 'all',
  `space_id` bigint DEFAULT 0,
  `tag_filter` varchar(100) DEFAULT '',
  `sort_by` varchar(20) DEFAULT 'relevance',
  `is_public` int DEFAULT 0,
  `use_count` int DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  `update_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_saved_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='保存的搜索表';


-- aw_webhook_config
DROP TABLE IF EXISTS `aw_webhook_config`;
CREATE TABLE `aw_webhook_config` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL DEFAULT 0,
  `space_id` bigint NOT NULL DEFAULT 0,
  `name` varchar(100) NOT NULL DEFAULT '',
  `url` varchar(500) NOT NULL DEFAULT '',
  `secret` varchar(100) DEFAULT '',
  `events` varchar(500) NOT NULL DEFAULT '',
  `is_active` int DEFAULT 1,
  `create_time` bigint NOT NULL DEFAULT 0,
  `update_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_webhook_space` (`space_id`),
  KEY `idx_webhook_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Webhook 配置表';


-- aw_webhook_delivery
DROP TABLE IF EXISTS `aw_webhook_delivery`;
CREATE TABLE `aw_webhook_delivery` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `webhook_id` bigint NOT NULL DEFAULT 0,
  `event` varchar(50) NOT NULL DEFAULT '',
  `payload` text DEFAULT NULL,
  `status_code` int NOT NULL DEFAULT 0,
  `response` varchar(1000) DEFAULT '',
  `success` int DEFAULT 0,
  `retry_count` int DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_delivery_webhook` (`webhook_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Webhook 投递记录表';


-- aw_api_key
DROP TABLE IF EXISTS `aw_api_key`;
CREATE TABLE `aw_api_key` (
  `id` int NOT NULL AUTO_INCREMENT,
  `app_name` varchar(128) NOT NULL DEFAULT '',
  `owner_id` int NOT NULL DEFAULT 0,
  `key_hash` varchar(128) NOT NULL DEFAULT '',
  `key_enc` varchar(512) NOT NULL DEFAULT '',
  `status` varchar(16) NOT NULL DEFAULT 'active',
  `rate_limit` int NOT NULL DEFAULT 60,
  `expires_at` datetime DEFAULT NULL,
  `last_used_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_key_hash` (`key_hash`),
  KEY `idx_apikey_status` (`status`),
  KEY `idx_owner_id` (`owner_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='开放平台API Key';


-- aw_document_permission
DROP TABLE IF EXISTS `aw_document_permission`;
CREATE TABLE `aw_document_permission` (
  `id` int NOT NULL AUTO_INCREMENT,
  `doc_id` int NOT NULL DEFAULT 0,
  `space_id` int NOT NULL DEFAULT 0,
  `subject_type` varchar(16) NOT NULL DEFAULT '',
  `subject_id` int NOT NULL DEFAULT 0,
  `effect` varchar(8) NOT NULL DEFAULT '',
  `created_by` int DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_doc_subject` (`doc_id`, `subject_type`, `subject_id`),
  KEY `idx_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='aw_document_permission';


-- aw_oidc_provider
DROP TABLE IF EXISTS `aw_oidc_provider`;
CREATE TABLE `aw_oidc_provider` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `display_name` varchar(100) DEFAULT NULL,
  `type` varchar(20) NOT NULL DEFAULT 'oidc',
  `client_id` varchar(255) NOT NULL DEFAULT '',
  `client_secret` varchar(512) NOT NULL DEFAULT '',
  `issuer_url` varchar(512) DEFAULT NULL,
  `auth_url` varchar(512) DEFAULT NULL,
  `token_url` varchar(512) DEFAULT NULL,
  `user_info_url` varchar(512) DEFAULT NULL,
  `scopes` varchar(512) DEFAULT 'openid,profile,email',
  `enabled` tinyint DEFAULT 1,
  `auto_create` tinyint DEFAULT 1,
  `default_role` varchar(20) DEFAULT 'viewer',
  `icon_url` varchar(512) DEFAULT NULL,
  `sort_order` int DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='OIDC 身份提供方配置';


-- aw_oidc_user_mapping
DROP TABLE IF EXISTS `aw_oidc_user_mapping`;
CREATE TABLE `aw_oidc_user_mapping` (
  `id` int unsigned NOT NULL AUTO_INCREMENT,
  `user_id` int NOT NULL DEFAULT 0,
  `provider_id` int unsigned NOT NULL DEFAULT 0,
  `subject` varchar(255) NOT NULL DEFAULT '',
  `email` varchar(255) DEFAULT NULL,
  `raw_info` text DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_oidc_user` (`user_id`, `provider_id`),
  KEY `idx_oidc_provider_subject` (`provider_id`, `subject`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户 OIDC 身份映射';


-- aw_schema_space
DROP TABLE IF EXISTS `aw_schema_space`;
CREATE TABLE `aw_schema_space` (
  `id` int NOT NULL AUTO_INCREMENT,
  `space_id` int NOT NULL DEFAULT 0,
  `name` varchar(255) NOT NULL DEFAULT '',
  `db_type` varchar(50) DEFAULT 'mysql',
  `status` int DEFAULT 1,
  `last_scan_at` datetime DEFAULT NULL,
  `doc_count` int DEFAULT 0,
  `table_count` int DEFAULT 0,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_space_id` (`space_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema空间';


-- aw_schema_table
DROP TABLE IF EXISTS `aw_schema_table`;
CREATE TABLE `aw_schema_table` (
  `id` int NOT NULL AUTO_INCREMENT,
  `schema_space_id` int NOT NULL DEFAULT 0,
  `table_name` varchar(128) NOT NULL DEFAULT '',
  `table_comment` varchar(500) DEFAULT NULL,
  `engine` varchar(50) DEFAULT NULL,
  `charset` varchar(50) DEFAULT NULL,
  `primary_key` varchar(255) DEFAULT NULL,
  `doc_id` int DEFAULT NULL,
  `doc_path` varchar(500) DEFAULT NULL,
  `raw_ddl` text DEFAULT NULL,
  `status` int DEFAULT 1,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_schema_table` (`schema_space_id`, `table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema表';


-- aw_schema_column
DROP TABLE IF EXISTS `aw_schema_column`;
CREATE TABLE `aw_schema_column` (
  `id` int NOT NULL AUTO_INCREMENT,
  `schema_space_id` int NOT NULL DEFAULT 0,
  `table_id` int NOT NULL DEFAULT 0,
  `table_name` varchar(128) NOT NULL DEFAULT '',
  `column_name` varchar(128) NOT NULL DEFAULT '',
  `data_type` varchar(50) NOT NULL DEFAULT '',
  `length` int DEFAULT NULL,
  `nullable` int DEFAULT 1,
  `default_value` varchar(255) DEFAULT NULL,
  `column_comment` varchar(500) DEFAULT NULL,
  `is_primary_key` int DEFAULT 0,
  `is_foreign_key` int DEFAULT 0,
  `is_index` int DEFAULT 0,
  `ordinal_position` int DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_table_column` (`table_id`, `column_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema列';


-- aw_schema_relation
DROP TABLE IF EXISTS `aw_schema_relation`;
CREATE TABLE `aw_schema_relation` (
  `id` int NOT NULL AUTO_INCREMENT,
  `schema_space_id` int NOT NULL DEFAULT 0,
  `source_table` varchar(128) NOT NULL DEFAULT '',
  `source_column` varchar(128) NOT NULL DEFAULT '',
  `target_table` varchar(128) NOT NULL DEFAULT '',
  `target_column` varchar(128) NOT NULL DEFAULT '',
  `relation_type` varchar(50) DEFAULT 'one_to_many',
  `confidence` float DEFAULT 1.0,
  `source` varchar(20) DEFAULT 'ddl',
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_relation` (`schema_space_id`, `source_table`, `source_column`, `target_table`, `target_column`),
  KEY `idx_target` (`schema_space_id`, `target_table`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema关系';


-- aw_schema_business_tag
DROP TABLE IF EXISTS `aw_schema_business_tag`;
CREATE TABLE `aw_schema_business_tag` (
  `id` int NOT NULL AUTO_INCREMENT,
  `schema_space_id` int NOT NULL DEFAULT 0,
  `table_id` int DEFAULT NULL,
  `table_name` varchar(128) DEFAULT NULL,
  `tag_name` varchar(100) NOT NULL DEFAULT '',
  `tag_type` varchar(50) DEFAULT 'auto',
  `confidence` float DEFAULT 1.0,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_schema_tag` (`schema_space_id`, `tag_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema业务标签';


-- aw_schema_scan_task
DROP TABLE IF EXISTS `aw_schema_scan_task`;
CREATE TABLE `aw_schema_scan_task` (
  `id` int NOT NULL AUTO_INCREMENT,
  `schema_space_id` int NOT NULL DEFAULT 0,
  `status` varchar(20) DEFAULT 'pending',
  `total_docs` int DEFAULT 0,
  `processed_docs` int DEFAULT 0,
  `found_tables` int DEFAULT 0,
  `error_message` text DEFAULT NULL,
  `started_at` datetime DEFAULT NULL,
  `completed_at` datetime DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Schema扫描任务';


-- aw_eval_dataset
DROP TABLE IF EXISTS `aw_eval_dataset`;
CREATE TABLE `aw_eval_dataset` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL DEFAULT '',
  `space_id` int NOT NULL DEFAULT 0,
  `questions` text DEFAULT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='RAG评估数据集';


-- aw_eval_result
DROP TABLE IF EXISTS `aw_eval_result`;
CREATE TABLE `aw_eval_result` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `dataset_id` bigint NOT NULL DEFAULT 0,
  `dataset_name` varchar(255) DEFAULT NULL,
  `space_id` int NOT NULL DEFAULT 0,
  `overall_score` decimal(5,4) DEFAULT NULL,
  `metrics_json` text DEFAULT NULL,
  `details_json` longtext DEFAULT NULL,
  `duration` varchar(32) DEFAULT NULL,
  `status` varchar(32) DEFAULT 'completed',
  `created_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_dataset_id` (`dataset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='RAG评估结果';


-- aw_search_log
DROP TABLE IF EXISTS `aw_search_log`;
CREATE TABLE `aw_search_log` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `keyword` varchar(100) NOT NULL DEFAULT '',
  `query` varchar(500) NOT NULL DEFAULT '',
  `source` varchar(20) NOT NULL DEFAULT 'fulltext',
  `type` varchar(20) NOT NULL DEFAULT '',
  `space_id` int NOT NULL DEFAULT 0,
  `user_id` int NOT NULL DEFAULT 0,
  `result_count` int NOT NULL DEFAULT 0,
  `latency_ms` bigint NOT NULL DEFAULT 0,
  `create_time` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_search_log_time_keyword` (`create_time`, `keyword`),
  KEY `idx_search_log_user_time` (`user_id`, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='搜索日志表';


SET FOREIGN_KEY_CHECKS = 1;
