package uuid

type Generator interface {
	New() string
}
