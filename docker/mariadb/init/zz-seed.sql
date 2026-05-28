USE `main`;

INSERT INTO `login`
  (account_id, userid,        user_pass,  sex, email,                  group_id, state, birthdate)
VALUES
  (2000001,    'admin',       'admin123', 'M', 'admin@racp.local',     99,       0,     '1990-01-01'),
  (2000002,    'staff_mod',   'mod123',   'F', 'mod@racp.local',       20,       0,     '1991-02-02'),
  (2000003,    'staff_enf',   'enf123',   'M', 'enf@racp.local',       10,       0,     '1992-03-03'),
  (2000004,    'event_nyx',   'event123', 'F', 'event@racp.local',     2,        0,     '1993-04-04'),
  (2000005,    'testuser',    'testpass', 'M', 'kaki@racp.local',      0,        0,     '1994-05-05'),
  (2000006,    'crazyarashi', 'testpass', 'F', 'arashi@racp.local',    0,        0,     '1995-06-06'),
  (2000007,    'player_01',   'testpass', 'M', 'p01@racp.local',       0,        0,     '1996-07-07'),
  (2000008,    'player_02',   'testpass', 'F', 'p02@racp.local',       0,        0,     '1997-08-08'),
  (2000009,    'player_03',   'testpass', 'M', 'p03@racp.local',       0,        1,     '1998-09-09'),
  (2000010,    'player_04',   'testpass', 'F', 'p04@racp.local',       0,        0,     '1999-10-10'),
  (2000011,    'player_05',   'testpass', 'M', 'p05@racp.local',       0,        1,     '2000-11-11'),
  (2000012,    'player_06',   'testpass', 'F', 'p06@racp.local',       0,        5,     '2001-12-12');

INSERT INTO `char`
  (char_id, account_id, char_num, name,             class, base_level, job_level, sex, str, agi, vit, `int`, dex, luk, max_hp, hp,   max_sp, sp,  zeny,    last_map,     save_map,     hair, hair_color)
