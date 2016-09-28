package main

import (
	"errors"
	"log"
	"time"

	"gopkg.in/mgo.v2"
)

type commandType uint8

const (
	insert commandType = iota
	upsert
	find
	remove
)

type cmd struct {
	Type       commandType
	DB         string
	Collection string
	Args       []interface{}
	Return     chan interface{}
}

type Mongo struct {
	addr      string
	session   *mgo.Session
	commands  chan *cmd
	connected bool
}

func NewMongo(addr string) *Mongo {
	m := &Mongo{addr: addr, commands: make(chan *cmd, 64)}
	go m.backend()
	return m
}

func (p *Mongo) Close() {
	p.session.Close()
}

func (p *Mongo) Insert(db, collection string, doc interface{}) {
	p.commands <- &cmd{insert, db, collection, []interface{}{doc}, nil}
}

func (p *Mongo) Upsert(db, collection string, query, doc interface{}) {
	p.commands <- &cmd{upsert, db, collection, []interface{}{query, doc}, nil}
}

func (p *Mongo) Remove(db, collection string, query interface{}) {
	p.commands <- &cmd{remove, db, collection, []interface{}{query}, nil}
}

func (p *Mongo) Find(db, collection string, query interface{}) (chan map[string]interface{}, error) {
	if !p.connected {
		return nil, errors.New("db not connected")
	}
	ret := make(chan interface{})
	p.commands <- &cmd{find, db, collection, []interface{}{query}, ret}
	res := <-ret
	if err, ok := res.(error); ok {
		return nil, err
	}
	return res.(chan map[string]interface{}), nil
}

func (p *Mongo) backend() {
	for {
		s, err := mgo.Dial(p.addr)
		if err != nil {
			log.Print("error connecting mongodb: ", err)
			time.Sleep(1 * time.Second)
			continue
		}
		p.session = s
		p.connected = true
		p.commandLoop()
		p.connected = false
	}
}

func (p *Mongo) commandLoop() {
	for command := range p.commands {
		switch command.Type {
		case insert:
			{
				if err := p.insert(command); err != nil {
					return
				}
			}
		case upsert:
			{
				if err := p.upsert(command); err != nil {
					return
				}
			}
		case remove:
			{
				if err := p.remove(command); err != nil {
					return
				}
			}
		case find:
			{
				if err := p.find(command); err != nil {
					return
				}
			}
		}
	}
}

func (p *Mongo) insert(command *cmd) error {
	c := p.session.DB(command.DB).C(command.Collection)
	if err := c.Insert(command.Args[0]); err != nil {
		p.commands <- command
		return err
	}
	return nil
}

func (p *Mongo) upsert(command *cmd) error {
	c := p.session.DB(command.DB).C(command.Collection)
	if _, err := c.Upsert(command.Args[0], command.Args[1]); err != nil {
		p.commands <- command
		return err
	}
	return nil
}

func (p *Mongo) remove(command *cmd) error {
	c := p.session.DB(command.DB).C(command.Collection)
	if err := c.Remove(command.Args[0]); err != nil {
		p.commands <- command
		return err
	}
	return nil
}

func (p *Mongo) find(command *cmd) error {
	c := p.session.DB(command.DB).C(command.Collection)
	iter := c.Find(command.Args[0]).Iter()
	if err := iter.Err(); err != nil {
		command.Return <- err
	} else {
		result := make(chan map[string]interface{}, 64)
		go func() {
			obj := make(map[string]interface{})
			for iter.Next(&obj) {
				result <- obj
				obj = make(map[string]interface{})
			}
			close(result)
		}()
		command.Return <- result
	}
	return iter.Err()
}
