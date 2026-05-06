USE `log`;

CREATE TABLE IF NOT EXISTS `picklog` (
  `id` int(11) NOT NULL auto_increment,
  `time` datetime NOT NULL,
  `char_id` int(11) NOT NULL default '0',
  `type` enum('M','P','L','T','V','S','N','C','A','R','G','E','B','O','I','X','D','U','$','F','Y','Z','Q','H','J','W','0','1','2','3') NOT NULL default 'P',
  `nameid` int(10) unsigned NOT NULL default '0',
  `amount` int(11) NOT NULL default '1',
  `refine` tinyint(3) unsigned NOT NULL default '0',
  `card0` int(10) unsigned NOT NULL default '0',
  `card1` int(10) unsigned NOT NULL default '0',
  `card2` int(10) unsigned NOT NULL default '0',
  `card3` int(10) unsigned NOT NULL default '0',
  `option_id0` smallint(5) NOT NULL default '0',
  `option_val0` smallint(5) NOT NULL default '0',
  `option_parm0` tinyint(3) NOT NULL default '0',
  `option_id1` smallint(5) NOT NULL default '0',
  `option_val1` smallint(5) NOT NULL default '0',
  `option_parm1` tinyint(3) NOT NULL default '0',
  `option_id2` smallint(5) NOT NULL default '0',
  `option_val2` smallint(5) NOT NULL default '0',
  `option_parm2` tinyint(3) NOT NULL default '0',
  `option_id3` smallint(5) NOT NULL default '0',
  `option_val3` smallint(5) NOT NULL default '0',
  `option_parm3` tinyint(3) NOT NULL default '0',
  `option_id4` smallint(5) NOT NULL default '0',
  `option_val4` smallint(5) NOT NULL default '0',
  `option_parm4` tinyint(3) NOT NULL default '0',
  `unique_id` bigint(20) unsigned NOT NULL default '0',
  `map` varchar(11) NOT NULL default '',
  `bound` tinyint(1) unsigned NOT NULL default '0',
  `enchantgrade` tinyint unsigned NOT NULL default '0',
  PRIMARY KEY  (`id`),
  INDEX (`type`)
) ENGINE=MyISAM AUTO_INCREMENT=1;

CREATE TABLE IF NOT EXISTS `zenylog` (
  `id` int(11) NOT NULL auto_increment,
  `time` datetime NOT NULL,
  `char_id` int(11) NOT NULL default '0',
  `src_id` int(11) NOT NULL default '0',
  `type` enum('T','V','P','M','S','N','D','C','A','E','I','B','K','J','X','0','2') NOT NULL default 'S',
  `amount` int(11) NOT NULL default '0',
  `map` varchar(11) NOT NULL default '',
  PRIMARY KEY  (`id`),
  INDEX (`type`)
) ENGINE=MyISAM AUTO_INCREMENT=1;

CREATE TABLE IF NOT EXISTS `mvplog` (
  `mvp_id` mediumint(9) unsigned NOT NULL auto_increment,
  `mvp_date` datetime NOT NULL,
  `kill_char_id` int(11) NOT NULL default '0',
  `monster_id` smallint(6) NOT NULL default '0',
  `prize` int(10) unsigned NOT NULL default '0',
  `mvpexp` bigint(20) unsigned NOT NULL default '0',
  `map` varchar(11) NOT NULL default '',
  PRIMARY KEY  (`mvp_id`)
) ENGINE=MyISAM AUTO_INCREMENT=1;

CREATE TABLE IF NOT EXISTS `atcommandlog` (
  `atcommand_id` mediumint(9) unsigned NOT NULL auto_increment,
  `atcommand_date` datetime NOT NULL,
  `account_id` int(11) unsigned NOT NULL default '0',
  `char_id` int(11) unsigned NOT NULL default '0',
  `char_name` varchar(25) NOT NULL default '',
  `map` varchar(11) NOT NULL default '',
  `command` varchar(255) NOT NULL default '',
  PRIMARY KEY  (`atcommand_id`),
  INDEX (`account_id`),
  INDEX (`char_id`)
) ENGINE=MyISAM AUTO_INCREMENT=1;