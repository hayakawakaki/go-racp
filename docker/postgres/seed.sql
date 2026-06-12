INSERT INTO cp_currency (account_id, zeny, cashpoint) VALUES
  (2000005, 10000000000, 1000000),
  (2000006, 10000000000, 1000000),
  (2000007, 10000000000, 1000000),
  (2000008, 10000000000, 1000000),
  (2000010, 10000000000, 1000000),
  (2000026, 10000000000, 1000000)
ON CONFLICT (account_id) DO UPDATE SET zeny = EXCLUDED.zeny, cashpoint = EXCLUDED.cashpoint;

DO $seed$
DECLARE
  tid BIGINT;
BEGIN

IF EXISTS (SELECT 1 FROM cp_news LIMIT 1) THEN
  RAISE NOTICE 'cp seed: already seeded, skipping';
  RETURN;
END IF;

INSERT INTO cp_news (title, body, category, created_at) VALUES
  ('Welcome to the server!',
   'Server is live. Make an account, roll a character, ask questions in #general. PvM is open, WoE schedule is in #woe.',
   'Announcement',
   NOW() - INTERVAL '12 days'),
  ('Patch 1.0.3 deployed',
   'Rebalanced exp rates for 99+ characters. Card drop rate +10% in MVP maps. See full changelog in #patch-notes.',
   'Patch',
   NOW() - INTERVAL '6 days'),
  ('Christmas event live',
   'Talk to Santa in Lutie for daily gifts. Snowflakes drop x3 from Christmas mobs. Event ends Jan 5.',
   'Event',
   NOW() - INTERVAL '3 days'),
  ('Maintenance Saturday 03:00 UTC',
   'Brief restart for storage migration. Expected downtime ~15 minutes. Save your auto-trade carts before this.',
   'Announcement',
   NOW() - INTERVAL '36 hours'),
  ('WoE schedule update',
   'WoE 1.0 moves to Saturdays 20:00 UTC. WoE 2.0 stays Sunday 21:00 UTC. Castle drops doubled until end of season.',
   'Update',
   NOW() - INTERVAL '8 hours');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
VALUES (2000007, 'player_01', 'BugReport', 'Stuck after warp from Geffen', 'open', 'player', 1, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000007, 'player', 'public',
   'After warping out of Geffen I cannot move. Tried relog, still stuck at coords 119,66. Please help.',
   NOW() - INTERVAL '2 hours');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
VALUES (2000005, 'testuser', 'AccountHelp', 'Cannot change email - says verified already', 'open', 'staff', 3, NOW() - INTERVAL '6 hours', NOW() - INTERVAL '1 day')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000005, 'player', 'public',
   'Trying to change email from kaki@racp.local to a new one. Form just sits there.',
   NOW() - INTERVAL '1 day'),
  (tid, 2000002, 'staff',  'public',
   'Hi - which browser are you on? Also, did you receive a confirmation email recently?',
   NOW() - INTERVAL '20 hours'),
  (tid, 2000002, 'staff',  'internal',
   'Possible smtp queue delay - cross-check mailpit logs around the requested timestamp.',
   NOW() - INTERVAL '20 hours' + INTERVAL '30 seconds');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
VALUES (2000008, 'player_02', 'Donation', 'Donation pack not delivered', 'open', 'player', 4, NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '3 days')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000008, 'player', 'public',
   'I bought the November VIP pack but did not receive in-game items. Transaction id #ABC-12345.',
   NOW() - INTERVAL '3 days'),
  (tid, 2000002, 'staff',  'public',
   'Looking into this. Can you confirm character name to deliver on?',
   NOW() - INTERVAL '2 days'),
  (tid, 2000008, 'player', 'public',
   'MysticBow please.',
   NOW() - INTERVAL '2 days' + INTERVAL '5 minutes'),
  (tid, 2000008, 'player', 'public',
   'Any update?',
   NOW() - INTERVAL '30 minutes');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
