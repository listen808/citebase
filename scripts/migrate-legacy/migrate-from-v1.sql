-- ============================================================
-- Citebase 旧库(v1, `mw_*`) → 新库(v2, `aw_*`) 数据迁移
-- ============================================================
-- 适用: dump-citebase-202609271553.sql（MySQL 5.7 mysqldump，86 张 `mw_*` 表）
-- 目标: Citebase v2 已初始化完成的 `citebase` 库（91 张 `aw_*` 表）
--
-- 前置（run-migrate.ps1 会自动做，手动执行见下）:
--   1) CREATE DATABASE citebase_legacy CHARACTER SET utf8mb4;
--   2) mysql --default-character-set=utf8mb4 citebase_legacy < databases/dump-citebase-202609271553.sql
--      绝不把 dump 还原进目标库：那会往生产库里塞 86 张 mw_* 表。
--   3) mysql --default-character-set=utf8mb4 citebase_legacy < scripts/migrate-legacy/migrate-from-v1.sql
--      （SQL 内部用 `citebase`.`aw_x` 全限定名写目标库，所以连接时选历史库即可）
--
-- 库名改写: 目标库/历史库名只出现在两处 —— 下面的 `USE` 行 和 `citebase`.`aw_x` 前缀。
--          非默认库名用 run-migrate.ps1 -TargetDb/-LegacyDb 自动替换，勿手工改。
--
-- 幂等: 全部写入用 INSERT IGNORE + 主键沿用旧值，重复执行只补齐缺失行，不产生副本。
--       出错即停（mysql 默认遇错退出）；中途失败可直接重跑，或按 run-migrate.ps1 的备份回滚。
--       执行前请先备份目标库。
--
-- ⚠ 本脚本只搬数据库。文档正文(.md)、图片、附件在旧服务器的磁盘上，必须另行搬迁，
--   见第 7 节清单。数据库里没有正文：aw_document 无 content 列，正文按
--   <空间名>/<父级目录名...>/<文档名>.md（目录则为 .../<文档名>/README.md）寻址。
-- ============================================================

SET NAMES utf8mb4;

USE `citebase_legacy`;

-- ============================================================
-- 1. 预检报告（只读，不改数据）
-- ============================================================
SELECT '== 1. 预检 ==' AS `步骤`;

SELECT COUNT(*) AS `历史库_mw_表数量`,
       (SELECT COUNT(*) FROM information_schema.tables
         WHERE table_schema = 'citebase' AND table_name LIKE 'aw\_%') AS `目标库_aw_表数量`,
       (SELECT COUNT(*) FROM information_schema.tables
         WHERE table_schema = 'citebase_legacy' AND table_name LIKE 'mw\_%') AS `预期_86`
FROM information_schema.tables
WHERE table_schema = 'citebase_legacy' AND table_name LIKE 'mw\_%';

-- 目标库必须已建表（91 张）。为 0 说明 Citebase 还没启动过 / 库名写错，先初始化再迁移。
-- （这里只是提示：真缺表时第 3 节的第一条 INSERT 会直接报错中断，mysql 默认遇错退出。）
SELECT IF((SELECT COUNT(*) FROM information_schema.tables
             WHERE table_schema = 'citebase' AND table_name LIKE 'aw\_%') >= 91,
          'OK: 目标库表结构已就绪',
          '!! 目标库不足 91 张 aw_* 表：请先启动 Citebase 完成 AutoMigrate 或导入 databases/table.sql')
  AS `检查_目标库表结构`;

-- 旧库同名兄弟文档会撞上新增唯一键 uk_space_parent_name(space_id,parent_id,name)，
-- 不处理就会静默丢文档。第 2 节自动改名，这里只报规模。
SELECT COUNT(*) AS `同名兄弟冲突组数` FROM (
  SELECT 1 FROM `mw_document` GROUP BY `space_id`, `parent_id`, `name` HAVING COUNT(*) > 1
) t;

