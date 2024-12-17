package main

import (
	"context"

	"go.etcd.io/etcd/client/v3"
)

func main() {
	ctx := context.Background()
	cli, err := clientv3.New(
		clientv3.Config{
			Endpoints: []string{"localhost:2379"},
		},
	)
	if err != nil {
		return
	}
	defer cli.Close()

	for {
		go cli.Put(ctx, "/message", "HELLO")
	}
}