VALUES (2000006, 'crazyarashi', 'BugReport', 'Cart vending crashes client', 'resolved', 'staff', 5, NOW() - INTERVAL '4 days', NOW() - INTERVAL '7 days')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000006, 'player', 'public',
   'Opening cart vending in Prontera crashes my client. Repro 100%.',
   NOW() - INTERVAL '7 days'),
  (tid, 2000003, 'staff',  'public',
   'Thanks for reporting. Reproduced on test server. Pushing a fix.',
   NOW() - INTERVAL '6 days'),
  (tid, 2000003, 'staff',  'internal',
   'Root cause: malformed packet 0x0136 when cart has 0 items.',
   NOW() - INTERVAL '6 days'),
  (tid, 2000006, 'player', 'public',
   'Fix works for me, thanks!',
   NOW() - INTERVAL '5 days'),
  (tid, 2000003, 'staff',  'system',
   'Marked resolved.',
   NOW() - INTERVAL '4 days');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, closed_by, created_at)
VALUES (2000009, 'player_03', 'Other', 'Question about pet system', 'closed', 'staff', 2, NOW() - INTERVAL '10 days', 2000002, NOW() - INTERVAL '14 days')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000009, 'player', 'public',
   'Where do I get pet food for poporing?',
   NOW() - INTERVAL '14 days'),
  (tid, 2000002, 'staff',  'public',
   'Apple Juice from kafra cafe, or NPC in pron 138/199. Closing after no follow-up.',
   NOW() - INTERVAL '10 days');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
VALUES (2000010, 'player_04', 'BugReport', 'Skill icon missing for new RG skill', 'open', 'player', 1, NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '15 minutes')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000010, 'player', 'public',
   'Royal Guard "Genesis Ray" icon is the default questionmark. Skill works fine though.',
   NOW() - INTERVAL '15 minutes');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, created_at)
VALUES (2000011, 'player_05', 'Donation', 'Costume not in storage', 'resolved', 'staff', 3, NOW() - INTERVAL '1 day', NOW() - INTERVAL '3 days')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000011, 'player', 'public',
   'Got a Drooping Cat costume from October pack but not in any storage.',
   NOW() - INTERVAL '3 days'),
  (tid, 2000002, 'staff',  'public',
   'Delivered to char StormCaller. Please confirm.',
   NOW() - INTERVAL '2 days'),
  (tid, 2000011, 'player', 'public',
   'Got it, thank you.',
   NOW() - INTERVAL '1 day');

INSERT INTO cp_tickets (account_id, author_username, category, subject, status, last_actor, message_count, last_activity, closed_by, created_at)
VALUES (2000012, 'player_06', 'AccountHelp', 'Why am I banned???', 'closed', 'staff', 2, NOW() - INTERVAL '2 days', 2000001, NOW() - INTERVAL '5 days')
RETURNING id INTO tid;
INSERT INTO cp_ticket_messages (ticket_id, author_id, author_role, visibility, body, created_at) VALUES
  (tid, 2000012, 'player', 'public',
   'I did nothing wrong, please unban.',
   NOW() - INTERVAL '5 days'),
  (tid, 2000001, 'staff',  'public',
   'Ban is final. Multiple unique violations. See discord post in #appeals.',
   NOW() - INTERVAL '2 days');

INSERT INTO cp_ticket_views (account_id, ticket_id, last_viewed)
SELECT 2000002, id, last_activity - INTERVAL '1 minute'
FROM cp_tickets
WHERE status IN ('open','resolved');

INSERT INTO cp_audit_log (actor_user_id, target_user_id, action, reason, before_value, after_value, created_at) VALUES
  (2000001, 2000012, 'ban',
   'Multiple unique violations confirmed via packet logs',
   '0,0',  '5,0',                      NOW() - INTERVAL '5 days'),
  (2000001, 2000011, 'set_role',
   'Promoted to event helper',
   '0',    '2',                        NOW() - INTERVAL '4 days'),
  (2000002, 2000009, 'ban',
   'Verbal abuse in #global',
   '1,0',  '0,1735862400',             NOW() - INTERVAL '3 days'),
  (2000001, 2000009, 'unban',
   'Apology accepted, first offense',
   '0,1735862400', '0,0',              NOW() - INTERVAL '2 days'),
  (2000003, 2000010, 'ban',
   'RMT solicitation in trade chat',
   '0,0',  '0,1735948800',             NOW() - INTERVAL '36 hours'),
  (2000001, 2000003, 'set_role',
   'Recruited to enforcer team',
   '0',    '10',                       NOW() - INTERVAL '20 days'),
  (2000001, 2000002, 'set_role',
   'Promoted to moderator after probation',
   '10',   '20',                       NOW() - INTERVAL '60 days');

RAISE NOTICE 'cp seed: applied successfully';

END $seed$;
