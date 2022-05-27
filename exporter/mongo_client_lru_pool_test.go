package exporter

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"testing"
)

func TestMongoClientLRUPool_Put(t *testing.T) {
	p := NewMongoClientLRUPool(10)
	for i := 0; i < 11; i++ {
		p.Put(fmt.Sprintf("mongo client num: %d", i), MakeFakeMongoClientObj())
	}
	fmt.Println("data len: ", len(p.data))
	assert.True(t, len(p.data) == 10)
}

func TestMongoClientLRUPool_Get(t *testing.T) {
	p := NewMongoClientLRUPool(10)
	var num6Client *mongo.Client
	for i := 0; i < 11; i++ {
		if i == 6 {
			num6Client = MakeFakeMongoClientObj()
			p.Put(fmt.Sprintf("mongo client num: %d", i), num6Client)
		} else {
			p.Put(fmt.Sprintf("mongo client num: %d", i), MakeFakeMongoClientObj())
		}
	}
	cli, ok := p.Get("mongo client num: 6")
	if !ok || cli == nil {
		t.Fail()
	}
	if p.front.Val.URL != "mongo client num: 6" {
		t.Fail()
	}
}

func MakeFakeMongoClientObj() *mongo.Client {
	return new(mongo.Client)
}