VALUES
  (150000,  2000001,    0,        'AdminGod',       4060,  175,        70,        'M', 99,  99,  99,  99,    99,  99,  99999,  99999,9999,   9999,99999999,'prontera',   'prontera',   1,    6),
  (150001,  2000002,    0,        'ModRyo',         23,    99,         70,        'F', 60,  90,  50,  30,    99,  40,  9500,   9500, 2400,   2400,5000000, 'prontera',   'prontera',   2,    1),
  (150002,  2000002,    1,        'ModSora',        17,    85,         55,        'F', 1,   30,  60,  99,    20,  50,  6800,   6800, 5400,   5400,1200000, 'payon',      'payon',      3,    3),
  (150003,  2000003,    0,        'EnforcerKaze',   12,    90,         60,        'M', 90,  60,  80,  10,    50,  20,  12000,  12000,1200,   1200,3000000, 'geffen',     'geffen',     1,    0),
  (150004,  2000003,    1,        'EnforcerMira',   16,    78,         50,        'F', 30,  99,  40,  20,    70,  30,  7200,   7200, 1500,   1500,800000,  'alberta',    'alberta',    4,    2),
  (150005,  2000004,    0,        'EventNyx',       18,    99,         70,        'F', 1,   40,  50,  99,    30,  60,  6500,   6500, 7800,   7800,4500000, 'comodo',     'comodo',     2,    4),
  (150006,  2000005,    0,        'KakiArcher',     11,    75,         50,        'M', 30,  90,  30,  20,    99,  40,  5800,   5800, 1100,   1100,250000,  'payon',      'payon',      5,    1),
  (150007,  2000005,    1,        'KakiMage',       9,     60,         40,        'M', 1,   30,  30,  90,    20,  40,  3200,   3200, 4200,   4200,80000,   'geffen',     'geffen',     6,    2),
  (150008,  2000005,    2,        'KakiHunter',     11,    50,         35,        'F', 25,  80,  25,  20,    85,  30,  3900,   3900, 800,    800, 45000,   'morocc',     'morocc',     7,    3),
  (150009,  2000006,    0,        'ArashiNinja',    25,    99,         70,        'F', 50,  99,  50,  60,    80,  40,  9200,   9200, 3300,   3300,7800000, 'amatsu',     'amatsu',     8,    5),
  (150010,  2000006,    1,        'ArashiSamurai',  7,     80,         55,        'M', 99,  60,  70,  10,    60,  20,  10800,  10800,900,    900, 600000,  'amatsu',     'amatsu',     9,    6),
  (150011,  2000006,    2,        'ArashiPriest',   8,     70,         45,        'F', 10,  40,  60,  90,    30,  60,  5600,   5600, 6200,   6200,320000,  'prontera',   'prontera',   10,   7),
  (150012,  2000007,    0,        'BladeRider',     12,    85,         50,        'M', 80,  70,  60,  10,    50,  20,  10200,  10200,1000,   1000,1500000, 'izlude',     'izlude',     1,    0),
  (150013,  2000007,    1,        'ShadowWalker',   16,    70,         50,        'F', 30,  99,  40,  10,    70,  50,  6400,   6400, 1200,   1200,420000,  'morocc',     'morocc',     11,   2),
  (150014,  2000007,    2,        'IronFist',       19,    60,         40,        'M', 90,  30,  80,  10,    20,  10,  8200,   8200, 500,    500, 95000,   'prontera',   'prontera',   12,   4),
  (150015,  2000008,    0,        'MysticBow',      11,    95,         60,        'F', 40,  99,  40,  30,    99,  40,  7800,   7800, 1800,   1800,2200000, 'payon',      'payon',      13,   1),
  (150016,  2000008,    1,        'FrozenStar',     14,    80,         55,        'M', 80,  50,  90,  20,    50,  30,  13200,  13200,1100,   1100,1100000, 'lutie',      'lutie',      14,   3),
  (150017,  2000008,    2,        'EmberSong',      18,    65,         45,        'F', 1,   30,  40,  85,    20,  50,  4500,   4500, 5200,   5200,180000,  'geffen',     'geffen',     15,   5),
  (150018,  2000009,    0,        'GoldenLance',    7,     90,         60,        'M', 99,  60,  85,  10,    60,  20,  14800,  14800,1100,   1100,2800000, 'prontera',   'prontera',   16,   6),
  (150019,  2000009,    1,        'SilverDust',     16,    75,         50,        'F', 30,  99,  40,  10,    70,  50,  6800,   6800, 1300,   1300,520000,  'morocc',     'morocc',     17,   2),
  (150020,  2000009,    2,        'IronVeil',       12,    60,         40,        'M', 70,  50,  60,  10,    40,  20,  7600,   7600, 800,    800, 110000,  'izlude',     'izlude',     18,   0),
  (150021,  2000010,    0,        'NightProwler',   16,    99,         70,        'F', 35,  99,  50,  20,    90,  60,  8800,   8800, 1800,   1800,4200000, 'morocc',     'morocc',     19,   4),
  (150022,  2000010,    1,        'DawnSeeker',     11,    85,         55,        'M', 35,  90,  40,  20,    99,  40,  6400,   6400, 1300,   1300,750000,  'payon',      'payon',      20,   1),
  (150023,  2000010,    2,        'TwilightSage',   9,     70,         45,        'F', 1,   30,  40,  99,    20,  50,  3800,   3800, 5800,   5800,210000,  'geffen',     'geffen',     21,   3),
  (150024,  2000011,    0,        'StormCaller',    18,    95,         65,        'M', 1,   40,  50,  99,    30,  60,  6200,   6200, 7400,   7400,3300000, 'comodo',     'comodo',     22,   5),
  (150025,  2000011,    1,        'FrostWhisper',   14,    80,         55,        'F', 70,  60,  90,  20,    50,  40,  12800,  12800,1200,   1200,890000,  'lutie',      'lutie',      23,   2),
  (150026,  2000011,    2,        'SunBlade',       7,     65,         45,        'M', 90,  50,  70,  10,    50,  20,  9400,   9400, 700,    700, 145000,  'prontera',   'prontera',   24,   6),
  (150027,  2000012,    0,        'MoonShade',      19,    90,         60,        'F', 80,  40,  80,  20,    40,  30,  11200,  11200,900,    900, 950000,  'prontera',   'prontera',   25,   7),
  (150028,  2000012,    1,        'RuneCaller',     15,    75,         50,        'M', 1,   40,  40,  90,    30,  60,  5200,   5200, 6400,   6400,330000,  'juno',       'juno',       26,   1),
  (150029,  2000012,    2,        'ThornWeaver',    17,    60,         40,        'F', 1,   30,  40,  85,    20,  50,  4100,   4100, 4800,   4800,135000,  'prontera',   'prontera',   1,    3),
  (150030,  2000012,    3,        'IronOath',       7,     50,         35,        'M', 70,  40,  60,  10,    40,  20,  6800,   6800, 600,    600, 60000,   'prontera',   'prontera',   2,    0),
  (150031,  2000012,    4,        'GaleStrike',     11,    45,         30,        'F', 25,  75,  30,  20,    80,  30,  3400,   3400, 700,    700, 38000,   'payon',      'payon',      3,    5);

