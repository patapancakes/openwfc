-- --------------------------------------------------------
-- Host:                         localhost
-- Server version:               11.8.6-MariaDB-0+deb13u1 from Debian - -- Please help get to 10k stars at https://github.com/MariaDB/Server
-- Server OS:                    debian-linux-gnu
-- HeidiSQL Version:             12.18.0.7304
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
CREATE DATABASE IF NOT EXISTS `wfc` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci */;
USE `wfc`;

-- Dumping structure for table wfc.events
CREATE TABLE IF NOT EXISTS `events` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `event_type` tinytext NOT NULL,
  `event_data` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL CHECK (json_valid(`event_data`)),
  `event_time` timestamp NOT NULL DEFAULT utc_timestamp(),
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Dumping data for table wfc.events: ~0 rows (approximately)

-- Dumping structure for table wfc.gamestats_public_data
CREATE TABLE IF NOT EXISTS `gamestats_public_data` (
  `profile_id` int(10) unsigned NOT NULL,
  `dindex` tinytext NOT NULL,
  `ptype` tinytext NOT NULL,
  `pdata` text NOT NULL,
  `modified_time` timestamp NOT NULL,
  KEY `one_pdata_constraint` (`profile_id`,`dindex`(255),`ptype`(255)) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Dumping data for table wfc.gamestats_public_data: ~0 rows (approximately)

-- Dumping structure for table wfc.mario_kart_wii_sake
CREATE TABLE IF NOT EXISTS `mario_kart_wii_sake` (
  `regionid` smallint(5) unsigned NOT NULL,
  `courseid` smallint(5) unsigned NOT NULL,
  `score` int(10) unsigned NOT NULL,
  `pid` int(10) unsigned NOT NULL,
  `playerinfo` varchar(108) NOT NULL,
  `ghost` longblob DEFAULT NULL,
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `upload_time` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `one_time_per_course_constraint` (`courseid`,`pid`),
  CONSTRAINT `mario_kart_wii_sake_regionid_check` CHECK (`regionid` >= 1 and `regionid` <= 7),
  CONSTRAINT `mario_kart_wii_sake_courseid_check` CHECK (`courseid` >= 0 and `courseid` <= 32767),
  CONSTRAINT `mario_kart_wii_sake_score_check` CHECK (`score` > 0 and `score` < 360000),
  CONSTRAINT `mario_kart_wii_sake_pid_check` CHECK (`pid` > 0),
  CONSTRAINT `mario_kart_wii_sake_playerinfo_check` CHECK (octet_length(`playerinfo`) = 108),
  CONSTRAINT `mario_kart_wii_sake_ghost_check` CHECK (`ghost` is null or octet_length(`ghost`) >= 148 and octet_length(`ghost`) <= 10240)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Dumping data for table wfc.mario_kart_wii_sake: ~0 rows (approximately)

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
  CONSTRAINT `sake_records_fields_check` CHECK (json_length(json_keys(`fields`)) <= 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Dumping data for table wfc.sake_records: ~0 rows (approximately)

-- Dumping structure for table wfc.users
CREATE TABLE IF NOT EXISTS `users` (
  `profile_id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint(20) unsigned NOT NULL,
  `gsbrcd` tinytext NOT NULL,
  `password` tinytext NOT NULL,
  `ng_device_id` int(10) unsigned DEFAULT NULL,
  `email` tinytext NOT NULL,
  `unique_nick` tinytext NOT NULL,
  `firstname` tinytext DEFAULT NULL,
  `lastname` tinytext DEFAULT '',
  `last_ip_address` tinytext DEFAULT '',
  `last_ingamesn` tinytext DEFAULT '',
  `has_ban` tinyint(1) DEFAULT 0,
  `ban_issued` timestamp NULL DEFAULT NULL,
  `ban_expires` timestamp NULL DEFAULT NULL,
  `ban_reason` tinytext DEFAULT NULL,
  `ban_reason_hidden` tinytext DEFAULT NULL,
  `ban_moderator` tinytext DEFAULT NULL,
  `ban_tos` tinyint(1) DEFAULT NULL,
  `open_host` tinyint(1) DEFAULT 0,
  `allow_default_keys` tinyint(1) DEFAULT 0,
  PRIMARY KEY (`profile_id`)
) ENGINE=InnoDB AUTO_INCREMENT=1000000000 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- Dumping data for table wfc.users: ~0 rows (approximately)

/*!40103 SET TIME_ZONE=IFNULL(@OLD_TIME_ZONE, 'system') */;
/*!40101 SET SQL_MODE=IFNULL(@OLD_SQL_MODE, '') */;
/*!40014 SET FOREIGN_KEY_CHECKS=IFNULL(@OLD_FOREIGN_KEY_CHECKS, 1) */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40111 SET SQL_NOTES=IFNULL(@OLD_SQL_NOTES, 1) */;
