package exporter

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Exporter struct {
	client *mongo.Client
}

type Opts struct {
	DSN string
}

func New(opts *Opts) (*Exporter, error) {
	if opts == nil {
		opts = new(Opts)
	}

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(opts.DSN))
	if err != nil {
		return nil, err
	}

	if err = client.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}

	return &Exporter{
		client: client,
	}, nil
}