INSERT INTO `guild`
  (guild_id, name,                char_id, master,           guild_lv, connect_member, max_member, average_lv, mes1,                                 mes2)
VALUES
  (1,        'EmperiumKings',     150000,  'AdminGod',       50,       3,              56,         170,        'The first guild on the server.',     'Welcome home.'),
  (2,        'CrimsonOrder',      150001,  'ModRyo',         12,       2,              28,         92,         'Hunters of the crimson dawn.',       ''),
  (3,        'SilentBlades',      150003,  'EnforcerKaze',   8,        1,              24,         85,         'Justice in silence.',                ''),
  (4,        'NyxConclave',       150005,  'EventNyx',       15,       2,              30,         95,         'Where the night gathers.',           ''),
  (5,        'ArrowOfDawn',       150006,  'KakiArcher',     5,        1,              20,         72,         'Aim true, shoot fast.',              ''),
  (6,        'NinjaWardens',      150009,  'ArashiNinja',    18,       3,              36,         98,         'Shadows of Amatsu.',                 ''),
  (7,        'BladeStorm',        150012,  'BladeRider',     7,        1,              22,         80,         'Charge with us.',                    ''),
  (8,        'ShadowGuild',       150013,  'ShadowWalker',   6,        1,              22,         75,         'Trust no light.',                    ''),
  (9,        'MysticHaven',       150015,  'MysticBow',      9,        2,              26,         88,         'Mages and marksmen welcome.',        ''),
  (10,       'FrozenAegis',       150016,  'FrozenStar',     7,        1,              22,         78,         'Tank lives matter.',                 ''),
  (11,       'GoldenLegion',      150018,  'GoldenLance',    11,       2,              28,         87,         'Forged in Prontera steel.',          ''),
  (12,       'SilverWings',       150019,  'SilverDust',     5,        1,              20,         72,         'Run fast. Hit hard.',                ''),
  (13,       'NightHunters',      150021,  'NightProwler',   13,       2,              30,         96,         'PvM and WoE friendly.',              ''),
  (14,       'StormChasers',      150024,  'StormCaller',    14,       2,              30,         93,         'Lightning never asks permission.',   ''),
  (15,       'MoonlitOrder',      150027,  'MoonShade',      10,       2,              26,         89,         'Devotees of the moon.',              ''),
  (16,       'RuneSeekers',       150028,  'RuneCaller',     6,        1,              22,         74,         'Scholars and runesmiths.',           ''),
  (17,       'EmberFlame',        150017,  'EmberSong',      4,        1,              20,         64,         'Newbies welcome.',                   ''),
  (18,       'SunlitDawn',        150022,  'DawnSeeker',     8,        1,              24,         83,         'Wake up with us.',                   ''),
  (19,       'TwilightCovenant',  150023,  'TwilightSage',   6,        1,              22,         70,         'Wisdom over haste.',                 ''),
  (20,       'IronVeilClan',      150020,  'IronVeil',       3,        1,              16,         60,         'Small but loyal.',                   '');