-- 旧库里已删除空间的残留文档：无家可归，第 3 节按 space 存在性过滤掉，此处报明细。
SELECT d.`document_id` AS `文档ID`, d.`space_id` AS `空间ID`, d.`parent_id` AS `父文档`, d.`name` AS `标题`,
       d.`is_delete` AS `已删除`
  FROM `mw_document` d
  LEFT JOIN `mw_space` s ON s.`space_id` = d.`space_id`
 WHERE s.`space_id` IS NULL;

-- 旧账号/空间规模，用于和第 5 节结果对照
SELECT (SELECT COUNT(*) FROM `mw_user`)        AS `旧_用户`,
       (SELECT COUNT(*) FROM `mw_space`)       AS `旧_空间`,
       (SELECT COUNT(*) FROM `mw_document`)    AS `旧_文档`,
       (SELECT COUNT(*) FROM `mw_attachment`)  AS `旧_附件`,
       (SELECT COUNT(*) FROM `mw_document_version`) AS `旧_版本历史`;

-- ============================================================
-- 2. 冲突消解：同级重名文档改名（只改历史库副本，不碰 dump 文件）
--    新库多了唯一键 uk_space_parent_name，同名兄弟会被 INSERT IGNORE 静默丢掉整行，
--    所以必须先消解。规则: 同组保留 document_id 最大的一行，其余改名为「原标题 (文档ID)」。
--    本份 dump: 13 组 / 15 行需改名，且全部是 is_delete=1 的回收站文档（存活文档不改名），
--    一轮即收敛，无二次冲突。
-- ============================================================
SELECT '== 2. 同级重名改名 ==' AS `步骤`;

DROP TEMPORARY TABLE IF EXISTS `tmp_doc_dup`;
CREATE TEMPORARY TABLE `tmp_doc_dup` AS
SELECT `space_id`, `parent_id`, `name`, MAX(`document_id`) AS `keep_id`, COUNT(*) AS `cnt`
  FROM `mw_document`
 GROUP BY `space_id`, `parent_id`, `name`
HAVING COUNT(*) > 1;

-- 改名清单：迁移磁盘文件时按此对照重命名。type: 1=文档页(<父目录>/<名>.md) 2=目录(<父目录>/<名>/README.md)
SELECT t.`space_id` AS `空间ID`, sp.`name` AS `空间名`, t.`parent_id` AS `父文档ID`, p.`name` AS `父文档名`,
       d.`document_id` AS `被改名文档ID`, d.`type` AS `类型`,
       (SELECT COUNT(*) FROM `mw_document` c WHERE c.`parent_id` = d.`document_id`) AS `子文档数`,
       d.`name` AS `原标题`, CONCAT(d.`name`, ' (', d.`document_id`, ')') AS `新标题`,
       CONCAT(COALESCE(p.`name`, sp.`name`), '/', d.`name`, IF(d.`type` = 2, '/README.md', '.md')) AS `对应文件`
  FROM `tmp_doc_dup` t
  JOIN `mw_document` d ON d.`space_id` = t.`space_id` AND d.`parent_id` = t.`parent_id` AND d.`name` = t.`name`
  LEFT JOIN `mw_document` p ON p.`document_id` = t.`parent_id`
  LEFT JOIN `mw_space` sp ON sp.`space_id` = t.`space_id`
 WHERE d.`document_id` <> t.`keep_id`
 ORDER BY t.`space_id`, t.`parent_id`, d.`document_id`;

UPDATE `mw_document` d
  JOIN `tmp_doc_dup` t ON t.`space_id` = d.`space_id` AND t.`parent_id` = d.`parent_id` AND t.`name` = d.`name`
   SET d.`name` = CONCAT(d.`name`, ' (', d.`document_id`, ')')
 WHERE d.`document_id` <> t.`keep_id`;

DROP TEMPORARY TABLE IF EXISTS `tmp_doc_dup`;

