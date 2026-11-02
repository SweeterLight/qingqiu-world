package dops

import (
	"errors"
	"testing"

	"qingqiu-world-server/internal/database"
	"qingqiu-world-server/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAgentAssignmentRequiresCurrentPassingProbe(t *testing.T) {
	previous := database.DB
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/probe.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = db
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close(); database.DB = previous })
	if err := db.AutoMigrate(&model.LLMConfig{}, &model.Person{}, &model.AgentConfig{}); err != nil {
		t.Fatal(err)
	}

	config := model.LLMConfig{Name: "working", ModelID: "model", BaseURL: "http://example.test", APIKey: "key", ProbeStatus: "unverified"}
	config.ProbeConfigHash = config.ProbeFingerprint()
	if err := db.Create(&config).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := CreateAIPerson("blocked", "", "", config.ID, ""); !errors.Is(err, ErrLLMProbeRequired) {
		t.Fatalf("unverified assignment: %v", err)
	}
	if err := db.Model(&config).Update("probe_status", "passed").Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := CreateAIPerson("allowed", "", "", config.ID, ""); err != nil {
		t.Fatalf("passing assignment: %v", err)
	}
	person, err := GetPersonByName("allowed")
	if err != nil {
		t.Fatal(err)
	}

	other := model.LLMConfig{Name: "other", ModelID: "model", BaseURL: "http://example.test", APIKey: "key", ProbeStatus: "failed"}
	other.ProbeConfigHash = other.ProbeFingerprint()
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := UpdateAIPerson(&AIPersonUpdates{PersonID: person.ID, LLMConfigID: &other.ID}); !errors.Is(err, ErrLLMProbeRequired) {
		t.Fatalf("failed probe assignment: %v", err)
	}
	if err := db.Model(&config).Update("api_key", "changed").Error; err != nil {
		t.Fatal(err)
	}
	if err := RequirePassingLLMProbe(config.ID); !errors.Is(err, ErrLLMProbeRequired) {
		t.Fatalf("stale probe: %v", err)
	}
	if err := UpdateAIPerson(&AIPersonUpdates{PersonID: person.ID, LLMConfigID: &config.ID}); err != nil {
		t.Fatalf("existing assignment should remain: %v", err)
	}
	if err := db.Model(&other).Updates(map[string]interface{}{"probe_status": "passed", "probe_config_hash": other.ProbeFingerprint()}).Error; err != nil {
		t.Fatal(err)
	}
	if err := UpdateAIPerson(&AIPersonUpdates{PersonID: person.ID, LLMConfigID: &other.ID}); err != nil {
		t.Fatalf("retested assignment: %v", err)
	}
}
