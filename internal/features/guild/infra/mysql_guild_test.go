//go:build integration

package infra

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/hayakawakaki/go-racp/internal/features/guild/domain"
	"github.com/hayakawakaki/go-racp/internal/testutil"
)

func randomSuffix(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}

	return hex.EncodeToString(b[:])
}

func insertChar(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	res, err := db.Exec("INSERT INTO `char` (account_id, name, sex) VALUES (?, ?, 'M')", 1, name)
	if err != nil {
		t.Fatalf("insert char %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("char LastInsertId: %v", err)
	}
	charID := int(id)
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM `char` WHERE char_id = ?", charID) })

	return charID
}

type seedOpts struct {
	emblemData []byte
	emblemLen  int
	members    []seedMember
	positions  []seedPosition
	guildLevel int
	maxMember  int
}

type seedMember struct {
	charName string
	position int
}

type seedPosition struct {
	name     string
	position int
}

type seedResult struct {
	guildID       int
	masterID      int
	memberCharIDs []int
}

func seedGuild(t *testing.T, db *sql.DB, name string, opts seedOpts) seedResult {
	t.Helper()
	masterName := name + "_m"
	masterCharID := insertChar(t, db, masterName)

	res, err := db.Exec(
		"INSERT INTO guild (name, char_id, master, guild_lv, max_member, emblem_len, emblem_data) VALUES (?, ?, ?, ?, ?, ?, ?)",
		name, masterCharID, masterName, opts.guildLevel, opts.maxMember, opts.emblemLen, opts.emblemData,
	)
	if err != nil {
		t.Fatalf("insert guild %q: %v", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("guild LastInsertId: %v", err)
	}
	guildID := int(id)
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM guild WHERE guild_id = ?", guildID)
		_, _ = db.Exec("DELETE FROM guild_member WHERE guild_id = ?", guildID)
		_, _ = db.Exec("DELETE FROM guild_position WHERE guild_id = ?", guildID)
	})

	if _, err := db.Exec("INSERT INTO guild_member (guild_id, char_id, position) VALUES (?, ?, 0)", guildID, masterCharID); err != nil {
		t.Fatalf("insert master guild_member: %v", err)
	}

	memberIDs := []int{masterCharID}
	for _, m := range opts.members {
		charID := insertChar(t, db, m.charName)
		if _, err := db.Exec("INSERT INTO guild_member (guild_id, char_id, position) VALUES (?, ?, ?)", guildID, charID, m.position); err != nil {
			t.Fatalf("insert guild_member %q: %v", m.charName, err)
		}
		memberIDs = append(memberIDs, charID)
	}

	for _, p := range opts.positions {
		if _, err := db.Exec("INSERT INTO guild_position (guild_id, position, name) VALUES (?, ?, ?)", guildID, p.position, p.name); err != nil {
			t.Fatalf("insert guild_position %q: %v", p.name, err)
		}
	}

	return seedResult{guildID: guildID, masterID: masterCharID, memberCharIDs: memberIDs}
}

func TestRepository_GetByID_Hit(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{guildLevel: 5, maxMember: 16})

	got, err := repo.GetByID(context.Background(), seed.guildID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != seed.guildID || got.Name != "kaki_"+suf {
		t.Errorf("got = %+v, want id=%d name=kaki_%s", got, seed.guildID, suf)
	}
	if got.MasterCharID != seed.masterID {
		t.Errorf("MasterCharID = %d, want %d", got.MasterCharID, seed.masterID)
	}
	if got.GuildLevel != 5 || got.MaxMember != 16 {
		t.Errorf("level/max = %d/%d, want 5/16", got.GuildLevel, got.MaxMember)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)

	_, err := repo.GetByID(context.Background(), -1)
	if !errors.Is(err, domain.ErrGuildNotFound) {
		t.Errorf("got %v, want ErrGuildNotFound", err)
	}
}

func TestRepository_List_PaginatesAndSearches(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	wanted := seedGuild(t, db, "kaki_"+suf, seedOpts{guildLevel: 5, maxMember: 16})
	_ = seedGuild(t, db, "other_"+suf, seedOpts{guildLevel: 10, maxMember: 32})

	page, err := repo.List(context.Background(), domain.ListQuery{Page: 1, PerPage: 20, Query: "kaki_" + suf})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Total != 1 || len(page.Guilds) != 1 {
		t.Fatalf("page total/len = %d/%d, want 1/1", page.Total, len(page.Guilds))
	}
	if page.Guilds[0].ID != wanted.guildID {
		t.Errorf("returned ID = %d, want %d", page.Guilds[0].ID, wanted.guildID)
	}
}

