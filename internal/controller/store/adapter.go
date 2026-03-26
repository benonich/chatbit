package store

const (
	PrefixRoom    = uint16(0)
	PrefixMessage = uint16(1)
	PrefixAlias   = uint16(2)
)

type Adapter struct {
	Room    DBInternal
	Message MessageInternal
}

func NewAdapter(db Database) *Adapter {
	return &Adapter{
		Room:    NewAdapterInternal(db, PrefixRoom),
		Message: NewMessageAdapterInternal(db, PrefixMessage),
	}
}