UPDATE `char` SET guild_id = 1  WHERE char_id = 150000;
UPDATE `char` SET guild_id = 2  WHERE char_id = 150001;
UPDATE `char` SET guild_id = 3  WHERE char_id = 150003;
UPDATE `char` SET guild_id = 4  WHERE char_id = 150005;
UPDATE `char` SET guild_id = 5  WHERE char_id = 150006;
UPDATE `char` SET guild_id = 6  WHERE char_id = 150009;
UPDATE `char` SET guild_id = 7  WHERE char_id = 150012;
UPDATE `char` SET guild_id = 8  WHERE char_id = 150013;
UPDATE `char` SET guild_id = 9  WHERE char_id = 150015;
UPDATE `char` SET guild_id = 10 WHERE char_id = 150016;
UPDATE `char` SET guild_id = 11 WHERE char_id = 150018;
UPDATE `char` SET guild_id = 12 WHERE char_id = 150019;
UPDATE `char` SET guild_id = 13 WHERE char_id = 150021;
UPDATE `char` SET guild_id = 14 WHERE char_id = 150024;
UPDATE `char` SET guild_id = 15 WHERE char_id = 150027;
UPDATE `char` SET guild_id = 16 WHERE char_id = 150028;
UPDATE `char` SET guild_id = 17 WHERE char_id = 150017;
UPDATE `char` SET guild_id = 18 WHERE char_id = 150022;
UPDATE `char` SET guild_id = 19 WHERE char_id = 150023;
UPDATE `char` SET guild_id = 20 WHERE char_id = 150020;

UPDATE `char` SET guild_id = 1 WHERE char_id IN (150002, 150004);
UPDATE `char` SET guild_id = 2 WHERE char_id IN (150007, 150030);
UPDATE `char` SET guild_id = 6 WHERE char_id IN (150010, 150011);
UPDATE `char` SET guild_id = 9 WHERE char_id IN (150008, 150031);
UPDATE `char` SET guild_id = 13 WHERE char_id IN (150025, 150026);
UPDATE `char` SET guild_id = 11 WHERE char_id IN (150014, 150029);

INSERT INTO `guild_member` (guild_id, char_id, position) VALUES
  (1,  150000, 0), (1,  150002, 5), (1,  150004, 5),
  (2,  150001, 0), (2,  150007, 5), (2,  150030, 5),
  (3,  150003, 0),
  (4,  150005, 0),
  (5,  150006, 0),
  (6,  150009, 0), (6,  150010, 1), (6,  150011, 5),
  (7,  150012, 0),
  (8,  150013, 0),
  (9,  150015, 0), (9,  150008, 5), (9,  150031, 5),
  (10, 150016, 0),
  (11, 150018, 0), (11, 150014, 5), (11, 150029, 5),
  (12, 150019, 0),
  (13, 150021, 0), (13, 150025, 1), (13, 150026, 5),
  (14, 150024, 0),
  (15, 150027, 0),
  (16, 150028, 0),
  (17, 150017, 0),
  (18, 150022, 0),
  (19, 150023, 0),
  (20, 150020, 0);

INSERT INTO `guild_position` (guild_id, position, name, mode, exp_mode)
SELECT g.guild_id, p.position, p.name, p.mode, p.exp_mode
FROM `guild` g
CROSS JOIN (
  SELECT 0 AS position, 'GuildMaster'  AS name, 17 AS mode, 100 AS exp_mode UNION ALL
  SELECT 1,             'Officer',          17,     50 UNION ALL
  SELECT 2,             'Veteran',          1,      50 UNION ALL
  SELECT 3,             'Member',           0,      50 UNION ALL
  SELECT 4,             'Newbie',           0,      50 UNION ALL
  SELECT 5,             'Newcomer',         0,      0  UNION ALL
  SELECT 6,             'Position 6',       0,      0  UNION ALL
  SELECT 7,             'Position 7',       0,      0  UNION ALL
  SELECT 8,             'Position 8',       0,      0  UNION ALL
  SELECT 9,             'Position 9',       0,      0
) p;

INSERT INTO `cart_inventory` (id, char_id, nameid, amount, identify) VALUES
  (1,  150010, 501,  500,  1),
  (2,  150010, 502,  300,  1),
  (3,  150011, 607,  50,   1),
  (4,  150011, 608,  20,   1),
  (5,  150014, 511,  1000, 1),
  (6,  150014, 512,  800,  1),
  (7,  150015, 1201, 1,    1),
  (8,  150015, 1101, 1,    1),
  (9,  150017, 714,  10,   1),
  (10, 150017, 969,  500,  1),
  (11, 150020, 601,  100,  1),
  (12, 150020, 602,  100,  1),
  (13, 150023, 7227, 99,   1),
  (14, 150023, 7228, 99,   1),
  (15, 150025, 7048, 50,   1),
  (16, 150025, 7049, 50,   1),
  (17, 150026, 1601, 1,    1),
  (18, 150026, 2104, 1,    1),
  (19, 150029, 7615, 30,   1),
  (20, 150029, 7616, 30,   1);

