package interf

type IDB interface {
	New(args ...string) (IDB, error)
	Close()
	Get(key string) (map[string]string, error)
	Put(key, value string) error
	Delete(key string) error
}
