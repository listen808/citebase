-- ============================================================
-- Citebase 迁移核对报告：citebase_legacy(mw_*)  vs  citebase(aw_*)
-- ============================================================
-- 只读、可重复执行，不改任何数据。表清单、主键、列、索引、排序规则全部
-- 现从 information_schema 取；改库名只需改下面的 @src/@dst —— 唯一例外是
-- 第 6 节（自增水位）的库名写死在 SQL 里，换库名要一并改。
--
-- 用法（任选其一）:
--   mysql --defaults-extra-file=my.cnf --table < scripts/migrate-legacy/verify-gap.sql
--   mysql -h HOST -P PORT -u root -p --table citebase < scripts/migrate-legacy/verify-gap.sql
--   run-migrate.ps1 跑完后手工执行一次，对照 migrate-from-v1.sql 第 5 节
--
-- 兼容 MySQL 8.0 与 MariaDB 10.5+（不使用 CTE / 窗口函数）。
-- 本文件必须存成 UTF-8 无 BOM + LF，否则 mysql 客户端会在第一行报 1064。
-- ============================================================

SET NAMES utf8mb4;
SET SESSION group_concat_max_len = 1000000;

SET @src = 'citebase_legacy';
SET @dst = 'citebase';

SELECT '== 0. 前提 ==' AS `报告`;

SELECT (SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = @src) AS `旧库存在`,
       (SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = @dst) AS `新库存在`,
       (SELECT COUNT(*) FROM information_schema.tables
         WHERE table_schema = @src AND LEFT(table_name, 3) = 'mw_') AS `旧库_mw_表`,
       (SELECT COUNT(*) FROM information_schema.tables
         WHERE table_schema = @dst AND LEFT(table_name, 3) = 'aw_') AS `新库_aw_表`,
       IF((SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = @src) = 0,
          '!! 旧库不存在：第 2 节行级核对会自动跳过，其余节仍可比结构', 'OK') AS `检查`;

-- ============================================================
-- 1. 表清单差异
-- ============================================================
SELECT '== 1. 表清单 ==' AS `报告`;

SELECT '旧库有、新库无' AS `方向`, t.table_name AS `表`
  FROM information_schema.tables t
 WHERE t.table_schema = @src AND LEFT(t.table_name, 3) = 'mw_'
   AND NOT EXISTS (SELECT 1 FROM information_schema.tables x
                    WHERE x.table_schema = @dst AND x.table_name = CONCAT('aw_', SUBSTRING(t.table_name, 4)))
UNION ALL
SELECT '新库有、旧库无', t.table_name
  FROM information_schema.tables t
 WHERE t.table_schema = @dst AND LEFT(t.table_name, 3) = 'aw_'
   AND NOT EXISTS (SELECT 1 FROM information_schema.tables x
                    WHERE x.table_schema = @src AND x.table_name = CONCAT('mw_', SUBSTRING(t.table_name, 4)));

-- ============================================================
-- 2. 行级核对（动态生成，覆盖两边同名的全部表；只列有行的一侧非空者）
--    src      旧库行数
--    dst      新库行数
--    matched  旧库里能按主键在新库找到同一行的条数  → 差额即"没进来"的行
--    missing  新库里按主键找不到的旧库行（=dst-src+matched 的直观化）
-- ============================================================
SELECT '== 2. 行级核对 ==' AS `报告`;