INSERT INTO `vendings`
  (id, account_id, char_id, sex, map,        x,   y,   title,                          autotrade)
VALUES
  (1,  2000006,    150010,  'M', 'prontera', 145, 175, 'Cheap potions, fresh stock',   1),
  (2,  2000006,    150011,  'F', 'prontera', 152, 180, 'Yggdrasil berries B>30 ea',    1),
  (3,  2000007,    150014,  'M', 'prontera', 160, 178, 'Herbs and Apples - bulk',      1),
  (4,  2000008,    150015,  'F', 'payon',    165, 100, 'Bows and Knives',              1),
  (5,  2000008,    150017,  'F', 'prontera', 156, 165, 'Emperium 999z - WoE prep',     1),
  (6,  2000009,    150020,  'M', 'izlude',   128, 98,  'Wings, wings, wings',          1),
  (7,  2000010,    150023,  'F', 'geffen',   119, 70,  'Crystal jewels',               1),
  (8,  2000011,    150025,  'F', 'lutie',    143, 142, 'Holy water + arrows',          1),
  (9,  2000011,    150026,  'M', 'prontera', 148, 170, 'Knight gear - cheap',          1),
  (10, 2000012,    150029,  'F', 'prontera', 154, 168, 'Bloody Branches - lol no',     1);

INSERT INTO `vending_items` (vending_id, `index`, cartinventory_id, amount, price) VALUES
  (1,  0, 1,  500,  150),
  (1,  1, 2,  300,  300),
  (2,  0, 3,  50,   30000),
  (2,  1, 4,  20,   25000),
  (3,  0, 5,  1000, 50),
  (3,  1, 6,  800,  60),
  (4,  0, 7,  1,    8000),
  (4,  1, 8,  1,    6500),
  (5,  0, 9,  10,   999),
  (5,  1, 10, 500,  100),
  (6,  0, 11, 100,  600),
  (6,  1, 12, 100,  700),
  (7,  0, 13, 99,   12000),
  (7,  1, 14, 99,   14000),
  (8,  0, 15, 50,   200),
  (8,  1, 16, 50,   220),
  (9,  0, 17, 1,    180000),
  (9,  1, 18, 1,    95000),
  (10, 0, 19, 30,   3500),
  (10, 1, 20, 30,   3800);

INSERT INTO `buyingstores`
  (id, account_id, char_id, sex, map,        x,   y,   title,                             `limit`,    autotrade)
VALUES
  (1,  2000005,    150006,  'M', 'prontera', 147, 178, 'B> Jellopy 10z each',             1000000,    1),
  (2,  2000005,    150007,  'M', 'geffen',   119, 70,  'B> Yellow Gemstones 600z',        2000000,    1),
  (3,  2000005,    150008,  'F', 'payon',    167, 105, 'B> Trunks 25z',                   500000,     1),
  (4,  2000007,    150013,  'F', 'morocc',   156, 95,  'B> Animal Hair 80z',              800000,     1),
  (5,  2000008,    150016,  'M', 'lutie',    143, 145, 'B> Stiletto +0 5000z',            1500000,    1),
  (6,  2000009,    150018,  'M', 'prontera', 158, 180, 'B> Pieces of Bamboo 30z',         600000,     1),
  (7,  2000009,    150019,  'F', 'morocc',   158, 90,  'B> Star Crumb 1500z',             3000000,    1),
  (8,  2000010,    150022,  'M', 'payon',    170, 110, 'B> Tough Vines 120z',             1200000,    1),
  (9,  2000011,    150024,  'M', 'comodo',   190, 150, 'B> Worn Out Pages 25000z',        5000000,    1),
  (10, 2000012,    150027,  'F', 'prontera', 160, 175, 'B> White Herb 90z',               900000,     1);

INSERT INTO `buyingstore_items` (buyingstore_id, `index`, item_id, amount, price) VALUES
  (1,  0, 909,  500, 10),
  (1,  1, 910,  500, 12),
  (2,  0, 715,  100, 600),
  (3,  0, 1019, 200, 25),
  (4,  0, 914,  200, 80),
  (5,  0, 1201, 1,   5000),
  (6,  0, 1019, 300, 30),
  (7,  0, 1000, 50,  1500),
  (8,  0, 905,  150, 120),
  (9,  0, 7433, 20,  25000),
  (10, 0, 509,  500, 90);

