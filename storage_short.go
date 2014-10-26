package trindex

import (
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"bufio"
	"os"
	"sync"

	"github.com/flosch/cache"
)

const (
	slotSize = 7500
)

type listStorage struct {
	filename        string
	filename_header string
	db              *os.File

	header *header

	trigrams_lock        sync.Mutex
	trigrams_items_cache *cache.Cache // trigram -> []document_ids
}

type trigram_index struct {
	Position uint64
	Items    uint64
}

type header struct {
	Compaction_needed bool
	Trigrams          map[string][]*trigram_index
}

func newListStorage(filename string) *listStorage {
	storage := &listStorage{
		filename:        filename,
		filename_header: fmt.Sprintf("%s.hdr", filename),
	}

	// Create LRU cache for trigrams
	storage.trigrams_items_cache = cache.New(cache.Config{MaxBytes: cacheSize})

	// Open the database for append
	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	storage.db = fd

	// Read the header now or create an empty one
	storage.readHeaderFile()

	return storage
}

func (storage *listStorage) writeHeaderFile() {
	fd, err := os.OpenFile(storage.filename_header, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	enc := gob.NewEncoder(fd)
	err = enc.Encode(storage.header)
	if err != nil {
		panic(err)
	}
}

func (storage *listStorage) readHeaderFile() {
	fd, err := os.OpenFile(storage.filename_header, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	dec := gob.NewDecoder(fd)
	err = dec.Decode(&storage.header)
	if err != nil {
		// Create new header
		storage.header = &header{
			Trigrams: make(map[string][]*trigram_index),
		}
	}
}

func (storage *listStorage) AddItem(trigram string, doc_id uint64) {
	storage.trigrams_lock.Lock()
	defer storage.trigrams_lock.Unlock()

	// Invalidate the cache item, if there's any
	storage.trigrams_items_cache.Remove(trigram)

	// Check for trigram header item
	idx, has := storage.header.Trigrams[trigram]
	if !has {
		// Create a new trigram header and the data file
		new_index := make([]*trigram_index, 0, 1)
		storage.header.Trigrams[trigram] = new_index
		offset, err := storage.db.Seek(0, 2)
		if err != nil {
			panic(err)
		}
		storage.header.Trigrams[trigram] = append(new_index, &trigram_index{
			Position: uint64(offset),
			Items:    1,
		})

		// Write the item to disk
		err = binary.Write(storage.db, binary.LittleEndian, doc_id)
		if err != nil {
			panic(err)
		}

		_, err = storage.db.Seek((slotSize-1)*8-1, 1)
		if err != nil {
			panic(err)
		}
		_, err = storage.db.Write([]byte{0})
		if err != nil {
			panic(err)
		}
	} else {
		// Already got an entry in the file
		last_idx := idx[len(idx)-1]

		if last_idx.Items >= slotSize {
			// This slot is full, create new one
			offset, err := storage.db.Seek(0, 2)
			if err != nil {
				panic(err)
			}
			storage.header.Trigrams[trigram] = append(storage.header.Trigrams[trigram], &trigram_index{
				Position: uint64(offset),
				Items:    1,
			})

			// Write the item to disk
			err = binary.Write(storage.db, binary.LittleEndian, doc_id)
			if err != nil {
				panic(err)
			}

			_, err = storage.db.Seek((slotSize-1)*8-1, 1)
			if err != nil {
				panic(err)
			}
			_, err = storage.db.Write([]byte{0})
			if err != nil {
				panic(err)
			}
		} else {
			last_idx.Items++
			_, err := storage.db.Seek(int64(last_idx.Position+last_idx.Items*8), 0)
			if err != nil {
				panic(err)
			}
			err = binary.Write(storage.db, binary.LittleEndian, doc_id)
			if err != nil {
				panic(err)
			}
		}
	}
}

func (storage *listStorage) GetItems(trigram string) []uint64 {
	storage.trigrams_lock.Lock()
	defer storage.trigrams_lock.Unlock()

	list, has := storage.trigrams_items_cache.Get(trigram)
	if has {
		return list.([]uint64)
	}

	//fmt.Println("not in cache, loading from disk: ", trigram)

	indexes, has := storage.header.Trigrams[trigram]
	if !has {
		return nil
	}

	total_items := uint64(0)
	for _, idx := range indexes {
		total_items += idx.Items
	}

	result := make([]uint64, 0, total_items)

	// Read all items
	var tmp_id uint64
	var err error
	for _, idx := range indexes {
		storage.db.Seek(int64(idx.Position), 0)
		r := bufio.NewReader(storage.db)
		for i := uint64(0); i < idx.Items; i++ {
			err = binary.Read(r, binary.LittleEndian, &tmp_id)
			if err != nil {
				panic(err)
			}
			result = append(result, tmp_id)
		}
	}

	// Cache it
	storage.trigrams_items_cache.Set(trigram, result, int64(total_items*8))

	return result
}

func (storage *listStorage) Close() {
	storage.trigrams_lock.Lock()
	defer storage.trigrams_lock.Unlock()
	err := storage.db.Sync()
	if err != nil {
		panic(err)
	}
	storage.writeHeaderFile()
	if err := storage.db.Close(); err != nil {
		panic(err)
	}
}