-- 改名后必须为 0，否则第 3 节会丢文档（例如原本就有「xxx (12)」这种标题）
SELECT IF(NOT EXISTS (SELECT 1 FROM `mw_document` GROUP BY `space_id`, `parent_id`, `name` HAVING COUNT(*) > 1),
          'OK: 已无同名兄弟冲突', '!! 仍有同名冲突，请手工处理后再执行第 3 节') AS `检查_改名结果`;

-- ============================================================
-- 3. 业务数据迁移（按依赖顺序）
-- ============================================================
SELECT '== 3. 写入目标库 ==' AS `步骤`;
SET FOREIGN_KEY_CHECKS = 0;

-- 3.1 用户（密码沿用旧 bcrypt 散列，旧账号密码可直接登录；覆盖逻辑见 4.3）
INSERT IGNORE INTO `citebase`.`aw_user`
  (`user_id`, `username`, `password`, `given_name`, `mobile`, `phone`, `email`, `department`,
   `position`, `location`, `im`, `last_ip`, `last_time`, `role_id`, `is_forbidden`, `is_delete`,
   `create_time`, `update_time`)
SELECT s.`user_id`, s.`username`, s.`password`, s.`given_name`, s.`mobile`, s.`phone`, s.`email`,
       s.`department`, s.`position`, s.`location`, s.`im`, s.`last_ip`, s.`last_time`, s.`role_id`,
       s.`is_forbidden`, s.`is_delete`, COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_user` s;

-- 3.2 空间
INSERT IGNORE INTO `citebase`.`aw_space`
  (`space_id`, `name`, `description`, `tags`, `status`, `visit_level`, `is_share`, `is_export`,
   `share_password`, `created_by`, `create_user_id`, `edit_user_id`, `is_delete`, `sequence`,
   `create_time`, `update_time`)
SELECT s.`space_id`, s.`name`, s.`description`, s.`tags`, s.`status`, s.`visit_level`, s.`is_share`,
       s.`is_export`, s.`share_password`, s.`created_by`, s.`create_user_id`, s.`edit_user_id`,
       s.`is_delete`, s.`sequence`, COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_space` s;

-- 3.3 空间成员
INSERT IGNORE INTO `citebase`.`aw_space_user`
  (`space_user_id`, `user_id`, `space_id`, `privilege`, `create_time`, `update_time`)
SELECT s.`space_user_id`, s.`user_id`, s.`space_id`, s.`privilege`,
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_space_user` s;

-- 3.4 文档树（跳过所属空间已不存在的孤儿文档）
INSERT IGNORE INTO `citebase`.`aw_document`
  (`document_id`, `parent_id`, `space_id`, `name`, `type`, `path`, `sequence`, `icon`, `cover_image`,
   `create_user_id`, `edit_user_id`, `is_delete`, `create_time`, `update_time`, `status`, `published_at`)
SELECT s.`document_id`, s.`parent_id`, s.`space_id`, s.`name`, s.`type`, s.`path`, s.`sequence`,
       s.`icon`, s.`cover_image`, s.`create_user_id`, s.`edit_user_id`, s.`is_delete`,
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0), s.`status`, s.`published_at`
  FROM `mw_document` s
  JOIN `citebase`.`aw_space` sp ON sp.`space_id` = s.`space_id`;

-- 3.5 文档版本历史（正文快照在 content 列里，是旧库唯一带正文的数据）
INSERT IGNORE INTO `citebase`.`aw_document_version`
  (`id`, `document_id`, `version`, `content`, `change_log`, `created_by`, `created_at`)
SELECT s.`id`, s.`document_id`, s.`version`, s.`content`, s.`change_log`, s.`created_by`, s.`created_at`
  FROM `mw_document_version` s;

-- 3.6 回收站
INSERT IGNORE INTO `citebase`.`aw_document_trash`
  (`id`, `document_id`, `parent_id`, `space_id`, `name`, `type`, `path`, `sequence`, `content_path`,
   `deleted_by`, `deleted_at`, `delete_log`)
SELECT s.`id`, s.`document_id`, s.`parent_id`, s.`space_id`, s.`name`, s.`type`, s.`path`, s.`sequence`,
       s.`content_path`, s.`deleted_by`, s.`deleted_at`, s.`delete_log`
  FROM `mw_document_trash` s;

-- 3.7 附件与图片索引（只搬索引行，文件本身按第 7 节搬迁）
INSERT IGNORE INTO `citebase`.`aw_attachment`
  (`attachment_id`, `user_id`, `document_id`, `name`, `path`, `source`, `size`, `media_type`,
   `duration`, `create_time`, `update_time`)
SELECT s.`attachment_id`, s.`user_id`, s.`document_id`, s.`name`, s.`path`, s.`source`, s.`size`,
       s.`media_type`, s.`duration`, COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_attachment` s;

