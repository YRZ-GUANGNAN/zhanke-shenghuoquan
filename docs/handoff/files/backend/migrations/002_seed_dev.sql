INSERT INTO users (id, open_id, nickname, school, campus, role, status, created_at, updated_at) VALUES
	(1, 'mock_openid_admin', '热心学长', '海大', '海大', 'user', 'active', NOW(), NOW()),
  (2, 'dev-openid-life', '校园生活组', '海景校区', '海景', 'user', 'active', NOW(), NOW()),
  (3, 'dev-openid-market', '校友集市', '海大', '海大', 'user', 'active', NOW(), NOW())
ON DUPLICATE KEY UPDATE nickname=VALUES(nickname), school=VALUES(school), campus=VALUES(campus), role=VALUES(role), status=VALUES(status), updated_at=NOW();

INSERT INTO posts (id,user_id,topic,title,content,review_status,pinned,liked_count,comment_count,created_at,updated_at) VALUES
  (1,1,'二手避坑','二手书不要论斤卖','教材和备考资料请标清版本与新旧程度。','approved',TRUE,128,36,NOW()-INTERVAL 10 MINUTE,NOW()),
  (2,2,'校园问答','新生常用导航整理','常用小程序入口集中在导航页，仅提供跳转。','approved',FALSE,86,14,NOW()-INTERVAL 32 MINUTE,NOW())
ON DUPLICATE KEY UPDATE topic=VALUES(topic),title=VALUES(title),content=VALUES(content),review_status=VALUES(review_status),updated_at=NOW();

INSERT INTO products (id,seller_id,title,description,category,price,original_price,condition_label,campus,status,review_status,created_at,updated_at) VALUES
  (1,1,'二手教材一组','校内自提。','书籍',1000,4280,'八成新','海大','selling','approved',NOW(),NOW()),
  (2,3,'桌面收纳盒','轻微使用痕迹。','宿舍必备',800,0,'轻微使用痕迹','东区','reserved','approved',NOW()-INTERVAL 40 MINUTE,NOW())
ON DUPLICATE KEY UPDATE title=VALUES(title),description=VALUES(description),category=VALUES(category),price=VALUES(price),status=VALUES(status),review_status=VALUES(review_status),updated_at=NOW();

INSERT INTO navigation_categories (id,name,sort,enabled,created_at,updated_at) VALUES (1,'校园服务',10,TRUE,NOW(),NOW())
ON DUPLICATE KEY UPDATE name=VALUES(name),sort=VALUES(sort),enabled=VALUES(enabled),updated_at=NOW();
INSERT INTO navigation_items (id,category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled,created_at,updated_at) VALUES
  (1,1,'校园地图','楼宇导航','图','wx-map-demo','/pages/map/index','#4fd1c5',10,TRUE,NOW(),NOW()),
  (2,1,'学生兼职','校内外岗位','兼','wx-job-demo','/pages/home/index','#ef6a6a',20,TRUE,NOW(),NOW())
ON DUPLICATE KEY UPDATE category_id=VALUES(category_id),title=VALUES(title),subtitle=VALUES(subtitle),app_id=VALUES(app_id),path=VALUES(path),enabled=VALUES(enabled),updated_at=NOW();

INSERT INTO review_tasks (object_type,object_id,submitter_id,risk,status,created_at,updated_at) VALUES
  ('post',1,1,'seed approved','approved',NOW(),NOW()),('post',2,2,'seed approved','approved',NOW(),NOW()),
  ('product',1,1,'seed approved','approved',NOW(),NOW()),('product',2,3,'seed approved','approved',NOW(),NOW())
ON DUPLICATE KEY UPDATE status=VALUES(status),risk=VALUES(risk),updated_at=NOW();
