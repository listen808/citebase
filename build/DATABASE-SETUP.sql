-- ============================================================
--  Citebase - 数据库准备（只需三步）
-- ============================================================

--  前提：本机已安装并启动 MySQL 8.0（utf8mb4）。

--  第 1 步：创建空库

--      CREATE DATABASE citebase DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;

--      不需要导入 databases\table.sql 或 data.sql。首次启动会自动执行
--      AutoMigrate + seedDefaults，建好全部 94 张表、93 项权限、角色、
--      系统配置，并创建 admin 账号。

--      注意：table.sql 里每条 CREATE TABLE 前都带 DROP TABLE，只能对
--      空库执行；对着已有数据的库跑等于整库清空。

--  第 2 步：改 DSN

--      编辑 configs\config.yaml 的 database.dsn，例如：
--        dsn: "root:你的密码@tcp(127.0.0.1:3306)/citebase?charset=utf8mb4&parseTime=True"

--      口令建议用环境变量 CITEBASE_DB_DSN 注入（旧名 AIWIKI_DB_DSN 同样接受），别明文写进文件。

--  第 3 步：启动

--      双击 start.bat（安装版用桌面快捷方式），浏览器打开
--      http://localhost:3001。

--      admin 的随机初始密码只在首次启动时打印一次，见 data\logs 下的日志。

--  端口

--      端口在 config.yaml 的 server.port（默认 3001），该键不支持
--      环境变量覆盖，只能改这个文件。