-- 3.8 评论
INSERT IGNORE INTO `citebase`.`aw_comment`
  (`id`, `document_id`, `parent_id`, `resolved`, `content`, `selection_text`, `selection_range`,
   `creator_id`, `create_time`, `update_time`)
SELECT s.`id`, s.`document_id`, s.`parent_id`, s.`resolved`, s.`content`, s.`selection_text`,
       s.`selection_range`, s.`creator_id`, COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_comment` s;

-- 3.9 收藏
INSERT IGNORE INTO `citebase`.`aw_collection`
  (`collection_id`, `user_id`, `type`, `resource_id`, `create_time`, `update_time`)
SELECT s.`collection_id`, s.`user_id`, s.`type`, s.`resource_id`,
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_collection` s;

-- 3.10 关注（旧表的 `id` 列在新表已不存在，只搬 follow_id 及以下）
INSERT IGNORE INTO `citebase`.`aw_follow`
  (`follow_id`, `user_id`, `type`, `object_id`, `create_time`, `update_time`)
SELECT s.`follow_id`, s.`user_id`, s.`type`, s.`object_id`,
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_follow` s;

-- 3.11 快捷链接
INSERT IGNORE INTO `citebase`.`aw_link`
  (`link_id`, `name`, `url`, `sort`, `is_delete`, `create_time`, `update_time`)
SELECT s.`link_id`, s.`name`, s.`url`, s.`sort`, s.`is_delete`,
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_link` s;

-- 3.12 联系人
INSERT IGNORE INTO `citebase`.`aw_contact`
  (`contact_id`, `name`, `mobile`, `email`, `position`, `create_time`, `update_time`, `created_at`, `updated_at`)
SELECT s.`contact_id`, s.`name`, s.`mobile`, s.`email`, s.`position`,
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0), s.`created_at`, s.`updated_at`
  FROM `mw_contact` s;

-- ============================================================
-- 4. 配置与口令（跨库字符串比较必须显式 COLLATE，否则 illegal mix of collations）
-- ============================================================
SELECT '== 4. 配置与口令 ==' AS `步骤`;

-- 4.1 新库已有的配置项：只取旧库的值，标签/分组/控件类型以 v2 为准（v2 的 label 才是界面文案）。
--     system_version 例外：旧值 v0.2.1 会把新版本号显示成旧版本，跳过。
UPDATE `citebase`.`aw_config` c
  JOIN `mw_config` s ON s.`key` COLLATE utf8mb4_unicode_ci = c.`key`
   SET c.`value` = s.`value`, c.`update_time` = COALESCE(s.`update_time`, c.`update_time`)
 WHERE s.`key` <> 'system_version';

-- 4.2 旧库独有的配置项（如 theme）整行插入，label 取旧库 name 列
INSERT IGNORE INTO `citebase`.`aw_config`
  (`key`, `value`, `label`, `group_key`, `type`, `options`, `sort`, `is_private`, `create_time`, `update_time`)
