package mpal

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	strategyfiles "github.com/revrost/mpal-cli/strategies"
)

type StrategyInfo struct {
	ID                  string           `json:"id"`
	Version             string           `json:"version"`
	Name                string           `json:"name,omitempty"`
	Description         string           `json:"description,omitempty"`
	WhenToUse           string           `json:"when_to_use,omitempty"`
	Cadence             string           `json:"cadence,omitempty"`
	Approved            bool             `json:"approved"`
	Source              string           `json:"source"`
	Path                string           `json:"path"`
	ConfigHash          string           `json:"config_hash"`
	ConfigHashAlgorithm string           `json:"config_hash_algorithm"`
	ScoringContract     string           `json:"scoring_contract"`
	APIContract         string           `json:"api_contract"`
	APICompatible       bool             `json:"api_compatible"`
	APICompatibility    ValidationResult `json:"api_compatibility"`
	Validation          ValidationResult `json:"validation"`
}

type StrategyRegistry struct {
	UserDir string
}

func DefaultStrategyRegistry() StrategyRegistry {
	home, _ := os.UserHomeDir()
	return StrategyRegistry{
		UserDir: filepath.Join(home, ".marketpal", "strategies"),
	}
}

func (r StrategyRegistry) List() ([]StrategyInfo, error) {
	infos, err := listEmbeddedStrategies()
	if err != nil {
		return nil, err
	}
	userInfos, err := listStrategyDir("user", r.UserDir)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]StrategyInfo, len(infos)+len(userInfos))
	for _, info := range infos {
		byID[strings.ToLower(info.ID)] = info
	}
	for _, info := range userInfos {
		// User strategies intentionally override embedded strategies with the
		// same id, which keeps `strategy list` readable while preserving local
		// customization.
		byID[strings.ToLower(info.ID)] = info
	}
	infos = make([]StrategyInfo, 0, len(byID))
	for _, info := range byID {
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].ID == infos[j].ID {
			return infos[i].Source < infos[j].Source
		}
		return infos[i].ID < infos[j].ID
	})
	return infos, nil
}

func listEmbeddedStrategies() ([]StrategyInfo, error) {
	matches, err := fs.Glob(strategyfiles.FS, "*.yaml")
	if err != nil {
		return nil, err
	}
	infos := make([]StrategyInfo, 0, len(matches))
	for _, path := range matches {
		raw, err := strategyfiles.FS.ReadFile(path)
		if err != nil {
			return nil, err
		}
		cfg, hash, validation := strategyInfoFromBytes(raw)
		apiCompatibility := ValidateHostedStrategyAPICompatibility(cfg)
		infos = append(infos, StrategyInfo{
			ID:                  cfg.ID,
			Version:             cfg.Version,
			Name:                cfg.Name,
			Description:         cfg.Description,
			WhenToUse:           cfg.WhenToUse,
			Cadence:             cfg.Cadence,
			Approved:            cfg.Approved,
			Source:              "builtin",
			Path:                "embedded:strategies/" + path,
			ConfigHash:          hash,
			ConfigHashAlgorithm: StrategyConfigHashAlgorithm,
			ScoringContract:     StrategyScoringContract(cfg),
			APIContract:         HostedStrategyAPIContract,
			APICompatible:       apiCompatibility.Valid,
			APICompatibility:    apiCompatibility,
			Validation:          validation,
		})
	}
	return infos, nil
}

func listStrategyDir(source, dir string) ([]StrategyInfo, error) {
	if dir == "" {
		return nil, nil
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	infos := make([]StrategyInfo, 0, len(matches))
	for _, path := range matches {
		cfg, hash, err := LoadStrategyFile(path)
		validation := ValidationResult{Valid: err == nil}
		if err != nil {
			validation.Errors = []string{err.Error()}
		} else {
			validation = ValidateStrategyConfig(cfg)
		}
		apiCompatibility := ValidateHostedStrategyAPICompatibility(cfg)
		infos = append(infos, StrategyInfo{
			ID:                  cfg.ID,
			Version:             cfg.Version,
			Name:                cfg.Name,
			Description:         cfg.Description,
			WhenToUse:           cfg.WhenToUse,
			Cadence:             cfg.Cadence,
			Approved:            cfg.Approved,
			Source:              source,
			Path:                path,
			ConfigHash:          hash,
			ConfigHashAlgorithm: StrategyConfigHashAlgorithm,
			ScoringContract:     StrategyScoringContract(cfg),
			APIContract:         HostedStrategyAPIContract,
			APICompatible:       apiCompatibility.Valid,
			APICompatibility:    apiCompatibility,
			Validation:          validation,
		})
	}
	return infos, nil
}

func (r StrategyRegistry) Show(id string) (StrategyInfo, StrategyConfig, error) {
	infos, err := r.List()
	if err != nil {
		return StrategyInfo{}, StrategyConfig{}, err
	}
	for _, info := range infos {
		if strings.EqualFold(info.ID, id) {
			if strings.HasPrefix(info.Path, "embedded:") {
				cfg, _, err := loadEmbeddedStrategy(strings.TrimPrefix(info.Path, "embedded:"))
				return info, cfg, err
			}
			cfg, _, err := LoadStrategyFile(info.Path)
			return info, cfg, err
		}
	}
	return StrategyInfo{}, StrategyConfig{}, fmt.Errorf("strategy %q not found", id)
}

func loadEmbeddedStrategy(path string) (StrategyConfig, string, error) {
	path = strings.TrimPrefix(path, "strategies/")
	raw, err := strategyfiles.FS.ReadFile(path)
	if err != nil {
		return StrategyConfig{}, "", err
	}
	return LoadStrategyBytes(raw)
}

func strategyInfoFromBytes(raw []byte) (StrategyConfig, string, ValidationResult) {
	cfg, hash, err := LoadStrategyBytes(raw)
	if err != nil {
		return cfg, hash, ValidationResult{Valid: false, Errors: []string{err.Error()}}
	}
	return cfg, hash, ValidateStrategyConfig(cfg)
}

func StrategyRefFromInfo(info StrategyInfo) StrategyRef {
	return StrategyRef{
		ID:         info.ID,
		Version:    info.Version,
		ConfigHash: info.ConfigHash,
		Approved:   info.Approved,
		Source:     info.Source,
		Path:       info.Path,
	}
}
