package cache

type Redis struct{}

func NewRedis() *Redis {
	return &Redis{}
}
