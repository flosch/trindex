package trindex

// TODO:
// - Support both case-sensitive and case-insensitive
// - Support preprocessing of inputs (like removing special chars, renaming umlauts (ö -> oe), etc.)
// - 2 storage engines (provide an Sample() method to detect whether to use the short or the large storage
// [large storage will be used when avg(size(sample)) >= 500 bytes and uses a bitmap; short storage uses list of 8 bytes
// integer items]

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
)

const (
	writeCacheSize             = 250000
	cacheDocIDSize             = 1e7
	cacheSize                  = 1024 * 1024 * 512 // 512 MiB, given in byte
	sampleTresholdCount        = 50
	sampleTresholdLargeStorage = 500 // in bytes, we change to a bitmap storage for large files (= huge amount of trigrams)
)

type storageType int

const (
	storageShort storageType = iota
	storageLong
)

type Index struct {
	filename     string
	sample_count int
	sample_size  int

	item_id      uint64
	item_db_lock sync.Mutex
	item_db      *os.File
	cache        map[uint64]uint32
	write_cache  []uint64

	storageEngine storageType
	storage       IStorage
}

type Result struct {
	ID         uint64
	Similarity float64
}

type ResultSet []*Result

func (r *Result) String() string {
	return fmt.Sprintf("<Result ID=%d Similarity=%.2f>", r.ID, r.Similarity)
}

func (rs ResultSet) Len() int {
	return len(rs)
}

func (rs ResultSet) Less(i, j int) bool {
	return rs[i].Similarity >= rs[j].Similarity
}

func (rs ResultSet) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

func NewIndex(filename string) *Index {
	idx := &Index{
		filename:    filename,
		storage:     newListStorage(filename),
		cache:       make(map[uint64]uint32),
		write_cache: make([]uint64, 0, writeCacheSize),
	}

	fd, err := os.OpenFile(fmt.Sprintf("%s.docs", filename), os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}
	idx.item_db = fd

	offset, err := fd.Seek(0, 2)
	if err != nil {
		panic(err)
	}
	if offset == 0 {
		// Write init item
		err = binary.Write(fd, binary.LittleEndian, uint64(0))
		if err != nil {
			panic(err)
		}
	} else {
		// Read last item id
		_, err = fd.Seek(0, 0)
		if err != nil {
			panic(err)
		}
		err = binary.Read(fd, binary.LittleEndian, &idx.item_id)
		if err != nil {
			panic(err)
		}

		// Read all IDs into cache
		var tmp_id uint32
		for i := uint64(0); i < idx.item_id; i++ {
			err = binary.Read(fd, binary.LittleEndian, &tmp_id)
			if err != nil {
				panic(err)
			}
			idx.cache[uint64(i+1)] = tmp_id
		}
	}

	return idx
}

// It's important to close the index to flush all writes to disk.
func (idx *Index) Close() {
	idx.storage.Close()

	idx.item_db_lock.Lock()
	_, err := idx.item_db.Seek(0, 2)
	if err != nil {
		panic(err)
	}
	err = binary.Write(idx.item_db, binary.LittleEndian, uint64(0))
	if err != nil {
		panic(err)
	}
	idx.item_db_lock.Unlock()

	idx.flushWriteCache()

	if err := idx.item_db.Close(); err != nil {
		panic(err)
	}
}

// Inserts a document to the index. It is safe for concurrent use.
func (idx *Index) Insert(data string) uint64 {
	new_doc_id := atomic.AddUint64(&idx.item_id, 1)

	trigrams := trigramize(data)

	for _, trigram := range trigrams {
		idx.storage.AddItem(trigram, new_doc_id)
	}

	if len(idx.cache) > cacheDocIDSize {
		counter := 0
		treshold := int(cacheDocIDSize / 4)

		// Flush anything to disk before we delete something from cache
		idx.flushWriteCache()

		for doc_id, _ := range idx.cache {
			counter++
			delete(idx.cache, doc_id)

			if counter >= treshold {
				break
			}
		}
	}

	if len(idx.write_cache) >= writeCacheSize {
		idx.flushWriteCache()
	}

	idx.cache[new_doc_id] = uint32(len(trigrams))

	idx.item_db_lock.Lock()
	defer idx.item_db_lock.Unlock()

	idx.write_cache = append(idx.write_cache, new_doc_id)

	return new_doc_id
}

func (idx *Index) getTotalTrigrams(doc_id uint64) int {
	idx.item_db_lock.Lock()
	defer idx.item_db_lock.Unlock()

	count, has := idx.cache[doc_id]
	if has {
		return int(count)
	}

	_, err := idx.item_db.Seek(int64(doc_id*8), 0)
	if err != nil {
		panic(err)
	}
	var rtv uint32
	err = binary.Read(idx.item_db, binary.LittleEndian, &rtv)
	if err != nil {
		panic(err)
	}
	idx.cache[doc_id] = rtv

	// TODO: Put an cache invalidator here

	return int(rtv)
}

func (idx *Index) flushWriteCache() {
	idx.item_db_lock.Lock()
	defer idx.item_db_lock.Unlock()

	// write_cache
	for _, doc_id := range idx.write_cache {
		_, err := idx.item_db.Seek(int64(doc_id*8), 0)
		if err != nil {
			panic(err)
		}
		err = binary.Write(idx.item_db, binary.LittleEndian, idx.cache[doc_id])
		if err != nil {
			panic(err)
		}
	}

	idx.write_cache = make([]uint64, 0, writeCacheSize)
}

func (idx *Index) Query(query string, max_items int) ResultSet {
	trigrams := trigramize(query)
	trigrams_len := float64(len(trigrams))

	temp_map := make(map[uint64]int)

	for _, trigram := range trigrams {
		doc_ids := idx.storage.GetItems(trigram)

		// 1 document only once per trigram
		once_map := make(map[uint64]bool)

		for _, id := range doc_ids {
			_, has := once_map[id]
			if has {
				continue
			}
			once_map[id] = true

			c, has := temp_map[id]
			if has {
				temp_map[id] = c + 1
			} else {
				temp_map[id] = 1
			}
		}
	}

	lowest_similarity := 0.0
	result_set := make(ResultSet, 0, max_items+1)
	for id, count := range temp_map {
		if len(result_set) < max_items {
			x := trigrams_len / float64(idx.getTotalTrigrams(id))
			if x > 1 {
				x = 1.0 / x
			}
			result_set = append(result_set, &Result{
				ID:         id,
				Similarity: (float64(count) / trigrams_len) - (1.0 - x),
			})

			sort.Sort(result_set)
			lowest_similarity = result_set[len(result_set)-1].Similarity
			continue
		}

		x := trigrams_len / float64(idx.getTotalTrigrams(id))
		if x > 1 {
			x = 1.0 / x
		}
		s := (float64(count) / trigrams_len) - (1.0 - x)

		if s < lowest_similarity {
			continue
		}

		result_set = append(result_set, &Result{
			ID:         id,
			Similarity: s,
		})

		sort.Sort(result_set)
		result_set = result_set[:min(max_items, len(result_set))]
		lowest_similarity = result_set[len(result_set)-1].Similarity
	}

	if len(result_set) > 0 {
		return result_set[:min(len(temp_map), max_items)]
	}

	return nil
}