SELECT s.`key`, s.`value`,
       IF(s.`name` IS NULL OR s.`name` = '', s.`key`, s.`name`),
       IFNULL(NULLIF(s.`group_key`, ''), 'system'),
       IFNULL(NULLIF(s.`type`, ''), 'text'),
       s.`options`, COALESCE(s.`sort`, 0), COALESCE(s.`is_private`, 0),
       COALESCE(s.`create_time`, 0), COALESCE(s.`update_time`, 0)
  FROM `mw_config` s
  LEFT JOIN `citebase`.`aw_config` c ON c.`key` = s.`key` COLLATE utf8mb4_unicode_ci
 WHERE c.`key` IS NULL;

-- 4.3 同名账号沿用旧密码散列，让所有人用原来的密码登录。
--     若目标库 admin 的随机初始密码更合适，注释掉本段（其余 9 个账号仍会作为新用户导入）。
UPDATE `citebase`.`aw_user` u
  JOIN `mw_user` s ON s.`username` COLLATE utf8mb4_unicode_ci = u.`username`
   SET u.`password` = s.`password`;

-- ============================================================
-- 5. 校验报告
--    `未写入` = 旧库有、目标库因主键/唯一键冲突没进去的行数，应逐条确认原因
-- ============================================================
SELECT '== 5. 校验（旧库行数 vs 已写入目标库） ==' AS `步骤`;

SELECT `表`, `旧库行数`, `已写入`, `未写入` FROM (
  SELECT 1 AS o, 'aw_user' AS `表`, COUNT(s.`user_id`) AS `旧库行数`, COUNT(u.`user_id`) AS `已写入`,
         COUNT(s.`user_id`) - COUNT(u.`user_id`) AS `未写入`
    FROM `mw_user` s LEFT JOIN `citebase`.`aw_user` u ON u.`user_id` = s.`user_id`
  UNION ALL SELECT 2, 'aw_space', COUNT(s.`space_id`), COUNT(t.`space_id`), COUNT(s.`space_id`) - COUNT(t.`space_id`)
    FROM `mw_space` s LEFT JOIN `citebase`.`aw_space` t ON t.`space_id` = s.`space_id`
  UNION ALL SELECT 3, 'aw_space_user', COUNT(s.`space_user_id`), COUNT(t.`space_user_id`), COUNT(s.`space_user_id`) - COUNT(t.`space_user_id`)
    FROM `mw_space_user` s LEFT JOIN `citebase`.`aw_space_user` t ON t.`space_user_id` = s.`space_user_id`
  UNION ALL SELECT 4, 'aw_document(可迁移范围)', COUNT(s.`document_id`), COUNT(t.`document_id`), COUNT(s.`document_id`) - COUNT(t.`document_id`)
    FROM `mw_document` s
    JOIN `citebase`.`aw_space` sp ON sp.`space_id` = s.`space_id`
    LEFT JOIN `citebase`.`aw_document` t ON t.`document_id` = s.`document_id`
  UNION ALL SELECT 5, 'aw_document_version', COUNT(s.`id`), COUNT(t.`id`), COUNT(s.`id`) - COUNT(t.`id`)
    FROM `mw_document_version` s LEFT JOIN `citebase`.`aw_document_version` t ON t.`id` = s.`id`
  UNION ALL SELECT 6, 'aw_document_trash', COUNT(s.`id`), COUNT(t.`id`), COUNT(s.`id`) - COUNT(t.`id`)
    FROM `mw_document_trash` s LEFT JOIN `citebase`.`aw_document_trash` t ON t.`id` = s.`id`
  UNION ALL SELECT 7, 'aw_attachment', COUNT(s.`attachment_id`), COUNT(t.`attachment_id`), COUNT(s.`attachment_id`) - COUNT(t.`attachment_id`)
    FROM `mw_attachment` s LEFT JOIN `citebase`.`aw_attachment` t ON t.`attachment_id` = s.`attachment_id`
  UNION ALL SELECT 8, 'aw_comment', COUNT(s.`id`), COUNT(t.`id`), COUNT(s.`id`) - COUNT(t.`id`)
    FROM `mw_comment` s LEFT JOIN `citebase`.`aw_comment` t ON t.`id` = s.`id`
  UNION ALL SELECT 9, 'aw_collection', COUNT(s.`collection_id`), COUNT(t.`collection_id`), COUNT(s.`collection_id`) - COUNT(t.`collection_id`)
    FROM `mw_collection` s LEFT JOIN `citebase`.`aw_collection` t ON t.`collection_id` = s.`collection_id`
  UNION ALL SELECT 10, 'aw_follow', COUNT(s.`follow_id`), COUNT(t.`follow_id`), COUNT(s.`follow_id`) - COUNT(t.`follow_id`)
    FROM `mw_follow` s LEFT JOIN `citebase`.`aw_follow` t ON t.`follow_id` = s.`follow_id`
  UNION ALL SELECT 11, 'aw_link', COUNT(s.`link_id`), COUNT(t.`link_id`), COUNT(s.`link_id`) - COUNT(t.`link_id`)
    FROM `mw_link` s LEFT JOIN `citebase`.`aw_link` t ON t.`link_id` = s.`link_id`
  UNION ALL SELECT 12, 'aw_contact', COUNT(s.`contact_id`), COUNT(t.`contact_id`), COUNT(s.`contact_id`) - COUNT(t.`contact_id`)
    FROM `mw_contact` s LEFT JOIN `citebase`.`aw_contact` t ON t.`contact_id` = s.`contact_id`
) r ORDER BY r.`o`;

