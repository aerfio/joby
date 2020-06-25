package main

import (
	"context"

	"github.com/aerfio/joblogs/slack_lambda/p"
)

func main() {
	p.HelloPubSub(context.TODO(), p.PubSubMessage{
		Data:       "",
		Attributes: p.Attributes{},
	})
}
