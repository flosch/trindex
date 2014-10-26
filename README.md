# trindex

[![GoDoc](https://godoc.org/github.com/flosch/trindex?status.png)](https://godoc.org/github.com/flosch/trindex)
[![Build Status](https://travis-ci.org/flosch/trindex.svg?branch=master)](https://travis-ci.org/flosch/trindex)
[![gratipay](http://img.shields.io/badge/gratipay-support%20trindex-brightgreen.svg)](https://gratipay.com/flosch/)

trindex is a trigram search library for terms written for and in Go (in alpha stage!). It provides a very simple API
and ships with its own database.

I put up a demo page online using trindex. I indexed all German wikidata lemmas (4064962 titles in total) and
made them available for search: https://www.florian-schlachter.de/trindex/

The Wikidata example (build & query) is in the repository: [`examples/wikidata`](https://github.com/flosch/trindex/tree/master/examples/wikidata).

```go
idx := trindex.NewIndex("trindex.db")
defer idx.Close()

dataset := []string{
    "Mallorca", "Ibiza", "Menorca", "Pityusen", "Formentera", 
    "Berlin", "New York", "Yorkshire",
}

for _, data := range dataset {
    id := idx.Insert(data)
    // Use ID to connect the term with the associated dataset;
    // for example save the ID in your SQL database about travel destinations
}

results := idx.Query("malorka", 3)

// Returns a sorted list of 3 results including the ID and
// a confidence number ("Similarity"; 1 = best match) 
```

trindex relies heavily on caching; it's API is safe for concurrent use. Please make sure that you'll call `idx.Close()`
**in any case** on application shutdown (to flush inserted data to disk).

## Related blog posts

 * [trindex: A trigram search library for Go](https://www.florian-schlachter.de/post/trindex/) [26th Oct 2014]
