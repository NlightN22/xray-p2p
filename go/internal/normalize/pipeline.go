package normalize

type Pipeline[Raw any, Canonical any] struct {
	Defaults func(*Raw)
	Rules    []Rule[Raw]
	Validate func(Raw) error
	Build    func(Raw) (Canonical, error)
}

func (p Pipeline[Raw, Canonical]) Normalize(raw Raw) (Canonical, Report, error) {
	var report Report
	if p.Defaults != nil {
		p.Defaults(&raw)
	}
	for _, rule := range p.Rules {
		if rule.Apply == nil {
			continue
		}
		if err := rule.Apply(&raw, &report); err != nil {
			var zero Canonical
			return zero, report, err
		}
	}
	if p.Validate != nil {
		if err := p.Validate(raw); err != nil {
			var zero Canonical
			return zero, report, err
		}
	}
	if p.Build == nil {
		var zero Canonical
		return zero, report, nil
	}
	canonical, err := p.Build(raw)
	if err != nil {
		var zero Canonical
		return zero, report, err
	}
	return canonical, report, nil
}
