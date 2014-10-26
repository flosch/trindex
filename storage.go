package trindex

type IStorage interface {
	AddItem(trigram string, doc_id uint64)
	GetItems(trigram string) []uint64
	Close()
}
