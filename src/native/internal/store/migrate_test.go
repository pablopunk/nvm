package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"testing"

	"github.com/pablopunk/nvm/native/internal/platform/fake"
)

// renameTally is the v1 to v2 step of the test chain: count becomes tally.
func renameTally(data json.RawMessage) (json.RawMessage, error) {
	return renameField(data, "count", "tally", nil)
}

// doubleTally is the v2 to v3 step of the test chain: tally returns to count, doubled.
func doubleTally(data json.RawMessage) (json.RawMessage, error) {
	return renameField(data, "tally", "count", func(value int) int { return value * 2 })
}

// renameField moves one JSON field to a new name, optionally transforming its integer value.
func renameField(data json.RawMessage, from, to string, transform func(int) int) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	value, ok := fields[from]
	if !ok {
		return data, nil
	}
	delete(fields, from)
	if transform != nil {
		var number int
		if err := json.Unmarshal(value, &number); err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(transform(number))
		if err != nil {
			return nil, err
		}
		value = encoded
	}
	fields[to] = value
	return json.Marshal(fields)
}

// migrationChain returns the two-step chain from version 1 to version 3.
func migrationChain() []Migration {
	return []Migration{
		{From: 1, Up: renameTally},
		{From: 2, Up: doubleTally},
	}
}

func TestMigrationStepV1ToV2RenamesCount(t *testing.T) {
	got, err := renameTally([]byte(`{"title":"a","count":2}`))
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if string(got) != `{"tally":2,"title":"a"}` {
		t.Fatalf("step output = %s", got)
	}
}

func TestMigrationStepV2ToV3DoublesTally(t *testing.T) {
	got, err := doubleTally([]byte(`{"title":"a","tally":2}`))
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if string(got) != `{"count":4,"title":"a"}` {
		t.Fatalf("step output = %s", got)
	}
	if _, err := doubleTally([]byte(`{"tally":"two"}`)); err == nil {
		t.Fatal("a non-numeric tally was accepted")
	}
}

func TestMigrationChainUpgradesV1FileAndKeepsPreMigrationCopy(t *testing.T) {
	s, files, _ := newFixture(t)
	original := readTestdata(t, "v1_settings.json")
	files.Seed(settingsPath, original)
	doc := Doc(s, "settings.json", 3, freshSettingsDoc, migrationChain()...)

	got, err := doc.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Title != "v1 title" || got.Count != 4 || len(got.Items) != 1 {
		t.Fatalf("migrated document = %+v", got)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath+".premigrate-v1"), original) {
		t.Fatal("the pre-migration copy does not hold the original bytes")
	}
	version, _ := envelopeOf(t, mustRead(t, files, settingsPath))
	if version != 3 {
		t.Fatalf("file after migration is version %d, want 3", version)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath+".bak.1"), original) {
		t.Fatal("the migration did not keep the original as the newest backup")
	}
}

func TestMigrationStepsRunInOrderOnce(t *testing.T) {
	s, files, _ := newFixture(t)
	files.Seed(settingsPath, readTestdata(t, "v1_settings.json"))
	var order []int
	chain := []Migration{
		{From: 1, Up: func(data json.RawMessage) (json.RawMessage, error) {
			order = append(order, 1)
			return renameTally(data)
		}},
		{From: 2, Up: func(data json.RawMessage) (json.RawMessage, error) {
			order = append(order, 2)
			return doubleTally(data)
		}},
	}
	if _, err := Doc(s, "settings.json", 3, freshSettingsDoc, chain...).Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("migration order = %v, want [1 2]", order)
	}
}

func TestFailingMigrationLeavesFileAndOpensReadOnly(t *testing.T) {
	s, files, _ := newFixture(t)
	original := readTestdata(t, "v1_settings.json")
	files.Seed(settingsPath, original)
	chain := []Migration{
		{From: 1, Up: renameTally},
		{From: 2, Up: func(json.RawMessage) (json.RawMessage, error) { return nil, errors.New("step broke") }},
	}
	doc := Doc(s, "settings.json", 3, freshSettingsDoc, chain...)
	if _, err := doc.Load(); !errors.Is(err, ErrMigration) {
		t.Fatalf("Load = %v, want ErrMigration", err)
	}
	if err := doc.Update(func(v *settingsDoc) { v.Count = 1 }); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Update = %v, want ErrReadOnly", err)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath), original) {
		t.Fatal("a failed migration changed the file")
	}
	if _, err := files.Stat(settingsPath + ".premigrate-v1"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a pre-migration copy was written for a migration that never completed: %v", err)
	}
	if _, ok := findNotice(drainNotices(s.Notices()), LoadFailed); !ok {
		t.Fatal("no LoadFailed notice for the failed migration")
	}
}

func TestMigrationOutputMustBeValidJSONAndMatchTheDocument(t *testing.T) {
	cases := map[string]Migration{
		"invalid JSON": {From: 1, Up: func(json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage("{"), nil
		}},
		"wrong shape": {From: 1, Up: func(json.RawMessage) (json.RawMessage, error) {
			return json.RawMessage(`"just a string"`), nil
		}},
	}
	for name, step := range cases {
		t.Run(name, func(t *testing.T) {
			s, files, _ := newFixture(t)
			files.Seed(settingsPath, readTestdata(t, "v1_settings.json"))
			_, err := Doc(s, "settings.json", 2, freshSettingsDoc, step).Load()
			if !errors.Is(err, ErrMigration) {
				t.Fatalf("Load = %v, want ErrMigration", err)
			}
		})
	}
}

func TestExistingPreMigrationCopyIsNeverReplaced(t *testing.T) {
	s, files, _ := newFixture(t)
	files.Seed(settingsPath, readTestdata(t, "v1_settings.json"))
	earlier := []byte("first pre-migration state")
	files.Seed(settingsPath+".premigrate-v1", earlier)
	if _, err := Doc(s, "settings.json", 3, freshSettingsDoc, migrationChain()...).Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(mustRead(t, files, settingsPath+".premigrate-v1"), earlier) {
		t.Fatal("an existing pre-migration copy was replaced")
	}
}

func TestFolderInThePreMigrationPathBlocksTheMigration(t *testing.T) {
	s, files, _ := newFixture(t)
	original := readTestdata(t, "v1_settings.json")
	files.Seed(settingsPath, original)
	if err := files.MkdirAll(settingsPath+".premigrate-v1", 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	doc := Doc(s, "settings.json", 3, freshSettingsDoc, migrationChain()...)
	if _, err := doc.Load(); err == nil {
		t.Fatal("Load migrated without being able to keep a copy")
	}
	if !bytes.Equal(mustRead(t, files, settingsPath), original) {
		t.Fatal("the original file changed without a copy")
	}
}

func TestMigrationChainSurvivesAFreshProcess(t *testing.T) {
	files := fake.NewFS()
	clock := fake.NewClock()
	files.Seed(settingsPath, readTestdata(t, "v1_settings.json"))
	first := openStoreOn(t, files, clock)
	if _, err := Doc(first, "settings.json", 3, freshSettingsDoc, migrationChain()...).Load(); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	second := openStoreOn(t, files, clock)
	got, err := Doc(second, "settings.json", 3, freshSettingsDoc, migrationChain()...).Load()
	if err != nil || got.Count != 4 {
		t.Fatalf("second process Load = %+v, %v; want the migrated document", got, err)
	}
}