INSERT INTO `cart_inventory` (id, char_id, nameid, amount, identify) VALUES
  (21, 150014, 501,  200, 1),
  (22, 150017, 501,  200, 1),
  (23, 150020, 501,  200, 1),
  (24, 150025, 501,  200, 1),
  (25, 150015, 607,  30,  1),
  (26, 150023, 607,  30,  1),
  (27, 150026, 607,  30,  1),
  (28, 150010, 714,  5,   1),
  (29, 150026, 714,  5,   1),
  (30, 150017, 512,  300, 1),
  (31, 150025, 512,  300, 1);

INSERT INTO `vending_items` (vending_id, `index`, cartinventory_id, amount, price) VALUES
  (3, 2, 21, 200, 165),
  (5, 2, 22, 200, 145),
  (6, 2, 23, 200, 155),
  (8, 2, 24, 200, 160),
  (4, 2, 25, 30,  28000),
  (7, 2, 26, 30,  32000),
  (9, 2, 27, 30,  29500),
  (1, 2, 28, 5,   1100),
  (9, 3, 29, 5,   950),
  (5, 3, 30, 300, 65),
  (8, 3, 31, 300, 55);

INSERT INTO `buyingstore_items` (buyingstore_id, `index`, item_id, amount, price) VALUES
  (4, 1, 909,  500, 12),
  (6, 1, 909,  500, 8),
  (5, 1, 715,  100, 650),
  (9, 1, 715,  100, 580),
  (8, 1, 1019, 200, 28);

INSERT INTO `login`
  (account_id, userid,       user_pass,  sex, email,             group_id, state, birthdate)
VALUES
  (2000013,    'vendor_01',  'testpass', 'M', 'v01@racp.local',  0,        0,     '1995-01-15'),
  (2000014,    'vendor_02',  'testpass', 'F', 'v02@racp.local',  0,        0,     '1995-02-15'),
  (2000015,    'vendor_03',  'testpass', 'M', 'v03@racp.local',  0,        0,     '1995-03-15'),
  (2000016,    'vendor_04',  'testpass', 'F', 'v04@racp.local',  0,        0,     '1995-04-15'),
  (2000017,    'vendor_05',  'testpass', 'M', 'v05@racp.local',  0,        0,     '1995-05-15'),
  (2000018,    'vendor_06',  'testpass', 'F', 'v06@racp.local',  0,        0,     '1995-06-15'),
  (2000019,    'vendor_07',  'testpass', 'M', 'v07@racp.local',  0,        0,     '1995-07-15'),
  (2000020,    'vendor_08',  'testpass', 'F', 'v08@racp.local',  0,        1,     '1995-08-15'),
  (2000021,    'vendor_09',  'testpass', 'M', 'v09@racp.local',  0,        0,     '1995-09-15'),
  (2000022,    'vendor_10',  'testpass', 'F', 'v10@racp.local',  0,        0,     '1995-10-15'),
  (2000023,    'vendor_11',  'testpass', 'M', 'v11@racp.local',  0,        0,     '1995-11-15'),
  (2000024,    'vendor_12',  'testpass', 'F', 'v12@racp.local',  0,        0,     '1995-12-15'),
  (2000025,    'vendor_13',  'testpass', 'M', 'v13@racp.local',  0,        0,     '1996-01-15'),
  (2000026,    'player_07',  'testpass', 'M', 'p07@racp.local',  0,        0,     '1996-02-15'),
  (2000027,    'player_08',  'testpass', 'F', 'p08@racp.local',  0,        1,     '1996-03-15');

INSERT INTO `char`
  (char_id, account_id, char_num, name,           class, base_level, job_level, sex, str, agi, vit, `int`, dex, luk, max_hp, hp,   max_sp, sp,  zeny,    last_map,   save_map,   hair, hair_color, online)
