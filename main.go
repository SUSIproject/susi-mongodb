package main

import (
	"flag"
	"log"

	"gopkg.in/mgo.v2/bson"

	"github.com/webvariants/susigo"
)

var mongoEndpoint = flag.String("db", "localhost", "mongodb endpoint")
var susiAddr = flag.String("susi-addr", "localhost:4000", "susi server address")
var cert = flag.String("cert", "cert.crt", "certificate to use")
var key = flag.String("key", "key.key", "key to use")

func addError(err string, event *susigo.Event) {
	event.Headers = append(event.Headers, map[string]string{"Error": err})
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
	susi, err := susigo.NewSusi(*susiAddr, *cert, *key)
	if err != nil {
		log.Fatal(err)
	}
	mongo := NewMongo(*mongoEndpoint)

	susi.Wait()

	susi.RegisterProcessor("mongodb::insert", func(event *susigo.Event) {
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			db, ok1 := payload["db"].(string)
			collection, ok2 := payload["collection"].(string)
			doc, ok3 := payload["doc"].(map[string]interface{})
			if ok1 && ok2 && ok3 {
				id := bson.NewObjectId()
				doc["_id"] = id
				mongo.Insert(db, collection, doc)
				payload["id"] = id
			} else {
				addError("need db, collection and doc as parameters", event)
			}
		} else {
			addError("need db, collection and doc as parameters", event)
		}
		susi.Ack(event)
	})

	susi.RegisterProcessor("mongodb::remove", func(event *susigo.Event) {
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			db, ok1 := payload["db"].(string)
			collection, ok2 := payload["collection"].(string)
			id, ok3 := payload["id"].(string)
			if ok1 && ok2 && ok3 && bson.IsObjectIdHex(id) {
				mongo.Remove(db, collection, map[string]interface{}{"_id": bson.ObjectIdHex(id)})
				payload["success"] = true
			} else {
				addError("need db, collection and id as parameters", event)
			}
		} else {
			addError("need db, collection and id as parameters", event)
		}
		susi.Ack(event)
	})

	susi.RegisterProcessor("mongodb::upsert", func(event *susigo.Event) {
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			db, ok1 := payload["db"].(string)
			collection, ok2 := payload["collection"].(string)
			id, ok3 := payload["id"].(string)
			doc, ok4 := payload["doc"].(map[string]interface{})
			if ok1 && ok2 && ok3 && ok4 && bson.IsObjectIdHex(id) {
				mongo.Upsert(db, collection, map[string]interface{}{"_id": bson.ObjectIdHex(id)}, doc)
				payload["success"] = true
			} else {
				addError("need db, collection, id and doc as parameters", event)
			}
		} else {
			addError("need db, collection, id and doc as parameters", event)
		}
		susi.Ack(event)
	})

	susi.RegisterProcessor("mongodb::find", func(event *susigo.Event) {
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			db, ok1 := payload["db"].(string)
			collection, ok2 := payload["collection"].(string)
			query, ok3 := payload["query"].(map[string]interface{})
			if ok1 && ok2 && ok3 {
				iter, err := mongo.Find(db, collection, query)
				if err != nil {
					addError(err.Error(), event)
				} else {
					docs := make([]map[string]interface{}, 0, 32)
					for doc := range iter {
						docs = append(docs, doc)
					}
					payload["docs"] = docs
				}
			} else {
				addError("need db, collection and query as parameters", event)
			}
		} else {
			addError("need db, collection and query as parameters", event)
		}
		susi.Ack(event)
	})

	susi.RegisterProcessor("mongodb::get", func(event *susigo.Event) {
		if payload, ok := event.Payload.(map[string]interface{}); ok {
			db, ok1 := payload["db"].(string)
			collection, ok2 := payload["collection"].(string)
			id, ok3 := payload["id"].(string)
			if ok1 && ok2 && ok3 && bson.IsObjectIdHex(id) {
				iter, err := mongo.Find(db, collection, map[string]interface{}{"_id": bson.ObjectIdHex(id)})
				if err != nil {
					addError(err.Error(), event)
				} else {
					payload["doc"] = <-iter
				}
			} else {
				addError("need db, collection and id as parameters", event)
			}
		} else {
			addError("need db, collection and id as parameters", event)
		}
		susi.Ack(event)
	})

	select {}

}