-- 主键一律以【新库】为准（新库可能删掉/改掉了旧库的自增 id，例如 mw_follow.id）；
-- 只有两边都存在同名列时才做行级比对，否则 matched/未迁入 记 NULL，不参与判定。
SET @sql = (
  SELECT GROUP_CONCAT(CONCAT(
           'SELECT ', QUOTE(t.table_name), ' AS `旧表`, ',
           QUOTE(CONCAT('aw_', SUBSTRING(t.table_name, 4))), ' AS `新表`, ',
           '(SELECT COUNT(*) FROM `', @src, '`.`', t.table_name, '`) AS `src`, ',
           '(SELECT COUNT(*) FROM `', @dst, '`.`aw_', SUBSTRING(t.table_name, 4), '`) AS `dst`, ',
           IF(k.pk IS NULL, 'NULL AS `matched`, NULL AS `未迁入`',
              CONCAT('(SELECT COUNT(*) FROM `', @src, '`.`', t.table_name, '` a JOIN `', @dst,
                     '`.`aw_', SUBSTRING(t.table_name, 4), '` b ON b.`', k.pk, '` = a.`', k.pk, '`) AS `matched`, ',
                     '(SELECT COUNT(*) FROM `', @src, '`.`', t.table_name, '` a WHERE NOT EXISTS ',
                     '(SELECT 1 FROM `', @dst, '`.`aw_', SUBSTRING(t.table_name, 4), '` b ',
                     'WHERE b.`', k.pk, '` = a.`', k.pk, '`)) AS `未迁入`')),
           ' FROM DUAL WHERE (SELECT COUNT(*) FROM `', @src, '`.`', t.table_name, '`) > 0 ',
           'OR (SELECT COUNT(*) FROM `', @dst, '`.`aw_', SUBSTRING(t.table_name, 4), '`) > 0'
         )
         ORDER BY t.table_name SEPARATOR ' UNION ALL ')
    FROM information_schema.tables t
    JOIN information_schema.tables t2
      ON t2.table_schema = @dst AND t2.table_name = CONCAT('aw_', SUBSTRING(t.table_name, 4))
    LEFT JOIN (SELECT kc.table_name AS dt, MIN(kc.column_name) AS pk
                 FROM information_schema.key_column_usage kc
                 JOIN information_schema.columns sc
                   ON sc.table_schema = kc.table_schema
                  AND sc.table_name = kc.table_name
                  AND sc.column_name = kc.column_name
                 JOIN information_schema.columns lc
                   ON lc.table_schema = @src
                  AND lc.table_name = CONCAT('mw_', SUBSTRING(kc.table_name, 4))
                  AND lc.column_name = kc.column_name
                WHERE kc.constraint_name = 'PRIMARY' AND kc.table_schema = @dst
                  AND LEFT(kc.table_name, 3) = 'aw_'
                GROUP BY kc.table_name
               HAVING COUNT(*) = 1) k
           ON k.dt = t2.table_name
   WHERE t.table_schema = @src AND LEFT(t.table_name, 3) = 'mw_');

SET @sql = IFNULL(@sql, 'SELECT ''旧库不存在或无同名表，跳过行级核对'' AS `提示`');
PREPARE gapstmt FROM @sql;
EXECUTE gapstmt;
DEALLOCATE PREPARE gapstmt;

-- ============================================================
-- 3. 结构差异（列）
-- ============================================================
SELECT '== 3. 列差异 ==' AS `报告`;

SELECT '旧有新无' AS `方向`, c.table_name AS `表`, c.column_name AS `列`, c.column_type AS `定义`
  FROM information_schema.columns c
 WHERE c.table_schema = @src AND LEFT(c.table_name, 3) = 'mw_'
   -- 只比两边都存在的表；新库独有表（aw_wiki_*）的列不算列差异
   AND EXISTS (SELECT 1 FROM information_schema.tables pt
                WHERE pt.table_schema = @dst
                  AND pt.table_name = CONCAT('aw_', SUBSTRING(c.table_name, 4)))
   AND NOT EXISTS (SELECT 1 FROM information_schema.columns x
                    WHERE x.table_schema = @dst
                      AND x.table_name = CONCAT('aw_', SUBSTRING(c.table_name, 4))
                      AND x.column_name = c.column_name)
UNION ALL
SELECT '新有旧无', c.table_name, c.column_name, c.column_type
  FROM information_schema.columns c
 WHERE c.table_schema = @dst AND LEFT(c.table_name, 3) = 'aw_'
   AND EXISTS (SELECT 1 FROM information_schema.tables pt
                WHERE pt.table_schema = @src
                  AND pt.table_name = CONCAT('mw_', SUBSTRING(c.table_name, 4)))
   AND NOT EXISTS (SELECT 1 FROM information_schema.columns x
                    WHERE x.table_schema = @src
                      AND x.table_name = CONCAT('mw_', SUBSTRING(c.table_name, 4))
                      AND x.column_name = c.column_name)
 ORDER BY 1, 2, 3;

