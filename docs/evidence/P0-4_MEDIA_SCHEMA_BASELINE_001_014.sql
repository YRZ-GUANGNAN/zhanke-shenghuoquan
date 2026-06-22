
/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;
DROP TABLE IF EXISTS `comments`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `comments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `content` text COLLATE utf8mb4_unicode_ci,
  `review_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'submitted',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_comments_post_id` (`post_id`),
  KEY `idx_comments_user_id` (`user_id`),
  KEY `idx_comments_review_status` (`review_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `im_conversations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `im_conversations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `conversation_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `buyer_id` bigint unsigned NOT NULL,
  `seller_id` bigint unsigned NOT NULL,
  `reference_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `reference_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_im_relation` (`buyer_id`,`seller_id`,`reference_type`,`reference_id`),
  KEY `idx_conversations_conversation_id` (`conversation_id`),
  KEY `idx_conversations_buyer_updated` (`buyer_id`,`updated_at`,`id`),
  KEY `idx_conversations_seller_updated` (`seller_id`,`updated_at`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `im_user_mappings`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `im_user_mappings` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `im_user_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `user_id` (`user_id`),
  UNIQUE KEY `im_user_id` (`im_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_asset_audits`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_asset_audits` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `asset_id` bigint unsigned NOT NULL,
  `from_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `to_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `action` varchar(48) COLLATE utf8mb4_unicode_ci NOT NULL,
  `actor_type` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `actor_id` bigint unsigned NOT NULL DEFAULT '0',
  `detail` varchar(240) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_media_audit_asset` (`asset_id`,`created_at`),
  KEY `idx_media_audit_action` (`action`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_asset_references`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_asset_references` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `asset_id` bigint unsigned NOT NULL,
  `reference_type` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `reference_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `slot` smallint unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `parent_visibility` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'hidden',
  `aggregate_version` bigint unsigned NOT NULL DEFAULT '0',
  `operation_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `revocation_fence` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `fence_expires_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_media_reference_slot` (`reference_type`,`reference_id`,`slot`),
  UNIQUE KEY `uq_media_reference_asset_slot` (`asset_id`,`reference_type`,`reference_id`,`slot`),
  KEY `idx_media_reference_asset_status` (`asset_id`,`status`),
  KEY `idx_media_reference_parent_status` (`reference_type`,`reference_id`,`status`),
  KEY `idx_media_reference_operation` (`operation_id`),
  CONSTRAINT `fk_media_reference_asset` FOREIGN KEY (`asset_id`) REFERENCES `media_assets` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_parent_visibility` CHECK ((`parent_visibility` in (_utf8mb4'hidden',_utf8mb4'visible'))),
  CONSTRAINT `chk_media_reference_status` CHECK ((`status` in (_utf8mb4'pending',_utf8mb4'active',_utf8mb4'revoking',_utf8mb4'revoked'))),
  CONSTRAINT `chk_media_reference_type` CHECK ((`reference_type` in (_utf8mb4'post',_utf8mb4'product',_utf8mb4'avatar')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_assets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_assets` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `asset_public_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `owner_id` bigint unsigned NOT NULL,
  `public_id` char(32) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `owner_user_id` bigint unsigned DEFAULT NULL,
  `purpose` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `storage_stage` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'staging',
  `review_status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `technical_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending_upload',
  `trusted_sha256` char(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `object_key` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `declared_mime` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `verified_mime` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `byte_size` bigint unsigned NOT NULL DEFAULT '0',
  `declared_size` bigint NOT NULL DEFAULT '0',
  `verified_size` bigint NOT NULL DEFAULT '0',
  `width` bigint NOT NULL DEFAULT '0',
  `height` bigint NOT NULL DEFAULT '0',
  `staging_key` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `staging_version_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `final_key` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `final_version_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending_upload',
  `review_task_id` bigint unsigned DEFAULT NULL,
  `applied_decision_version` bigint unsigned NOT NULL DEFAULT '0',
  `projected_decision` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `projected_decided_at` datetime(3) DEFAULT NULL,
  `active_ref_count` bigint unsigned NOT NULL DEFAULT '0',
  `wechat_open_id` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `review_trace_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `review_suggest` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `review_label` bigint NOT NULL DEFAULT '0',
  `reference_type` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `reference_id` bigint unsigned NOT NULL DEFAULT '0',
  `reference_sort` bigint NOT NULL DEFAULT '0',
  `reservation_token` char(32) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `reservation_expires_at` datetime(3) DEFAULT NULL,
  `idempotency_key` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL,
  `upload_expires_at` datetime(3) DEFAULT NULL,
  `review_deadline_at` datetime(3) DEFAULT NULL,
  `review_attempts` bigint NOT NULL DEFAULT '0',
  `review_available_at` datetime(3) DEFAULT NULL,
  `last_error` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `delete_requested_at` datetime(3) DEFAULT NULL,
  `storage_deleted_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_media_asset_public_id_v11` (`asset_public_id`),
  UNIQUE KEY `uq_media_owner_idempotency_v11` (`owner_id`,`idempotency_key`),
  UNIQUE KEY `idx_media_public_id` (`public_id`),
  UNIQUE KEY `idx_media_object_key` (`object_key`),
  UNIQUE KEY `idx_media_owner_idempotency` (`owner_user_id`,`idempotency_key`),
  UNIQUE KEY `idx_media_review_trace` (`review_trace_id`),
  UNIQUE KEY `uq_media_staging_key_v11` (`staging_key`),
  UNIQUE KEY `uq_media_final_key_v11` (`final_key`),
  UNIQUE KEY `uq_media_review_task_v11` (`review_task_id`),
  KEY `idx_media_owner_created` (`owner_user_id`,`created_at`),
  KEY `idx_media_review_pending` (`status`,`review_available_at`,`id`),
  KEY `idx_media_delete_pending` (`delete_requested_at`,`storage_deleted_at`,`id`),
  KEY `idx_media_reference` (`reference_type`,`reference_id`,`reference_sort`),
  KEY `idx_media_review_task` (`review_task_id`),
  KEY `idx_media_upload_expires` (`upload_expires_at`),
  KEY `idx_media_reservation` (`reservation_token`),
  KEY `idx_media_technical_upload_v11` (`technical_status`,`upload_expires_at`,`id`),
  KEY `idx_media_review_deadline_v11` (`technical_status`,`review_deadline_at`,`id`),
  KEY `idx_media_delete_status_v11` (`technical_status`,`delete_requested_at`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_consumer_checkpoints`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_consumer_checkpoints` (
  `aggregate_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `last_event_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_aggregate_version` bigint unsigned NOT NULL,
  `last_operation` varchar(48) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `last_payload_hash` char(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `active_event_id` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_token` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_expires_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`aggregate_id`),
  KEY `idx_media_consumer_checkpoint_event` (`last_event_id`),
  KEY `idx_media_consumer_checkpoint_lease` (`lease_expires_at`,`active_event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_event_inbox`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_event_inbox` (
  `event_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `aggregate_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `aggregate_version` bigint unsigned NOT NULL,
  `operation` varchar(48) COLLATE utf8mb4_unicode_ci NOT NULL,
  `payload_hash` char(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `lease_token` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_expires_at` datetime(3) DEFAULT NULL,
  `attempt_count` int unsigned NOT NULL DEFAULT '0',
  `next_attempt_at` datetime(3) DEFAULT NULL,
  `last_error_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `processed_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`event_id`),
  KEY `idx_media_event_inbox_claim` (`status`,`next_attempt_at`,`lease_expires_at`,`event_id`),
  KEY `idx_media_event_inbox_aggregate` (`aggregate_id`,`aggregate_version`,`event_id`),
  CONSTRAINT `fk_media_event_inbox_outbox` FOREIGN KEY (`event_id`) REFERENCES `media_outbox` (`event_id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_event_inbox_status` CHECK ((`status` in (_utf8mb4'pending',_utf8mb4'processing',_utf8mb4'retry_wait',_utf8mb4'succeeded',_utf8mb4'deadletter')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_gc_auxiliary_jobs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_gc_auxiliary_jobs` (
  `job_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `lease_token` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_expires_at` datetime(3) DEFAULT NULL,
  `attempt_count` int unsigned NOT NULL DEFAULT '0',
  `last_error_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `purged_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`job_id`),
  UNIQUE KEY `uq_media_gc_aux_asset` (`asset_id`),
  KEY `idx_media_gc_aux_claim` (`status`,`lease_expires_at`,`job_id`),
  CONSTRAINT `fk_media_gc_aux_job_asset` FOREIGN KEY (`asset_id`) REFERENCES `media_assets` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_gc_aux_job_status` CHECK ((`status` in (_utf8mb4'pending',_utf8mb4'processing',_utf8mb4'succeeded',_utf8mb4'deadletter')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_gc_jobs`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_gc_jobs` (
  `job_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `object_stage` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `object_key` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL,
  `object_version_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `lease_token` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_expires_at` datetime(3) DEFAULT NULL,
  `attempt_count` int unsigned NOT NULL DEFAULT '0',
  `next_attempt_at` datetime(3) DEFAULT NULL,
  `last_error_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `provider_request_id` varchar(128) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `deleted_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`job_id`),
  UNIQUE KEY `uq_media_gc_object_identity` (`asset_id`,`object_stage`,`object_key`,`object_version_id`),
  KEY `idx_media_gc_claim` (`status`,`next_attempt_at`,`lease_expires_at`,`job_id`),
  KEY `idx_media_gc_asset_status` (`asset_id`,`status`,`object_stage`),
  CONSTRAINT `fk_media_gc_job_asset` FOREIGN KEY (`asset_id`) REFERENCES `media_assets` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_gc_fixed_identity` CHECK (((char_length(`object_key`) > 0) and (char_length(`object_version_id`) > 0))),
  CONSTRAINT `chk_media_gc_job_stage` CHECK ((`object_stage` in (_utf8mb4'staging',_utf8mb4'final'))),
  CONSTRAINT `chk_media_gc_job_status` CHECK ((`status` in (_utf8mb4'pending',_utf8mb4'processing',_utf8mb4'retry_wait',_utf8mb4'succeeded',_utf8mb4'deadletter')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_outbox`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_outbox` (
  `event_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `aggregate_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `aggregate_version` bigint unsigned NOT NULL,
  `operation` varchar(48) COLLATE utf8mb4_unicode_ci NOT NULL,
  `payload` json NOT NULL,
  `processed_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`event_id`),
  UNIQUE KEY `uq_media_outbox_aggregate_operation` (`aggregate_id`,`aggregate_version`,`operation`),
  KEY `idx_media_outbox_pending` (`processed_at`,`created_at`,`event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_quota_reservations`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_quota_reservations` (
  `reservation_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `owner_id` bigint unsigned NOT NULL,
  `bytes_reserved` bigint unsigned NOT NULL,
  `count_reserved` int unsigned NOT NULL,
  `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `ticket_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`reservation_id`),
  UNIQUE KEY `uq_media_quota_ticket` (`ticket_id`),
  KEY `idx_media_quota_owner_status` (`owner_id`,`status`),
  KEY `idx_media_quota_owner_created` (`owner_id`,`created_at`,`status`),
  CONSTRAINT `fk_media_quota_ticket` FOREIGN KEY (`ticket_id`) REFERENCES `media_upload_tickets` (`ticket_id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_quota_status` CHECK ((`status` in (_utf8mb4'active',_utf8mb4'committed',_utf8mb4'released')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_review_decisions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_review_decisions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `projection_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `review_task_id` bigint unsigned NOT NULL,
  `decision_version` bigint unsigned NOT NULL,
  `decision` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `payload_hash` char(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `result_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `received_at` datetime(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_media_review_decision_task_version` (`review_task_id`,`decision_version`),
  UNIQUE KEY `uq_media_review_decision_projection_version` (`projection_id`,`decision_version`),
  KEY `idx_media_review_decision_received` (`received_at`,`id`),
  CONSTRAINT `fk_media_review_decision_projection` FOREIGN KEY (`projection_id`) REFERENCES `media_review_projections` (`projection_id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_review_decision_status` CHECK ((`decision` in (_utf8mb4'approved',_utf8mb4'rejected',_utf8mb4'review_failed')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_review_projections`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_review_projections` (
  `projection_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `review_task_id` bigint unsigned DEFAULT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending',
  `applied_decision_version` bigint unsigned NOT NULL DEFAULT '0',
  `last_decision_hash` char(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `failure_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `submitted_at` datetime(3) DEFAULT NULL,
  `reviewing_at` datetime(3) DEFAULT NULL,
  `decided_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`projection_id`),
  UNIQUE KEY `uq_media_review_projection_asset` (`asset_id`),
  UNIQUE KEY `uq_media_review_projection_task` (`review_task_id`),
  KEY `idx_media_review_projection_status` (`status`,`updated_at`,`projection_id`),
  CONSTRAINT `fk_media_review_projection_asset` FOREIGN KEY (`asset_id`) REFERENCES `media_assets` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_review_projection_status` CHECK ((`status` in (_utf8mb4'pending',_utf8mb4'submitted',_utf8mb4'reviewing',_utf8mb4'approved',_utf8mb4'rejected',_utf8mb4'review_failed')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_upload_completions`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_upload_completions` (
  `ticket_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `phase` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `staging_version_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `staging_etag` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `staging_last_modified` datetime(3) DEFAULT NULL,
  `trusted_sha256` char(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `verified_mime` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `verified_size` bigint unsigned DEFAULT NULL,
  `verified_width` int unsigned DEFAULT NULL,
  `verified_height` int unsigned DEFAULT NULL,
  `final_key` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `final_version_id` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `final_etag` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `final_crc64` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_token` char(26) COLLATE utf8mb4_unicode_ci DEFAULT NULL,
  `lease_expires_at` datetime(3) DEFAULT NULL,
  `processing_deadline_at` datetime(3) NOT NULL,
  `validation_attempts` int unsigned NOT NULL DEFAULT '0',
  `promote_attempts` int unsigned NOT NULL DEFAULT '0',
  `last_error_code` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `completed_at` datetime(3) DEFAULT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`ticket_id`),
  UNIQUE KEY `uq_media_completion_asset` (`asset_id`),
  KEY `idx_media_completion_lease` (`phase`,`lease_expires_at`,`asset_id`),
  KEY `idx_media_completion_deadline` (`phase`,`processing_deadline_at`,`asset_id`),
  CONSTRAINT `fk_media_completion_asset` FOREIGN KEY (`asset_id`) REFERENCES `media_assets` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `fk_media_completion_ticket` FOREIGN KEY (`ticket_id`) REFERENCES `media_upload_tickets` (`ticket_id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_completion_phase` CHECK ((`phase` in (_utf8mb4'validating',_utf8mb4'promoting',_utf8mb4'final_ready',_utf8mb4'validation_failed')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `media_upload_tickets`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `media_upload_tickets` (
  `ticket_id` char(26) COLLATE utf8mb4_unicode_ci NOT NULL,
  `asset_id` bigint unsigned NOT NULL,
  `owner_id` bigint unsigned NOT NULL,
  `expected_size` bigint unsigned NOT NULL,
  `expected_md5` char(24) COLLATE utf8mb4_unicode_ci NOT NULL,
  `expected_content_type` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL,
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'created',
  `credential_issue_count` tinyint unsigned NOT NULL DEFAULT '0',
  `expires_at` datetime(3) NOT NULL,
  `row_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) NOT NULL,
  `updated_at` datetime(3) NOT NULL,
  PRIMARY KEY (`ticket_id`),
  UNIQUE KEY `uq_media_ticket_asset` (`asset_id`),
  KEY `idx_media_ticket_owner_status` (`owner_id`,`status`,`expires_at`),
  KEY `idx_media_ticket_expiry` (`status`,`expires_at`),
  KEY `idx_media_ticket_expiry_scan` (`status`,`expires_at`,`ticket_id`),
  CONSTRAINT `fk_media_ticket_asset` FOREIGN KEY (`asset_id`) REFERENCES `media_assets` (`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
  CONSTRAINT `chk_media_ticket_credential_count` CHECK ((`credential_issue_count` <= 1)),
  CONSTRAINT `chk_media_ticket_status` CHECK ((`status` in (_utf8mb4'created',_utf8mb4'authorization_issued',_utf8mb4'uploaded',_utf8mb4'consumed',_utf8mb4'expired',_utf8mb4'cancelled')))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `navigation_categories`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `navigation_categories` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `name` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sort` bigint NOT NULL DEFAULT '0',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_navigation_categories_sort` (`sort`)
) ENGINE=InnoDB AUTO_INCREMENT=2 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `navigation_items`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `navigation_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `category_id` bigint unsigned NOT NULL,
  `title` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `subtitle` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `icon_text` varchar(12) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `app_id` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL,
  `path` varchar(240) COLLATE utf8mb4_unicode_ci NOT NULL,
  `color` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sort` bigint NOT NULL DEFAULT '0',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_navigation_items_category_id` (`category_id`),
  KEY `idx_navigation_items_sort` (`sort`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `notifications`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `notifications` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `recipient_id` bigint unsigned NOT NULL,
  `actor_id` bigint unsigned NOT NULL,
  `type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `post_id` bigint unsigned NOT NULL DEFAULT '0',
  `comment_id` bigint unsigned NOT NULL DEFAULT '0',
  `read_at` datetime(3) DEFAULT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_notification_event` (`recipient_id`,`actor_id`,`type`,`post_id`,`comment_id`),
  KEY `idx_notifications_recipient_read_created` (`recipient_id`,`read_at`,`created_at`),
  KEY `idx_notifications_actor` (`actor_id`),
  KEY `idx_notifications_post` (`post_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `post_favorites`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `post_favorites` (
  `post_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`post_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `post_images`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `post_images` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint unsigned NOT NULL,
  `asset_id` bigint unsigned DEFAULT NULL,
  `url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sort` bigint NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_post_image_sort` (`post_id`,`sort`),
  KEY `idx_post_images_asset_id` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `post_likes`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `post_likes` (
  `post_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`post_id`,`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `posts`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `posts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `topic` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `title` varchar(120) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `content` text COLLATE utf8mb4_unicode_ci,
  `review_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'submitted',
  `pinned` tinyint(1) NOT NULL DEFAULT '0',
  `liked_count` bigint NOT NULL DEFAULT '0',
  `comment_count` bigint NOT NULL DEFAULT '0',
  `favorite_count` bigint NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_posts_user_id` (`user_id`),
  KEY `idx_posts_topic` (`topic`),
  KEY `idx_posts_review_status` (`review_status`),
  KEY `idx_posts_deleted_at` (`deleted_at`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `product_favorites`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `product_favorites` (
  `product_id` bigint unsigned NOT NULL,
  `user_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`product_id`,`user_id`),
  KEY `idx_product_favorites_user` (`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `product_images`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `product_images` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `product_id` bigint unsigned NOT NULL,
  `asset_id` bigint unsigned DEFAULT NULL,
  `url` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `sort` bigint NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_product_image_sort` (`product_id`,`sort`),
  KEY `idx_product_images_asset_id` (`asset_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `products`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `products` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `seller_id` bigint unsigned NOT NULL,
  `title` varchar(160) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `description` text COLLATE utf8mb4_unicode_ci,
  `category` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `price` bigint NOT NULL DEFAULT '0',
  `original_price` bigint NOT NULL DEFAULT '0',
  `condition_label` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `campus` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'selling',
  `review_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'submitted',
  `favorite_count` bigint NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_products_seller_id` (`seller_id`),
  KEY `idx_products_category` (`category`),
  KEY `idx_products_status` (`status`),
  KEY `idx_products_review_status` (`review_status`),
  KEY `idx_products_deleted_at` (`deleted_at`),
  KEY `idx_products_public_status_created` (`review_status`,`status`,`deleted_at`,`created_at`,`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `reports`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `reports` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `reporter_id` bigint unsigned NOT NULL,
  `target_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `target_id` bigint unsigned NOT NULL,
  `reason` varchar(240) COLLATE utf8mb4_unicode_ci NOT NULL,
  `detail` text COLLATE utf8mb4_unicode_ci,
  `review_status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'submitted',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_reports_reporter_id` (`reporter_id`),
  KEY `idx_reports_target_type` (`target_type`),
  KEY `idx_reports_target_id` (`target_id`),
  KEY `idx_reports_review_status` (`review_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `review_tasks`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `review_tasks` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `object_type` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL,
  `object_id` bigint unsigned NOT NULL,
  `submitter_id` bigint unsigned NOT NULL DEFAULT '0',
  `risk` varchar(240) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'submitted',
  `reason` varchar(240) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `operator_id` bigint unsigned NOT NULL DEFAULT '0',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_review_object` (`object_type`,`object_id`),
  KEY `idx_review_tasks_status` (`status`),
  KEY `idx_review_tasks_submitter_id` (`submitter_id`)
) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `trade_search_outbox`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `trade_search_outbox` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `product_id` bigint unsigned NOT NULL,
  `operation` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL,
  `attempts` bigint NOT NULL DEFAULT '0',
  `available_at` datetime(3) NOT NULL,
  `processed_at` datetime(3) DEFAULT NULL,
  `last_error` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_trade_search_outbox_product` (`product_id`),
  KEY `idx_trade_search_outbox_pending` (`processed_at`,`available_at`,`id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `user_follows`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `user_follows` (
  `follower_id` bigint unsigned NOT NULL,
  `followee_id` bigint unsigned NOT NULL,
  `created_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`follower_id`,`followee_id`),
  KEY `idx_user_follows_followee` (`followee_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
DROP TABLE IF EXISTS `users`;
/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!50503 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `open_id` varchar(96) COLLATE utf8mb4_unicode_ci NOT NULL,
  `nickname` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `avatar` varchar(512) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `avatar_asset_id` bigint unsigned DEFAULT NULL,
  `school` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `campus` varchar(80) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
  `role` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'user',
  `status` varchar(24) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'active',
  `token_version` bigint unsigned NOT NULL DEFAULT '1',
  `created_at` datetime(3) DEFAULT NULL,
  `updated_at` datetime(3) DEFAULT NULL,
  `deleted_at` datetime(3) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `open_id` (`open_id`),
  KEY `idx_users_deleted_at` (`deleted_at`),
  KEY `idx_users_avatar_asset_id` (`avatar_asset_id`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
/*!40101 SET character_set_client = @saved_cs_client */;
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*!40111 SET SQL_NOTES=@OLD_SQL_NOTES */;
