-- 众创：首页「众创」频道的外接小程序入口分类。
-- 分类以 systemKey 稳定识别（与 external_takeout_merchants 同模式），
-- 条目在后台「服务入口」中维护；前端按 systemKey 与其他工具分类分流。
INSERT INTO navigation_categories (name, system_key, sort, enabled, created_at, updated_at)
SELECT '众创', 'campus_maker', 30, TRUE, NOW(3), NOW(3)
WHERE NOT EXISTS (SELECT 1 FROM navigation_categories WHERE system_key = 'campus_maker');
