package trindex

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSimple(t *testing.T) {
	rand.Seed(time.Now().Unix())
	idx := NewIndex(filepath.Join(os.TempDir(), fmt.Sprintf("trindex_test_%d.idx", rand.Int63())))
	defer idx.Close()

	dataset := []string{
		"Mallorca", "Ibiza", "Menorca", "Pityusen", "Formentera", "Berlin", "New York", "Yorkshire",
	}

	for _, data := range dataset {
		idx.Insert(data)
	}

	queries := []string{
		"malorka", "ibza", "enorc", "yusen", "formtera", "bärlihn", "newyorc", "yorkshir",
	}

	for i, qry := range queries {
		rs := idx.Query(qry, 1, 0)
		if len(rs) < 1 {
			t.Fatalf("len(rs) != 1, instead %d", len(rs))
		}
		if rs[0].ID != uint64(i+1) {
			t.Log(rs)
			t.Fatalf("rs[0].ID != idx, instead %d", rs[0].ID)
		}
	}
}
