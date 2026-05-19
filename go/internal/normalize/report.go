package normalize

type Report struct {
	Warnings         []string
	DeprecatedFields []string
	AppliedRules     []string
	Notes            []string
}

func (r *Report) AddWarning(value string) {
	if value != "" {
		r.Warnings = append(r.Warnings, value)
	}
}

func (r *Report) AddDeprecatedField(value string) {
	if value != "" {
		r.DeprecatedFields = append(r.DeprecatedFields, value)
	}
}

func (r *Report) AddAppliedRule(value string) {
	if value != "" {
		r.AppliedRules = append(r.AppliedRules, value)
	}
}

func (r *Report) AddNote(value string) {
	if value != "" {
		r.Notes = append(r.Notes, value)
	}
}
