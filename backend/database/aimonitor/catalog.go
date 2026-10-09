package aimonitor

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
)

//go:embed reference_catalog.json
var referenceCatalog []byte

func ReferenceCatalog() json.RawMessage { return append(json.RawMessage(nil), referenceCatalog...) }

type Catalog struct {
	Source              string         `json:"source"`
	SourceURL           string         `json:"source_url"`
	Version             string         `json:"version"`
	AutomaticActivation bool           `json:"automatic_activation"`
	Entries             []CatalogEntry `json:"entries"`
}
type CatalogEntry struct {
	Provider             string  `json:"provider"`
	Model                string  `json:"model"`
	ProcessingMode       string  `json:"processing_mode"`
	ContextTier          string  `json:"context_tier"`
	InputPerMillion      *string `json:"input_per_million"`
	OutputPerMillion     *string `json:"output_per_million"`
	CacheReadPerMillion  *string `json:"cache_read_per_million"`
	CacheWritePerMillion *string `json:"cache_write_per_million"`
}

func resolveSelection(rule PricingRule) (PricingRule, error) {
	if rule.ReferenceModel == "" {
		return rule, nil
	}
	var catalog Catalog
	if err := json.Unmarshal(referenceCatalog, &catalog); err != nil {
		return rule, err
	}
	for _, entry := range catalog.Entries {
		if entry.Provider != rule.Provider || entry.Model != rule.ReferenceModel || entry.ProcessingMode != rule.ProcessingMode || entry.ContextTier != rule.ReferenceTier {
			continue
		}
		parse := func(raw *string) (*float64, error) {
			if raw == nil {
				return nil, nil
			}
			v, err := strconv.ParseFloat(*raw, 64)
			if err != nil {
				return nil, err
			}
			return &v, nil
		}
		var err error
		if rule.InputPerMillion, err = parse(entry.InputPerMillion); err != nil {
			return rule, err
		}
		if rule.OutputPerMillion, err = parse(entry.OutputPerMillion); err != nil {
			return rule, err
		}
		if rule.CacheReadPerMillion, err = parse(entry.CacheReadPerMillion); err != nil {
			return rule, err
		}
		if rule.CacheWritePerMillion, err = parse(entry.CacheWritePerMillion); err != nil {
			return rule, err
		}
		rule.Source = catalog.Source
		rule.SourceURL = catalog.SourceURL
		rule.Version = catalog.Version
		return rule, nil
	}
	return rule, fmt.Errorf("no exact provider/SKU/processing mode/context tier in the informational catalog")
}
