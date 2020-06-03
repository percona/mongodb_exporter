package exporter

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Exporter holds Exporter methods and attributes.
type Exporter struct {
	client *mongo.Client
}

// Opts holds new exporter options.
type Opts struct {
	DSN string
}

// New connects to the database and returns a new Exporter instance.
func New(opts *Opts) (*Exporter, error) {
	if opts == nil {
		opts = new(Opts)
	}

	client, err := connect(context.Background(), opts.DSN)
	if err != nil {
		return nil, err
	}

	return &Exporter{
		client: client,
	}, nil
}

// Disconnect from the database.
func (e *Exporter) Disconnect(ctx context.Context) error {
	return e.client.Disconnect(ctx)
}

func connect(ctx context.Context, dsn string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(dsn)
	clientOpts.SetDirect(true)
	clientOpts.SetAppName("mnogo_exporter")

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	if err = client.Ping(context.TODO(), nil); err != nil {
		return nil, err
	}

	return client, nil
}
