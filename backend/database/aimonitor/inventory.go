package aimonitor

// Inventory is independent of the selected reporting range and filters. It is
// bounded by the same response budget and admin query timeout as the overview.
type Selector struct {
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	BillingMode string `json:"billing_mode"`
}
type Inventory struct {
	Models    []string   `json:"models"`
	Tasks     []string   `json:"tasks"`
	Selectors []Selector `json:"selectors"`
}

func (s *Store) Inventory() (Inventory, error) {
	v := Inventory{Tasks: []string{}, Selectors: []Selector{}}
	var err error
	v.Models, err = s.ModelRegistry()
	if err != nil {
		return v, err
	}
	err = s.DB.Raw(`SELECT task FROM (SELECT task FROM ai_samples UNION SELECT task FROM ai_hourly_rollups) ORDER BY task LIMIT ?`, MaxResponseGroups+1).Scan(&v.Tasks).Error
	if err != nil {
		return v, err
	}
	err = s.DB.Raw(`SELECT provider,model,billing_mode FROM (SELECT provider,model,billing_mode FROM ai_samples UNION SELECT provider,model,billing_mode FROM ai_hourly_rollups UNION SELECT provider,model,billing_mode FROM ai_pricing_rules) ORDER BY provider,model,billing_mode LIMIT ?`, MaxResponseGroups+1).Scan(&v.Selectors).Error
	if len(v.Tasks) > MaxResponseGroups || len(v.Selectors) > MaxResponseGroups || len(v.Models) > MaxResponseGroups {
		return v, ErrQueryLimit
	}
	return v, err
}
