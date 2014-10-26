trindex
=======

[![GoDoc](https://godoc.org/github.com/flosch/trindex?status.png)](https://godoc.org/github.com/flosch/trindex)
[![Build Status](https://travis-ci.org/flosch/trindex.svg?branch=master)](https://travis-ci.org/flosch/trindex)
[![gratipay](http://img.shields.io/badge/gratipay-support%20trindex-brightgreen.svg)](https://gratipay.com/flosch/)

trindex is a trigram search library for terms written for and in Go (in alpha stage!). It provides a very simple API
and ships with its own database.

```go
idx := NewIndex("trindex.db")
defer idx.Close()

dataset := []string{
    "Mallorca", "Ibiza", "Menorca", "Pityusen", "Formentera", "Berlin", "New York", "Yorkshire",
}

for _, data := range dataset {
    id := idx.Insert(data)
    // Use ID to connect the term with the associated dataset;
    // for example save the ID in your SQL database about travel destinations
}

results := idx.Query("malorka", 3)

// Returns a sorted list of results including the ID and a confidence number ("Similarity"; 1 = best match) 
```

trindex relies heavily on caching; it's API is safe for concurrent use. Please make sure that you'll call `idx.Close()`
**in any case** on application shutdown (to flush inserted data to disk).