SELECT (SELECT COUNT(*) FROM `citebase`.`aw_document`) AS `新库文档总数`,
       (SELECT COUNT(*) FROM `citebase`.`aw_document` WHERE `is_delete` = 0) AS `未删除`,
       (SELECT COUNT(*) FROM `citebase`.`aw_space`)   AS `空间数`,
       (SELECT COUNT(*) FROM `citebase`.`aw_user`)    AS `用户数`,
       (SELECT COUNT(*) FROM `citebase`.`aw_space_user`) AS `空间成员数`,
       (SELECT COUNT(*) FROM `citebase`.`aw_attachment`) AS `附件数`;

-- 目录/文档类型分布应与旧库一致（旧库 type: 1=页 2=目录）
SELECT `type` AS `类型`, COUNT(*) AS `数量` FROM `citebase`.`aw_document` GROUP BY `type`;

-- 权限体系抽查：超级管理员应能落到全部权限
SELECT r.`role_id` AS `角色ID`, r.`name` AS `角色`, COUNT(rp.`privilege_id`) AS `权限数`
  FROM `citebase`.`aw_role` r
  LEFT JOIN `citebase`.`aw_role_privilege` rp ON rp.`role_id` = r.`role_id`
 GROUP BY r.`role_id`, r.`name`;

