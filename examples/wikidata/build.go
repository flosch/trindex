package main

import (
	"os"
	"time"
	"encoding/csv"
	"strconv"
	"log"
	"flag"
	"path/filepath"

	"github.com/flosch/trindex"
	"github.com/syndtr/goleveldb/leveldb"
)

func main() {
	var base_dir = flag.String("base_dir", "/data_ssd/world/trindex-wikidata/", "Specify the default output file.")
	var input_file = flag.String("input_file", "/data_ssd/world/wikidata_export_names.csv", "Specify the input CSV file.")
	var run = flag.Bool("run", false, "Please make this boolean true if you are sure.")
	flag.Parse()

	if *run == false {
		flag.Usage()
		return
	}

	err := os.MkdirAll(*base_dir, 0755)
	if err != nil {
		log.Fatal(err)
	}

	db, err := leveldb.OpenFile(filepath.Join(*base_dir, "names/"), nil)
	defer db.Close()

	idx := trindex.NewIndex(filepath.Join(*base_dir, "trindex.idx"))
	defer idx.Close()

	f, err := os.Open(*input_file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	dec := csv.NewReader(f)

	log.Println("Start building German Wikidata trigram index...")

	stime := time.Now()
	counter := 0
	for {
		record, err := dec.Read()
		if err != nil {
			break
		}

		if record[2] != "de" {
			continue
		}

		counter++

		id := idx.Insert(record[1])
		err = db.Put([]byte(strconv.Itoa(int(id))), []byte(record[1]), nil)
		if err != nil {
			panic(err)
		}

		if counter % 100000 == 0 {
			log.Println(counter)
		}
	}
	etime := time.Now().Sub(stime)
	log.Printf("Inserted %d items in %s (%d items/sec).\n", counter, etime, int(float64(counter)/etime.Seconds()))
}
