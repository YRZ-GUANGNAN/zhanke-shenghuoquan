-- 回滚 064：移除众创分类及其条目。
DELETE FROM navigation_items WHERE category_id IN (SELECT id FROM navigation_categories WHERE system_key = 'campus_maker');
DELETE FROM navigation_categories WHERE system_key = 'campus_maker';