func TestRepository_List_DefaultsAndClamps(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)

	page, err := repo.List(context.Background(), domain.ListQuery{Page: 0, PerPage: 0, Query: "__never_matches__"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Page != 1 {
		t.Errorf("Page = %d, want 1 (clamped)", page.Page)
	}
	if page.PerPage != DefaultPerPage {
		t.Errorf("PerPage = %d, want %d", page.PerPage, DefaultPerPage)
	}
}

func TestRepository_ListMembers_OrderingAndPositionJoin(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{
		guildLevel: 5,
		maxMember:  16,
		members: []seedMember{
			{charName: "crazyarashi_" + suf, position: 1},
			{charName: "zeta_" + suf, position: 1},
		},
		positions: []seedPosition{
			{position: 0, name: "GuildMaster"},
			{position: 1, name: "Officer"},
		},
	})

	members, err := repo.ListMembers(context.Background(), seed.guildID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 3 {
		t.Fatalf("len(members) = %d, want 3", len(members))
	}
	if members[0].Position != 0 || members[0].PositionName != "GuildMaster" {
		t.Errorf("master row = %+v, want position 0 GuildMaster", members[0])
	}
	if members[1].Name != "crazyarashi_"+suf {
		t.Errorf("second row name = %q, want crazyarashi_%s (alphabetical within position)", members[1].Name, suf)
	}
	if members[2].Name != "zeta_"+suf {
		t.Errorf("third row name = %q, want zeta_%s", members[2].Name, suf)
	}
	if members[1].PositionName != "Officer" || members[2].PositionName != "Officer" {
		t.Errorf("officer rows position names: %q / %q, want Officer", members[1].PositionName, members[2].PositionName)
	}
}

func TestRepository_ListMembers_EmptyGuild(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)

	members, err := repo.ListMembers(context.Background(), -1)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("len(members) = %d, want 0", len(members))
	}
}

func TestRepository_ListMembers_MissingPositionFallsBackToEmpty(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{
		guildLevel: 1,
		maxMember:  16,
		members:    []seedMember{{charName: "lone_" + suf, position: 5}},
	})

	members, err := repo.ListMembers(context.Background(), seed.guildID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("len(members) = %d, want 2", len(members))
	}
	var lone domain.Member
	for _, m := range members {
		if m.Position == 5 {
			lone = m
			break
		}
	}
	if lone.PositionName != "" {
		t.Errorf("PositionName = %q, want empty (no guild_position row)", lone.PositionName)
	}
}

func TestRepository_GetEmblem_BMP(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	blob := []byte{'B', 'M', 0x01, 0x02, 0x03}
	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{emblemData: blob, emblemLen: len(blob)})

	data, mime, err := repo.GetEmblem(context.Background(), seed.guildID)
	if err != nil {
		t.Fatalf("GetEmblem: %v", err)
	}
	if mime != "image/bmp" {
		t.Errorf("mime = %q, want image/bmp", mime)
	}
	if string(data) != string(blob) {
		t.Errorf("data = %x, want %x", data, blob)
	}
}

func TestRepository_GetEmblem_GIF(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	blob := []byte("GIF89a\x00\x00\x00")
	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{emblemData: blob, emblemLen: len(blob)})

	data, mime, err := repo.GetEmblem(context.Background(), seed.guildID)
	if err != nil {
		t.Fatalf("GetEmblem: %v", err)
	}
	if mime != "image/gif" {
		t.Errorf("mime = %q, want image/gif", mime)
	}
	if string(data) != string(blob) {
		t.Errorf("data = %q, want %q", data, blob)
	}
}

func TestRepository_GetEmblem_Empty(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{emblemLen: 0})

	_, _, err := repo.GetEmblem(context.Background(), seed.guildID)
	if !errors.Is(err, domain.ErrEmblemEmpty) {
		t.Errorf("got %v, want ErrEmblemEmpty", err)
	}
}

func TestRepository_GetEmblem_UnknownFormat(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)
	suf := randomSuffix(t)

	blob := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	seed := seedGuild(t, db, "kaki_"+suf, seedOpts{emblemData: blob, emblemLen: len(blob)})

	_, _, err := repo.GetEmblem(context.Background(), seed.guildID)
	if !errors.Is(err, domain.ErrEmblemUnknownFormat) {
		t.Errorf("got %v, want ErrEmblemUnknownFormat", err)
	}
}

func TestRepository_GetEmblem_GuildNotFound(t *testing.T) {
	db := testutil.OpenMariaDB(t, "DB_MAIN_URL")
	repo := NewRepository(db)

	_, _, err := repo.GetEmblem(context.Background(), -1)
	if !errors.Is(err, domain.ErrGuildNotFound) {
		t.Errorf("got %v, want ErrGuildNotFound", err)
	}
}