-- 类型差异分类：只看真实的家族变更与时间精度损失，忽略 tinyint(1)/int(11) 这类显示宽度噪音
SELECT `类别`, `新表`, `列`, `旧定义`, `新定义` FROM (
  SELECT CASE
           WHEN SUBSTRING_INDEX(REPLACE(c.column_type, ' unsigned', ''), '(', 1)
              <> SUBSTRING_INDEX(REPLACE(x.column_type, ' unsigned', ''), '(', 1)
                THEN '1 类型家族变更'
           WHEN SUBSTRING_INDEX(c.column_type, '(', 1) IN ('datetime', 'timestamp', 'time')
             AND c.column_type <> x.column_type
                THEN '2 时间精度变化'
           WHEN c.is_nullable <> x.is_nullable THEN '3 可空性变化'
           ELSE '9 显示宽度差异(忽略)'
         END AS `类别`,
         x.table_name AS `新表`, c.column_name AS `列`,
         c.column_type AS `旧定义`, x.column_type AS `新定义`
    FROM information_schema.columns c
    JOIN information_schema.columns x
      ON x.table_schema = @dst
     AND x.table_name = CONCAT('aw_', SUBSTRING(c.table_name, 4))
     AND x.column_name = c.column_name
   WHERE c.table_schema = @src AND LEFT(c.table_name, 3) = 'mw_') y
 WHERE `类别` <> '9 显示宽度差异(忽略)'
 ORDER BY `类别`, `新表`, `列`;

SELECT CONCAT('被忽略的显示宽度差异（tinyint(1)->tinyint(4) 一类，无语义影响）: ',
       COUNT(*)) AS `噪音统计`
  FROM information_schema.columns c
  JOIN information_schema.columns x
    ON x.table_schema = @dst
   AND x.table_name = CONCAT('aw_', SUBSTRING(c.table_name, 4))
   AND x.column_name = c.column_name
 WHERE c.table_schema = @src AND LEFT(c.table_name, 3) = 'mw_'
   AND c.column_type <> x.column_type
   AND SUBSTRING_INDEX(REPLACE(c.column_type, ' unsigned', ''), '(', 1)
     = SUBSTRING_INDEX(REPLACE(x.column_type, ' unsigned', ''), '(', 1)
   AND SUBSTRING_INDEX(c.column_type, '(', 1) NOT IN ('datetime', 'timestamp', 'time');

-- ============================================================
-- 4. 约束差异：新库更严的唯一键，是"行数对不上"最常见的正当原因
-- ============================================================
SELECT '== 4. 唯一约束 ==' AS `报告`;

SELECT CONCAT('aw_', SUBSTRING(l.tbl, 4), '  UNIQUE(', l.cols, ')') AS `新库独有唯一键`,
       IFNULL(a.`说明`, '（旧库无此约束，迁移时可能产生冲突行）') AS `已知影响`
  FROM (SELECT t.table_name tbl, i.index_name,
               GROUP_CONCAT(i.column_name ORDER BY i.seq_in_index) cols
          FROM information_schema.tables t
          JOIN information_schema.statistics i
            ON i.table_schema = @dst
           AND i.table_name = CONCAT('aw_', SUBSTRING(t.table_name, 4))
           AND i.non_unique = 0 AND i.index_name <> 'PRIMARY'
         WHERE t.table_schema = @src AND LEFT(t.table_name, 3) = 'mw_'
         GROUP BY t.table_name, i.index_name) l
  LEFT JOIN (SELECT table_name tbl, index_name, GROUP_CONCAT(column_name ORDER BY seq_in_index) cols
               FROM information_schema.statistics
              WHERE table_schema = @src AND non_unique = 0 AND index_name <> 'PRIMARY'
              GROUP BY table_name, index_name) o
         ON o.tbl = l.tbl AND o.cols = l.cols
  LEFT JOIN (SELECT 'aw_document|space_id,parent_id,name' k,
                    '旧库允许同名兄弟文档，新库不允许；migrate-from-v1.sql 第 2 节自动改名为「标题 (文档ID)」后应为 0 冲突' `说明`
               UNION ALL
               SELECT 'aw_ai_table_cache|table_name,space_id',
                      'v2 的多空间修正：v1 的 uk_table_name 是全局唯一，跨空间同名表会互相覆盖'
              ) a
         ON a.k = CONCAT('aw_', SUBSTRING(l.tbl, 4), '|', l.cols)
 WHERE o.cols IS NULL
 ORDER BY 1;