-- 配置最终态（4.1 覆盖值 + 4.2 补插旧库独有项，肉眼确认站点标题等是否就位）
SELECT `key`, `value`, `label`, `group_key` FROM `citebase`.`aw_config` ORDER BY `group_key`, `sort`;

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- 6.（默认关闭）运行日志迁移
--    65952 行系统日志 + 2976 行文档日志 + 3595 行浏览记录：纯历史噪音，默认不迁。
--    确需留痕就去掉下面 4 段的双连字符注释。document_edit_lock 是旧实例的残留
--    编辑锁（5 行），新系统用 Yjs 协作，迁移后会误锁文档 —— 任何情况下都不要搬。
-- ============================================================
-- INSERT IGNORE INTO `citebase`.`aw_log`
--   (`log_id`, `level`, `url`, `method`, `param`, `path`, `get`, `post`, `message`, `ip`,
--    `user_agent`, `referer`, `user_id`, `username`, `create_time`)
-- SELECT s.`log_id`, s.`level`, s.`url`, s.`method`, s.`param`, s.`path`, s.`get`, s.`post`,
--        s.`message`, s.`ip`, s.`user_agent`, s.`referer`, s.`user_id`, s.`username`, COALESCE(s.`create_time`, 0)
--   FROM `mw_log` s;
--
-- INSERT IGNORE INTO `citebase`.`aw_log_document`
--   (`log_document_id`, `document_id`, `space_id`, `user_id`, `action`, `comment`, `content`, `create_time`)
-- SELECT s.`log_document_id`, s.`document_id`, s.`space_id`, s.`user_id`, s.`action`, s.`comment`,
--        s.`content`, COALESCE(s.`create_time`, 0)
--   FROM `mw_log_document` s;
--
-- INSERT IGNORE INTO `citebase`.`aw_document_view_log`
--   (`id`, `document_id`, `space_id`, `user_id`, `ip`, `user_agent`, `viewed_at`)
-- SELECT s.`id`, s.`document_id`, s.`space_id`, s.`user_id`, s.`ip`, s.`user_agent`, s.`viewed_at`
--   FROM `mw_document_view_log` s;

-- ============================================================
-- 7. 不迁移清单（有意为之，勿补）
-- ============================================================
--  · mw_privilege / mw_role / mw_role_privilege
--      旧库 93 项权限与新库 seedPrivileges() 完全同一套（仅 parent_id 归 0、图标由 glyphicon-*
--      换成 icon-*、84~93 号排序不同）。导入会让 count>0 守卫跳过新库种子，菜单停在旧图标上。
--      角色 1/2/3 语义一致，用户的 role_id 原值照搬即可。
--  · mw_ai_prompt_template
--      旧库 4 条即新库 seedPromptTemplates() 的种子（名称逐字相同），新库自己会补。
--  · mw_ai_* / mw_text2sql_* / mw_schema_* / mw_wiki_* / mw_vector_sync_task 等
--      旧库全部空表，无数据可迁。新库另有 5 张 aw_wiki_* 表，由知识编译功能生成。
--  · mw_api_key / mw_oidc_* / mw_login_auth / mw_plugin / mw_notification / mw_webhook_* /
--      mw_saved_search / mw_document_link / mw_document_tag / mw_document_template /
--      mw_document_mention / mw_document_approval* / mw_tag_hierarchy / mw_git_sync_* /
--      mw_email / mw_ai_*_cache 等空表：同上。
--
-- 迁移后必做（脚本无法代劳）：
--  1) 磁盘文件搬迁（旧服务器数据目录 → 新库 configs/config.yaml 的 storage 目录）：
--       <旧>/markdowns/**  →  document_dir   (默认 ./data/markdowns)   文档正文
--       <旧>/images/**     →  image_dir      (默认 ./data/images)      内嵌图片，aw_attachment.path 形如 images/1/30/x.png
--       <旧>/attachment/** →  attachment_dir (默认 ./data/attachment)  下载附件
--     文件名由「空间名/父级目录名.../文档名」推导，改了空间名或执行了第 2 节改名，就要同步改文件/目录名。
--  2) 重启 Citebase：Bleve 全文索引开机自动重建；确认 aw_search 有结果。
--  3) 知识图谱/AI 分析结果为空：在「AI 设置 → 知识分析」重跑一次全量分析。
--  4) 向量库（Bolt/Chroma/Qdrant）需重新同步：迁移过来的文档没有向量。
--  5) 用旧密码登录任一迁移账号验证；admin 若沿用新库随机密码，见 4.3 注释。
--  6) 核对第 5 节报告无异常后清理历史库：
--       DROP DATABASE `citebase_legacy`;   -- 或 run-migrate.ps1 -DropLegacy
