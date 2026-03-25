package controller

import (
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

func TestLongVersionCreateDefaults(t *testing.T) {
	req := &apimodel.UpdateLangVersion{
		Lang:    "python",
		Version: "3.11",
		Show:    true,
	}

	version := buildLangVersionForCreate(req)

	if version.BuildStrategy != dbmodel.LongVersionBuildStrategySlug {
		t.Fatalf("expected default build strategy %q, got %q", dbmodel.LongVersionBuildStrategySlug, version.BuildStrategy)
	}
	if !version.IsAllowed {
		t.Fatal("expected create request to default is_allowed to true")
	}
}

func TestUpdateLangVersionKeepsExistingStrategyAndAllowedWhenOmitted(t *testing.T) {
	existing := &dbmodel.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.11",
		BuildStrategy: dbmodel.LongVersionBuildStrategyCNB,
		IsAllowed:     false,
		Show:          true,
		FirstChoice:   false,
	}
	req := &apimodel.UpdateLangVersion{
		Lang:        "python",
		Version:     "3.11",
		Show:        false,
		FirstChoice: true,
	}

	version := buildLangVersionForUpdate(existing, req)

	if version.BuildStrategy != dbmodel.LongVersionBuildStrategyCNB {
		t.Fatalf("expected update to keep existing build strategy %q, got %q", dbmodel.LongVersionBuildStrategyCNB, version.BuildStrategy)
	}
	if version.IsAllowed {
		t.Fatal("expected update to keep existing is_allowed=false when request omits the field")
	}
	if version.Show {
		t.Fatal("expected update to apply new show value")
	}
	if !version.FirstChoice {
		t.Fatal("expected update to apply new first_choice value")
	}
}

func TestUpdateLangVersionHonorsExplicitStrategyAndAllowed(t *testing.T) {
	existing := &dbmodel.EnterpriseLanguageVersion{
		Lang:          "python",
		Version:       "3.11",
		BuildStrategy: dbmodel.LongVersionBuildStrategySlug,
		IsAllowed:     true,
	}
	buildStrategy := dbmodel.LongVersionBuildStrategyCNB
	isAllowed := false
	req := &apimodel.UpdateLangVersion{
		Lang:          "python",
		Version:       "3.11",
		Show:          true,
		FirstChoice:   true,
		BuildStrategy: &buildStrategy,
		IsAllowed:     &isAllowed,
	}

	version := buildLangVersionForUpdate(existing, req)

	if version.BuildStrategy != dbmodel.LongVersionBuildStrategyCNB {
		t.Fatalf("expected update to use explicit build strategy %q, got %q", dbmodel.LongVersionBuildStrategyCNB, version.BuildStrategy)
	}
	if version.IsAllowed {
		t.Fatal("expected update to use explicit is_allowed=false")
	}
}
