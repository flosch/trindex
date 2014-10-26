package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"time"

	"github.com/flosch/trindex"
	"github.com/syndtr/goleveldb/leveldb"
)

func main() {
	fmt.Println("trindex demo with Wikidata data")
	fmt.Println("Starting up...")

	var base_dir = flag.String("base_dir", "/data_ssd/world/trindex-wikidata/", "Specifies where the trindex-db is located.")
	flag.Parse()

	trindexNameDB, err := leveldb.OpenFile(filepath.Join(*base_dir, "names/"), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer trindexNameDB.Close()

	idx := trindex.NewIndex(filepath.Join(*base_dir, "trindex.idx"))
	defer idx.Close()

	fmt.Println("Ready. Enter your query ('q' for quit).")

	var query string
	for {
		fmt.Printf("> ")
		_, err := fmt.Scanln(&query)
		if err != nil {
			continue
		}
		if query == "q" {
			break
		}
		fmt.Printf("Searching for '%s'...\n", query)
		stime := time.Now()
		results := idx.Query(query, 10, 0.35)
		etime := time.Now().Sub(stime)
		for idx, item := range results {
			buf, err := trindexNameDB.Get([]byte(strconv.Itoa(int(item.ID))), nil)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("%2d. %50s (%.4f)\n", idx+1, string(buf), item.Similarity)
		}
		fmt.Println()
		fmt.Printf("Query returned in %s.", etime)
		fmt.Println()
	}
	fmt.Println("Exiting...")
}
