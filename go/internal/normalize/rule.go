package normalize

type Rule[T any] struct {
	Name            string
	Description     string
	DeprecatedSince string
	RemovedSince    string
	RemovalNote     string
	Apply           func(*T, *Report) error
}