VALUES
  (150032, 2000013, 0, 'TradeKingOne',  5,  65, 50, 'M', 80, 40, 60, 30, 70, 40, 7200, 7200, 900,  900,  2400000, 'prontera', 'prontera', 1,  1, 1),
  (150033, 2000014, 0, 'BizQueenTwo',   5,  60, 45, 'F', 75, 45, 55, 30, 65, 50, 6600, 6600, 850,  850,  1800000, 'prontera', 'prontera', 2,  3, 1),
  (150034, 2000015, 0, 'CoinHoarder',   5,  70, 55, 'M', 85, 40, 65, 25, 72, 38, 7800, 7800, 800,  800,  3100000, 'geffen',   'geffen',   3,  4, 1),
  (150035, 2000016, 0, 'DealMakerF',    5,  55, 42, 'F', 70, 50, 50, 35, 60, 45, 5900, 5900, 950,  950,  1400000, 'payon',    'payon',    4,  2, 1),
  (150036, 2000017, 0, 'ZenyBaron',     5,  68, 52, 'M', 82, 42, 62, 28, 70, 40, 7500, 7500, 870,  870,  2900000, 'morocc',   'morocc',   5,  5, 1),
  (150037, 2000018, 0, 'BargainBel',    5,  52, 40, 'F', 68, 48, 48, 35, 58, 48, 5500, 5500, 980,  980,  1200000, 'izlude',   'izlude',   6,  1, 1),
  (150038, 2000019, 0, 'StockSage',     5,  63, 48, 'M', 78, 42, 58, 30, 68, 42, 6900, 6900, 880,  880,  2200000, 'comodo',   'comodo',   7,  6, 1),
  (150039, 2000020, 0, 'PennyWise',     5,  58, 44, 'F', 72, 46, 52, 33, 62, 46, 6200, 6200, 920,  920,  1600000, 'prontera', 'prontera', 8,  3, 1),
  (150040, 2000021, 0, 'GoldGrubber',   5,  66, 50, 'M', 81, 41, 61, 29, 71, 39, 7300, 7300, 860,  860,  2600000, 'lutie',    'lutie',    9,  7, 1),
  (150041, 2000022, 0, 'MarketMaven',   5,  61, 46, 'F', 74, 44, 56, 31, 64, 44, 6500, 6500, 900,  900,  2000000, 'prontera', 'prontera', 10, 2, 1),
  (150042, 2000023, 0, 'VendVeteran',   5,  69, 54, 'M', 84, 40, 64, 26, 72, 38, 7700, 7700, 810,  810,  3000000, 'geffen',   'geffen',   11, 4, 1),
  (150043, 2000024, 0, 'CartCaptain',   5,  57, 43, 'F', 71, 47, 51, 34, 61, 47, 6100, 6100, 940,  940,  1500000, 'prontera', 'prontera', 12, 1, 1),
  (150044, 2000025, 0, 'LedgerLord',    5,  64, 49, 'M', 79, 42, 59, 30, 69, 41, 7000, 7000, 875,  875,  2300000, 'payon',    'payon',    13, 6, 1),
  (150045, 2000013, 1, 'TradeKingAlt',  1,  30, 18, 'M', 20, 25, 18, 12, 22, 14, 2100, 2100, 320,  320,  40000,   'prontera', 'prontera', 1,  0, 0),
  (150046, 2000014, 1, 'BizQueenAlt',   1,  28, 16, 'F', 18, 24, 16, 12, 20, 16, 1900, 1900, 340,  340,  35000,   'prontera', 'prontera', 2,  0, 0),
  (150047, 2000015, 1, 'CoinHoardAlt',  1,  32, 20, 'M', 22, 26, 20, 12, 24, 14, 2300, 2300, 300,  300,  48000,   'geffen',   'geffen',   3,  0, 0),
  (150048, 2000016, 1, 'DealMakeAlt',   1,  25, 14, 'F', 16, 22, 15, 12, 18, 16, 1700, 1700, 360,  360,  28000,   'payon',    'payon',    4,  0, 0),
  (150049, 2000017, 1, 'ZenyBaronAlt',  1,  35, 22, 'M', 24, 28, 22, 12, 26, 14, 2500, 2500, 290,  290,  55000,   'morocc',   'morocc',   5,  0, 0),
  (150050, 2000026, 0, 'PeacefulOne',   11, 80, 58, 'M', 35, 90, 35, 25, 95, 45, 6400, 6400, 1300, 1300, 900000,  'payon',    'payon',    14, 3, 1),
  (150051, 2000026, 1, 'PeacefulTwo',   9,  40, 28, 'M', 10, 30, 28, 70, 22, 30, 2600, 2600, 3200, 3200, 90000,   'geffen',   'geffen',   15, 1, 0),
  (150052, 2000026, 2, 'PeacefulTri',   12, 55, 40, 'F', 70, 50, 55, 12, 45, 22, 6800, 6800, 700,  700,  220000,  'izlude',   'izlude',   16, 5, 0),
  (150053, 2000027, 0, 'RestlessOne',   16, 75, 52, 'F', 30, 95, 40, 18, 80, 55, 6200, 6200, 1200, 1200, 850000,  'morocc',   'morocc',   17, 4, 1),
  (150054, 2000027, 1, 'RestlessTwo',   14, 50, 38, 'M', 70, 55, 80, 18, 48, 30, 9200, 9200, 600,  600,  150000,  'lutie',    'lutie',    18, 2, 0),
  (150055, 2000027, 2, 'RestlessTri',   7,  60, 45, 'M', 85, 50, 70, 10, 55, 20, 9800, 9800, 700,  700,  300000,  'prontera', 'prontera', 19, 6, 0),
  (150056, 2000001, 1, 'AdminAlt',      19, 120,70, 'M', 90, 60, 90, 40, 60, 50, 18000,18000,2000, 2000, 5000000, 'prontera', 'prontera', 20, 7, 0),
  (150057, 2000002, 2, 'ModAlt',        17, 70, 48, 'F', 12, 40, 55, 90, 30, 55, 5400, 5400, 6200, 6200, 400000,  'prontera', 'prontera', 21, 1, 0),
  (150058, 2000018, 1, 'BargainAlt',    9,  30, 20, 'F', 10, 28, 26, 60, 20, 30, 2200, 2200, 2800, 2800, 60000,   'geffen',   'geffen',   22, 3, 0),
  (150059, 2000019, 1, 'StockAlt',      11, 45, 32, 'M', 28, 75, 30, 20, 80, 35, 3600, 3600, 800,  800,  120000,  'payon',    'payon',    23, 2, 0),
  (150060, 2000020, 1, 'PennyAlt',      16, 40, 30, 'F', 25, 80, 35, 15, 65, 45, 3400, 3400, 700,  700,  95000,   'morocc',   'morocc',   24, 4, 0),
  (150061, 2000021, 1, 'GoldAlt',       7,  50, 36, 'M', 75, 45, 60, 10, 45, 20, 7200, 7200, 600,  600,  140000,  'prontera', 'prontera', 25, 6, 0);

