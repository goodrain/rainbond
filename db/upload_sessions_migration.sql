-- 分片上传会话表
-- 用于支持大文件的分片上传和断点续传功能
-- 注意：GORM 会自动创建此表，此文件仅供参考或手动执行

CREATE TABLE IF NOT EXISTS `upload_sessions` (
  `id` VARCHAR(64) NOT NULL COMMENT '上传会话ID（UUID）',
  `event_id` VARCHAR(64) NOT NULL COMMENT '关联的事件ID',
  `file_name` VARCHAR(255) NOT NULL COMMENT '原始文件名',
  `file_size` BIGINT NOT NULL COMMENT '文件总大小（字节）',
  `file_md5` VARCHAR(32) DEFAULT NULL COMMENT '文件MD5校验值',
  `chunk_size` INT NOT NULL COMMENT '分片大小（字节）',
  `total_chunks` INT NOT NULL COMMENT '总分片数',
  `uploaded_chunks` TEXT DEFAULT NULL COMMENT '已上传的分片索引（逗号分隔）',
  `status` VARCHAR(20) NOT NULL COMMENT '状态：uploading, completed, failed, expired',
  `storage_path` VARCHAR(512) DEFAULT NULL COMMENT '最终文件存储路径',
  `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `expires_at` TIMESTAMP NULL DEFAULT NULL COMMENT '过期时间',
  PRIMARY KEY (`id`),
  INDEX `idx_event_id` (`event_id`),
  INDEX `idx_status` (`status`),
  INDEX `idx_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文件分片上传会话表';

-- 查询特定事件的上传会话
-- SELECT * FROM upload_sessions WHERE event_id = 'xxx' ORDER BY created_at DESC;

-- 查询进行中的上传
-- SELECT * FROM upload_sessions WHERE status = 'uploading' ORDER BY created_at DESC;

-- 清理过期的上传会话（由定时任务执行）
-- DELETE FROM upload_sessions WHERE expires_at < NOW() AND status != 'completed';

-- 查询某个会话的详细信息
-- SELECT
--   id,
--   event_id,
--   file_name,
--   file_size,
--   total_chunks,
--   LENGTH(uploaded_chunks) - LENGTH(REPLACE(uploaded_chunks, ',', '')) + 1 as uploaded_count,
--   ROUND((LENGTH(uploaded_chunks) - LENGTH(REPLACE(uploaded_chunks, ',', '')) + 1) * 100.0 / total_chunks, 2) as progress,
--   status,
--   created_at,
--   updated_at
-- FROM upload_sessions
-- WHERE id = 'xxx';
