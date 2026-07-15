-- --------------------------------------------------------
-- Host:                         127.0.0.1
-- Server version:               11.8.6-MariaDB-0+deb13u1 from Debian - -- Please help get to 10k stars at https://github.com/MariaDB/Server
-- Server OS:                    debian-linux-gnu
-- HeidiSQL Version:             12.18.1.1
-- --------------------------------------------------------

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET NAMES utf8 */;
/*!50503 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*!40111 SET @OLD_SQL_NOTES=@@SQL_NOTES, SQL_NOTES=0 */;


-- Dumping database structure for wfc
CREATE DATABASE IF NOT EXISTS `wfc` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_uca1400_ai_ci */;
USE `wfc`;

-- Dumping structure for table wfc.events
CREATE TABLE IF NOT EXISTS `events` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `event_type` tinytext NOT NULL,
  `event_data` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL CHECK (json_valid(`event_data`)),
  `event_time` timestamp NOT NULL DEFAULT utc_timestamp(),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Data exporting was unselected.

-- Dumping structure for table wfc.friends
CREATE TABLE IF NOT EXISTS `friends` (
  `sender` int(10) unsigned NOT NULL,
  `recipient` int(10) unsigned NOT NULL,
  `authorized` tinyint(1) unsigned NOT NULL DEFAULT 0,
  `created` timestamp NOT NULL DEFAULT utc_timestamp(),
  UNIQUE KEY `sender_recipient` (`sender`,`recipient`),
  KEY `FK_friends_profiles_2` (`recipient`),
  CONSTRAINT `FK_friends_profiles` FOREIGN KEY (`sender`) REFERENCES `profiles` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `FK_friends_profiles_2` FOREIGN KEY (`recipient`) REFERENCES `profiles` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;

-- Data exporting was unselected.

-- Dumping structure for table wfc.gamestats_persistdata
CREATE TABLE IF NOT EXISTS `gamestats_persistdata` (
  `pid` int(10) unsigned NOT NULL,
  `ptype` tinyint(3) unsigned NOT NULL,
  `dindex` int(10) unsigned NOT NULL,
  `data` text NOT NULL,
  `modified` timestamp NOT NULL DEFAULT utc_timestamp(),
  UNIQUE KEY `one_pdata_constraint` (`pid`,`ptype`,`dindex`) USING BTREE,
  CONSTRAINT `FK_gamestats_persistent_data_profiles` FOREIGN KEY (`pid`) REFERENCES `profiles` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Data exporting was unselected.

-- Dumping structure for table wfc.mario_kart_wii_sake
CREATE TABLE IF NOT EXISTS `mario_kart_wii_sake` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `regionid` smallint(5) unsigned NOT NULL,
  `courseid` smallint(5) unsigned NOT NULL,
  `score` int(10) unsigned NOT NULL,
  `pid` int(10) unsigned NOT NULL,
  `playerinfo` varchar(108) NOT NULL,
  `ghost` longblob DEFAULT NULL,
  `upload_time` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `one_time_per_course_constraint` (`courseid`,`pid`),
  KEY `FK_mario_kart_wii_sake_profiles` (`pid`),
  CONSTRAINT `FK_mario_kart_wii_sake_profiles` FOREIGN KEY (`pid`) REFERENCES `profiles` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `mario_kart_wii_sake_regionid_check` CHECK (`regionid` >= 1 and `regionid` <= 7),
  CONSTRAINT `mario_kart_wii_sake_courseid_check` CHECK (`courseid` >= 0 and `courseid` <= 32767),
  CONSTRAINT `mario_kart_wii_sake_score_check` CHECK (`score` > 0 and `score` < 360000),
  CONSTRAINT `mario_kart_wii_sake_playerinfo_check` CHECK (octet_length(`playerinfo`) = 108),
  CONSTRAINT `mario_kart_wii_sake_ghost_check` CHECK (`ghost` is null or octet_length(`ghost`) >= 148 and octet_length(`ghost`) <= 10240)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Data exporting was unselected.

-- Dumping structure for table wfc.profiles
CREATE TABLE IF NOT EXISTS `profiles` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `gsbrcd` char(11) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  `firstname` tinytext CHARACTER SET ascii COLLATE ascii_general_ci DEFAULT NULL,
  `lastname` tinytext CHARACTER SET ascii COLLATE ascii_general_ci DEFAULT NULL,
  `last_ingamesn` tinytext NOT NULL DEFAULT '',
  `last_ip_address` tinytext CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL DEFAULT '',
  `has_ban` tinyint(1) DEFAULT 0,
  `ban_issued` timestamp NULL DEFAULT NULL,
  `ban_expires` timestamp NULL DEFAULT NULL,
  `ban_reason` tinytext DEFAULT NULL,
  `ban_reason_hidden` tinytext DEFAULT NULL,
  `ban_moderator` tinytext DEFAULT NULL,
  `ban_tos` tinyint(1) DEFAULT NULL,
  PRIMARY KEY (`id`) USING BTREE,
  KEY `user_id` (`user_id`),
  CONSTRAINT `FK_profiles_users` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB AUTO_INCREMENT=1000000000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Data exporting was unselected.

-- Dumping structure for table wfc.sake_records
CREATE TABLE IF NOT EXISTS `sake_records` (
  `game_id` int(10) unsigned NOT NULL,
  `table_id` tinytext NOT NULL,
  `record_id` int(10) unsigned NOT NULL DEFAULT rand(2147483647),
  `owner_id` int(10) unsigned NOT NULL,
  `fields` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL CHECK (json_valid(`fields`)),
  `create_time` timestamp NOT NULL DEFAULT utc_timestamp(),
  `update_time` timestamp NOT NULL DEFAULT utc_timestamp(),
  UNIQUE KEY `one_sake_record_constraint` (`game_id`,`table_id`,`record_id`) USING HASH,
  KEY `FK_sake_records_profiles` (`owner_id`),
  CONSTRAINT `FK_sake_records_profiles` FOREIGN KEY (`owner_id`) REFERENCES `profiles` (`id`) ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT `sake_records_fields_check` CHECK (json_length(json_keys(`fields`)) <= 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Data exporting was unselected.

-- Dumping structure for table wfc.users
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint(20) unsigned NOT NULL,
  `unitcd` tinyint(1) unsigned NOT NULL,
  `macadr` char(12) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
  `passwd` smallint(6) DEFAULT NULL COMMENT 'ds only',
  `csnum` char(11) CHARACTER SET ascii COLLATE ascii_general_ci DEFAULT NULL COMMENT 'wii only',
  PRIMARY KEY (`id`) USING BTREE,
  UNIQUE KEY `macadr` (`macadr`),
  UNIQUE KEY `csnum` (`csnum`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_uca1400_ai_ci;

-- Data exporting was unselected.

/*!40103 SET TIME_ZONE=IFNULL(@OLD_TIME_ZONE, 'system') */;
/*!40101 SET SQL_MODE=IFNULL(@OLD_SQL_MODE, '') */;
/*!40014 SET FOREIGN_KEY_CHECKS=IFNULL(@OLD_FOREIGN_KEY_CHECKS, 1) */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40111 SET SQL_NOTES=IFNULL(@OLD_SQL_NOTES, 1) */;