-- ============================================================
-- 5. 排序规则与字符集（跨库比较必须显式 COLLATE 的根据）
-- ============================================================
SELECT '== 5. 排序规则 ==' AS `报告`;

SELECT @src AS `库`, table_collation AS `排序规则`, COUNT(*) AS `表数`
  FROM information_schema.tables
 WHERE table_schema = @src AND LEFT(table_name, 3) = 'mw_'
 GROUP BY table_collation
UNION ALL
SELECT @dst, table_collation, COUNT(*)
  FROM information_schema.tables
 WHERE table_schema = @dst AND LEFT(table_name, 3) = 'aw_'
 GROUP BY table_collation;

-- ============================================================
-- 6. 自增水位：判断"以后还能不能补"（旧库未迁的表，新库若已自己用了同号段就会撞）
-- ============================================================
SELECT '== 6. 自增水位 / 补迁可行性 ==' AS `报告`;

-- 逐表对照自增水位与"未迁入 / 已被新行占用同号"两个数，用来判断事后补迁的可行性。
-- 想覆盖更多表，照抄一段改掉表名和主键列名即可。
SELECT `旧表`, `旧库最大ID`, `新库最大ID`, `未迁入行`, `同主键已占行`,
       CASE WHEN `未迁入行` = 0 THEN '已完整迁入'
            WHEN `同主键已占行` = 0 THEN '可直接补迁（INSERT IGNORE 即可）'
            ELSE CONCAT('还有 ', `未迁入行`, ' 行未进新库，其中 ', `同主键已占行`,
                   ' 行的主键在新库已被占用（可能是已迁入的同主键行，也可能是应用新写入的行）；补迁前先对照第 7 节判断该不该补')
       END AS `结论`
  FROM (
        SELECT 'mw_log' `旧表`,
               (SELECT IFNULL(MAX(log_id), 0) FROM `citebase_legacy`.`mw_log`) `旧库最大ID`,
               (SELECT IFNULL(MAX(log_id), 0) FROM `citebase`.`aw_log`) `新库最大ID`,
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_log` a WHERE NOT EXISTS
                  (SELECT 1 FROM `citebase`.`aw_log` b WHERE b.`log_id` = a.`log_id`)) `未迁入行`,
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_log` a JOIN `citebase`.`aw_log` b
                  ON b.`log_id` = a.`log_id`) `同主键已占行`
        UNION ALL
        SELECT 'mw_log_document',
               (SELECT IFNULL(MAX(log_document_id), 0) FROM `citebase_legacy`.`mw_log_document`),
               (SELECT IFNULL(MAX(log_document_id), 0) FROM `citebase`.`aw_log_document`),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_log_document` a WHERE NOT EXISTS
                  (SELECT 1 FROM `citebase`.`aw_log_document` b WHERE b.`log_document_id` = a.`log_document_id`)),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_log_document` a JOIN `citebase`.`aw_log_document` b
                  ON b.`log_document_id` = a.`log_document_id`)
        UNION ALL
        SELECT 'mw_document_view_log',
               (SELECT IFNULL(MAX(id), 0) FROM `citebase_legacy`.`mw_document_view_log`),
               (SELECT IFNULL(MAX(id), 0) FROM `citebase`.`aw_document_view_log`),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_document_view_log` a WHERE NOT EXISTS
                  (SELECT 1 FROM `citebase`.`aw_document_view_log` b WHERE b.`id` = a.`id`)),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_document_view_log` a JOIN `citebase`.`aw_document_view_log` b
                  ON b.`id` = a.`id`)
        UNION ALL
        SELECT 'mw_document_edit_lock',
               (SELECT IFNULL(MAX(document_id), 0) FROM `citebase_legacy`.`mw_document_edit_lock`),
               (SELECT IFNULL(MAX(document_id), 0) FROM `citebase`.`aw_document_edit_lock`),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_document_edit_lock` a WHERE NOT EXISTS
                  (SELECT 1 FROM `citebase`.`aw_document_edit_lock` b WHERE b.`document_id` = a.`document_id`)),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_document_edit_lock` a JOIN `citebase`.`aw_document_edit_lock` b
                  ON b.`document_id` = a.`document_id`)
        UNION ALL
        SELECT 'mw_document',
               (SELECT IFNULL(MAX(document_id), 0) FROM `citebase_legacy`.`mw_document`),
               (SELECT IFNULL(MAX(document_id), 0) FROM `citebase`.`aw_document`),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_document` a WHERE NOT EXISTS
                  (SELECT 1 FROM `citebase`.`aw_document` b WHERE b.`document_id` = a.`document_id`)),
               (SELECT COUNT(*) FROM `citebase_legacy`.`mw_document` a JOIN `citebase`.`aw_document` b
                  ON b.`document_id` = a.`document_id`)
      ) x;

-- ============================================================
-- 7. 已知有意差异（自带注，避免下一轮又被当成"漏迁"）
-- ============================================================
SELECT '== 7. 有意差异清单 ==' AS `报告`;

SELECT `对象`, `差异`, `原因与处置` FROM (
  SELECT 1 n, 'aw_document' `对象`, '比旧库少 2 行' `差异`,
         '空间 2 已在旧库删除，其下 159/160 两篇文档无归属；migrate-from-v1.sql 按空间存在性过滤，第 1 节会列明细' `原因与处置`
  UNION ALL SELECT 2, 'aw_document.name', '16 行带「 (文档ID)」后缀',
         '撞 uk_space_parent_name 的同级重名被改名消解；改名只发生在旧库副本，因此两库这 16 行标题必然不一致，且全部是 is_delete=1 的回收站文档'
  UNION ALL SELECT 3, 'aw_log / aw_log_document / aw_document_view_log', '旧库行数远多于新库',
         '运行日志默认不迁（migrate-from-v1.sql 第 6 节，注释块）。注意新库这几张表已被应用自身写入，事后补迁会撞主键'
  UNION ALL SELECT 4, 'aw_document_edit_lock', '恒为 0',
         'v1 的编辑锁是残留状态，v2 用 Yjs 协作，迁入会误锁文档 —— 设计上永不迁移'
  UNION ALL SELECT 5, 'aw_privilege / aw_role / aw_role_privilege / aw_ai_prompt_template', '行数与旧库恰好相同',
         '不是迁来的，是新库 seedPrivileges()/seedPromptTemplates() 自种；v1 与 v2 的种子本来就是同一套，所以数字巧合相等'
  UNION ALL SELECT 6, 'mw_config.name', '新库无此列',
         'v2 用 label 承担界面文案，旧库 name 列被丢弃；migrate 时只回填 value，label/分组/控件类型以新库为准'
  UNION ALL SELECT 7, 'mw_follow.id', '新库无此列',
         'v2 主键就是 follow_id，旧库那个自增 id 已删'
  UNION ALL SELECT 8, 'mw_document_version_tag.COLUMN', '新库列名为 color',
         'v1→v2 的列改名；两库该表都为空，无数据损失'
  UNION ALL SELECT 9, '全部 varchar/text 的 DEFAULT', '新库多数无默认值',
         'genddl 旧版会合成 DEFAULT ''''/DEFAULT 0，而模型未声明；AutoMigrate 首次启动已把差异收敛（现象：18 条 MariaDB "Can''t DROP FOREIGN KEY" 噪音）'
  UNION ALL SELECT 10, '磁盘 markdowns / images / attachment', 'SQL 之外',
         '文档正文与附件文件不在库里；aw_document 无 content 列，正文按 <空间名>/<父级目录名…>/<文档名>.md 寻址，缺文件即空页'
  UNION ALL SELECT 11, 'aw_follow 第 2 节显示「未迁入 4」', '实际不缺数据',
         '旧库这 4 行 follow_id=0（v1 写入时漏给主键赋值，全表 351 行只有 348 个不同 follow_id）；新库 follow_id 是主键+自增，插 0 会被重新编号（本机为 367~370）。两侧都是 351 行，按 (user_id,type,object_id) 一一对应'
) r ORDER BY r.n;

-- ============================================================
-- 8. 一句话结论
-- ============================================================
SELECT '== 8. 汇总 ==' AS `报告`;

SELECT CONCAT('旧库非空表 ', (SELECT COUNT(*) FROM information_schema.tables t
                              WHERE t.table_schema = @src AND LEFT(t.table_name, 3) = 'mw_'
                                AND t.table_rows > 0), ' 张（information_schema 估算，仅供快速看规模）') AS `提示`,
       '精确行数见第 2 节' AS `备注`;
