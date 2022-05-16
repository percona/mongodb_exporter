package exporter

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"sync"
)

type LinkList[T any] struct {
	Pre  *LinkList[T]
	Next *LinkList[T]
	Val  T
}

type MongoClientAndURL struct {
	URL    string
	Client *mongo.Client
}

func (l *LinkList[T]) MoveToFront(originalFront *LinkList[T]) {
	if l.Pre != nil {
		l.Pre.Next = l.Next
	}
	l.Pre = originalFront.Pre
	l.Next = originalFront
}

type MongoClientLRUPool struct {
	limit int
	front *LinkList[MongoClientAndURL]
	last  *LinkList[MongoClientAndURL]
	data  map[string]*LinkList[MongoClientAndURL]
	mux   sync.Mutex
}

func NewMongoClientLRUPool(limit int) *MongoClientLRUPool {
	return &MongoClientLRUPool{
		limit: limit,
		data:  make(map[string]*LinkList[MongoClientAndURL]),
	}
}

func (p *MongoClientLRUPool) refreshLast() {
	if p.front == nil {
		return
	}
	obj := p.front
	for obj.Next != nil {
		obj = obj.Next
	}
	p.last = obj
}

func (p *MongoClientLRUPool) refresh(obj *LinkList[MongoClientAndURL]) {
	if p.last == nil {
		p.refreshLast()
	}
	if obj == nil {
		return
	}
	if p.front == nil {
		p.front = obj
		return
	}
	// normal refresh logic

	// is this obj last of the LinkList
	if p.last == obj {
		p.last = obj.Pre
	}
	obj.MoveToFront(p.front)
	p.front = obj
}

func (p *MongoClientLRUPool) Get(url string) (*mongo.Client, bool) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if obj, ok := p.data[url]; ok {
		p.refresh(obj)
		return obj.Val.Client, ok
	}
	return nil, false
}

func (p *MongoClientLRUPool) dropOverflowingClient(num int) {
	for ; num > 0; num-- {
		last := p.last
		delete(p.data, last.Val.URL)
		p.last = last.Pre
		go last.Val.Client.Disconnect(context.Background())
	}
}

func (p *MongoClientLRUPool) Put(url string, cli *mongo.Client) {
	p.mux.Lock()
	defer p.mux.Unlock()
	if obj, ok := p.data[url]; ok {
		p.refresh(obj)
		return
	}
	if dataLen := len(p.data); dataLen >= p.limit {
		p.dropOverflowingClient(dataLen - p.limit + 1)
	}
	clientAndURL := MongoClientAndURL{url, cli}
	l := LinkList[MongoClientAndURL]{
		nil,
		nil,
		clientAndURL,
	}
	p.data[url] = &l
	p.refresh(&l)
}
