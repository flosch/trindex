package trindex

// TODO:
// - Support both case-sensitive and case-insensitive
// - Support preprocessing of inputs (like removing special chars, renaming umlauts (ö -> oe), etc.)
// - 2 storage engines (provide an Sample() method to detect whether to use the short or the large storage
// [large storage will be used when avg(size(sample)) >= 500 bytes and uses a bitmap; short storage uses list of 8 bytes
// integer items]

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/petar/GoLLRB/llrb"
)

const (
	writeCacheSize = 250000
	cacheDocIDSize = 1e7
	cacheSize      = 1024 * 1024 * 512 // 512 MiB, given in byte
	//sampleTresholdCount        = 50
	//sampleTresholdLargeStorage = 500 // in bytes, we change to a bitmap storage for large files (= huge amount of trigrams)
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

	cache       map[uint64]uint32
	write_cache []uint64

	storageEngine storageType
	storage       IStorage
}

type Result struct {
	index      int
	ID         uint64
	Similarity float64
}

type ResultSet []*Result

func (r *Result) String() string {
	return fmt.Sprintf("<Result ID=%d Similarity=%.2f>", r.ID, r.Similarity)
}

func (r *Result) Less(other llrb.Item) bool {
	return r.Similarity < other.(*Result).Similarity
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

		// For the time being, read min(item_id, 5000000) IDs into cache.
		// We could skip these step but have it in the library for performance reasons.
		r := bufio.NewReader(fd)
		var tmp_id uint32
		for i := uint64(0); i < uint64(min(int(idx.item_id), 5000000)); i++ {
			err = binary.Read(r, binary.LittleEndian, &tmp_id)
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
	_, err := idx.item_db.Seek(0, 0)
	if err != nil {
		panic(err)
	}
	err = binary.Write(idx.item_db, binary.LittleEndian, idx.item_id)
	if err != nil {
		panic(err)
	}
	idx.item_db_lock.Unlock()

	idx.flushWriteCache()

	if err = idx.item_db.Close(); err != nil {
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

		idx.item_db_lock.Lock()
		for doc_id, _ := range idx.cache {
			counter++
			delete(idx.cache, doc_id)

			if counter >= treshold {
				break
			}
		}
		idx.item_db_lock.Unlock()
	}

	if len(idx.write_cache) >= writeCacheSize {
		idx.flushWriteCache()
	}

	idx.item_db_lock.Lock()
	defer idx.item_db_lock.Unlock()

	idx.cache[new_doc_id] = uint32(len(trigrams))

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

	if doc_id <= 0 {
		panic("doc_id <= 0 not available")
	}

	_, err := idx.item_db.Seek(int64(8+(doc_id-1)*4), 0)
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
		_, err := idx.item_db.Seek(int64(8+(doc_id-1)*4), 0)
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

func (idx *Index) Query(query string, max_results int) ResultSet {
	trigrams := trigramize(query)
	trigrams_len := float64(len(trigrams))

	//stime := time.Now()

	temp_map := make(map[uint64]int)
	for _, trigram := range trigrams {
		doc_ids := idx.storage.GetItems(trigram)

		for _, id := range doc_ids {
			c, has := temp_map[id]
			if has {
				temp_map[id] = c + 1
			} else {
				temp_map[id] = 1
			}
		}
	}

	//etime := time.Now().Sub(stime)
	//fmt.Printf("[%s] Time to get all document IDs per trigram took: %s\n", query, etime)
	//stime = time.Now()

	tree := llrb.New()

	lowest_similarity := 0.0
	for id, count := range temp_map {
		x := trigrams_len / float64(idx.getTotalTrigrams(id))
		if x > 1 {
			x = 1.0 / x
		}
		s := (float64(count) / trigrams_len) - (1.0 - x)

		if tree.Len() > max_results && s < lowest_similarity {
			continue
		}

		tree.InsertNoReplace(&Result{
			ID:         id,
			Similarity: s,
		})

		lowest_similarity = s
	}

	//etime = time.Now().Sub(stime)
	//fmt.Printf("[%s] Time to calculate top X took: %s\n", query, etime)

	if tree.Len() > 0 {
		item_count := min(tree.Len(), max_results)
		result_set := make(ResultSet, 0, item_count)

		tree.DescendLessOrEqual(&Result{Similarity: 1.1}, func(i llrb.Item) bool {
			if len(result_set) < item_count {
				result_set = append(result_set, i.(*Result))
				return true
			}
			return false
		})

		return result_set
	}

	return nil
}