UPDATE `buyingstores` SET char_id = 150032, account_id = 2000013, sex = 'M' WHERE id = 2;
UPDATE `buyingstores` SET char_id = 150033, account_id = 2000014, sex = 'F' WHERE id = 3;
UPDATE `buyingstores` SET char_id = 150035, account_id = 2000016, sex = 'F' WHERE id = 4;
UPDATE `buyingstores` SET char_id = 150037, account_id = 2000018, sex = 'F' WHERE id = 5;
UPDATE `buyingstores` SET char_id = 150038, account_id = 2000019, sex = 'M' WHERE id = 6;
UPDATE `buyingstores` SET char_id = 150039, account_id = 2000020, sex = 'F' WHERE id = 7;
UPDATE `buyingstores` SET char_id = 150040, account_id = 2000021, sex = 'M' WHERE id = 8;
UPDATE `buyingstores` SET char_id = 150042, account_id = 2000023, sex = 'M' WHERE id = 9;
UPDATE `buyingstores` SET char_id = 150044, account_id = 2000025, sex = 'M' WHERE id = 10;

UPDATE `vendings` SET char_id = 150034, account_id = 2000015, sex = 'M' WHERE id = 2;
UPDATE `vendings` SET char_id = 150036, account_id = 2000017, sex = 'M' WHERE id = 5;
UPDATE `vendings` SET char_id = 150041, account_id = 2000022, sex = 'F' WHERE id = 9;
UPDATE `vendings` SET char_id = 150043, account_id = 2000024, sex = 'F' WHERE id = 10;

UPDATE `cart_inventory` SET char_id = 150034 WHERE id IN (3, 4);
UPDATE `cart_inventory` SET char_id = 150036 WHERE id IN (9, 10, 22, 30);
UPDATE `cart_inventory` SET char_id = 150041 WHERE id IN (17, 18, 27, 29);
UPDATE `cart_inventory` SET char_id = 150043 WHERE id IN (19, 20);

UPDATE `char` SET online = 1 WHERE char_id IN (
  150006, 150010, 150014, 150015, 150020, 150023, 150025,
  150000, 150001, 150003, 150005
);
