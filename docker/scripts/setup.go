package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func waitForStartup(client *mongo.Client, ctx context.Context) error {
	for {
		err := client.Ping(ctx, nil)
		if err == nil {
			break
		}
		fmt.Print(".")
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Started..")
	return nil
}

func cnfServers(client *mongo.Client, ctx context.Context, configServers []string) error {
	fmt.Println("Setting up config servers")
	rsConfig := mongo.Cmd{
		{"replSetInitiate", map[string]interface{}{
			"_id":         "configReplSet",
			"configsvr":   true,
			"members":     configServers,
			"protocolVersion": 1,
		}},
	}

	_, err := client.Database("admin").RunCommand(ctx, rsConfig).DecodeBytes()
	return err
}

func generalServers(client *mongo.Client, ctx context.Context, replicaSet []string, arbiter string) error {
	fmt.Println("Setting up general servers")
	rsConfig := mongo.Cmd{
		{"replSetInitiate", map[string]interface{}{
			"_id":         "rs0",
			"members":     replicaSet,
			"protocolVersion": 1,
		}},
	}

	_, err := client.Database("admin").RunCommand(ctx, rsConfig).DecodeBytes()
	if err != nil {
		return err
	}

	arbCmd := fmt.Sprintf("rs.addArb(\"%s\")", arbiter)
	_, err = client.Database("admin").RunCommand(ctx, mongo.Cmd{"eval", arbCmd}).DecodeBytes()

	return err
}

func main() {
	ctx := context.TODO()

	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		fmt.Println("Error connecting to MongoDB:", err)
		os.Exit(1)
	}
	defer client.Disconnect(ctx)

	mongo1 := os.Getenv("MONGO1")
	mongo2 := os.Getenv("MONGO2")
	mongo3 := os.Getenv("MONGO3")
	arbiter := os.Getenv("ARBITER")

	configServers := []string{mongo1, mongo2, mongo3}
	replicaSet := []string{mongo1, mongo2, mongo3}

	err = waitForStartup(client, ctx)
	if err != nil {
		fmt.Println("Error waiting for startup:", err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "cnf_servers":
		err = cnfServers(client, ctx, configServers)
	case "general_servers":
		err = generalServers(client, ctx, replicaSet, arbiter)
	default:
		err = generalServers(client, ctx, replicaSet, arbiter)
	}

	if err != nil {
		fmt.Println("Error setting up MongoDB replica set:", err)
		os.Exit(1)
	}
}

