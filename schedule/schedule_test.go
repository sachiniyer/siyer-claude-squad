package schedule

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestSchedules writes a schedules file to a temp dir and overrides
// getSchedulesPath for the duration of the test.
func setupTestSchedules(t *testing.T, schedules []Schedule) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, schedulesFileName)

	if schedules != nil {
		data, err := json.MarshalIndent(schedules, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, data, 0644))
	}

	// Override the path resolver for the test
	origGetPath := getSchedulesPathFn
	getSchedulesPathFn = func() (string, error) { return path, nil }
	t.Cleanup(func() { getSchedulesPathFn = origGetPath })

	return path
}

func TestLoadSchedulesEmpty(t *testing.T) {
	setupTestSchedules(t, nil) // no file on disk
	schedules, err := LoadSchedules()
	assert.NoError(t, err)
	assert.Empty(t, schedules)
}

func TestLoadSchedules(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	input := []Schedule{
		{ID: "a1b2", Name: "Test Schedule", Prompt: "do stuff", CronExpr: "0 9 * * *", ProjectPath: "/tmp", Program: "claude", Enabled: true, CreatedAt: now},
	}
	setupTestSchedules(t, input)

	schedules, err := LoadSchedules()
	assert.NoError(t, err)
	require.Len(t, schedules, 1)
	assert.Equal(t, "a1b2", schedules[0].ID)
	assert.Equal(t, "Test Schedule", schedules[0].Name)
	assert.Equal(t, "do stuff", schedules[0].Prompt)
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	setupTestSchedules(t, []Schedule{})

	s := Schedule{
		ID:          "abc1",
		Name:        "Nightly Build",
		Prompt:      "run build",
		CronExpr:    "0 0 * * *",
		ProjectPath: "/tmp/repo",
		Program:     "claude",
		Enabled:     true,
		CreatedAt:   time.Now().Truncate(time.Second),
	}

	err := SaveSchedules([]Schedule{s})
	require.NoError(t, err)

	loaded, err := LoadSchedules()
	assert.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, s.ID, loaded[0].ID)
	assert.Equal(t, s.Name, loaded[0].Name)
	assert.Equal(t, s.CronExpr, loaded[0].CronExpr)
}

func TestAddSchedule(t *testing.T) {
	setupTestSchedules(t, []Schedule{})

	s1 := Schedule{ID: "s1", Name: "First", Prompt: "p1", CronExpr: "0 * * * *", ProjectPath: "/tmp", Program: "claude", Enabled: true, CreatedAt: time.Now()}
	s2 := Schedule{ID: "s2", Name: "Second", Prompt: "p2", CronExpr: "0 0 * * *", ProjectPath: "/tmp", Program: "claude", Enabled: true, CreatedAt: time.Now()}

	require.NoError(t, AddSchedule(s1))
	require.NoError(t, AddSchedule(s2))

	loaded, err := LoadSchedules()
	assert.NoError(t, err)
	assert.Len(t, loaded, 2)
	assert.Equal(t, "s1", loaded[0].ID)
	assert.Equal(t, "s2", loaded[1].ID)
}

func TestRemoveSchedule(t *testing.T) {
	schedules := []Schedule{
		{ID: "keep", Name: "Keep", Prompt: "p", CronExpr: "0 * * * *", ProjectPath: "/tmp", Program: "claude", Enabled: true},
		{ID: "remove", Name: "Remove", Prompt: "p", CronExpr: "0 * * * *", ProjectPath: "/tmp", Program: "claude", Enabled: true},
	}
	setupTestSchedules(t, schedules)

	err := RemoveSchedule("remove")
	assert.NoError(t, err)

	loaded, err := LoadSchedules()
	assert.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "keep", loaded[0].ID)
}

func TestRemoveScheduleNotFound(t *testing.T) {
	setupTestSchedules(t, []Schedule{{ID: "exists"}})

	err := RemoveSchedule("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetSchedule(t *testing.T) {
	schedules := []Schedule{
		{ID: "x1", Name: "First"},
		{ID: "x2", Name: "Second"},
	}
	setupTestSchedules(t, schedules)

	s, err := GetSchedule("x2")
	assert.NoError(t, err)
	assert.Equal(t, "Second", s.Name)
}

func TestGetScheduleNotFound(t *testing.T) {
	setupTestSchedules(t, []Schedule{})

	_, err := GetSchedule("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateSchedule(t *testing.T) {
	schedules := []Schedule{
		{ID: "u1", Name: "Old Name", Prompt: "old prompt", CronExpr: "0 * * * *", Enabled: true},
	}
	setupTestSchedules(t, schedules)

	updated := Schedule{ID: "u1", Name: "New Name", Prompt: "new prompt", CronExpr: "0 0 * * *", Enabled: false}
	err := UpdateSchedule(updated)
	assert.NoError(t, err)

	s, err := GetSchedule("u1")
	assert.NoError(t, err)
	assert.Equal(t, "New Name", s.Name)
	assert.Equal(t, "new prompt", s.Prompt)
	assert.Equal(t, "0 0 * * *", s.CronExpr)
	assert.False(t, s.Enabled)
}

func TestUpdateScheduleNotFound(t *testing.T) {
	setupTestSchedules(t, []Schedule{})

	err := UpdateSchedule(Schedule{ID: "missing"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGenerateID(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()
	assert.Len(t, id1, 8) // 4 bytes = 8 hex chars
	assert.Len(t, id2, 8)
	assert.NotEqual(t, id1, id2)
}

func TestScheduleNameInJSON(t *testing.T) {
	s := Schedule{ID: "n1", Name: "My Schedule", Prompt: "do things"}
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"name":"My Schedule"`)

	// Name omitted when empty
	s2 := Schedule{ID: "n2", Prompt: "do things"}
	data2, err := json.Marshal(s2)
	require.NoError(t, err)
	assert.NotContains(t, string(data2), `"name"`)
}
