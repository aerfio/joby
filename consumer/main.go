package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"cloud.google.com/go/pubsub"
)

func main() {
	projectID := os.Getenv("PROJECT_ID")
	topicID := os.Getenv("TOPIC_ID")

	if len(projectID) == 0 || len(topicID) == 0 {
		log.Fatal("please provide proper config")
	}

	// copied from official docs just to test shit
	if err := pullMsgs(os.Stdout, projectID, topicID); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func pullMsgs(w io.Writer, projectID, subID string) error {
	// projectID := "my-project-id"
	// subID := "my-sub"
	// topic of type https://godoc.org/cloud.google.com/go/pubsub#Topic
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("pubsub.NewClient: %v", err)
	}

	// Consume 10 messages.
	var mu sync.Mutex
	received := 0
	sub := client.Subscription(subID)
	cctx, cancel := context.WithCancel(ctx)
	err = sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		_, err := fmt.Fprintf(w, "Got message, attr %+v\n", msg.Attributes, string(msg.Data))
		if err != nil {
			log.Fatal(err)
		}

		// s, err := strconv.Unquote(string(msg.Data))
		// if err != nil {
		// 	log.Fatal(err)
		// }
		_, err = fmt.Fprintf(w, "%s\n", string(msg.Data))
		if err != nil {
			log.Fatal(err)
		}
		msg.Ack()
		mu.Lock()
		defer mu.Unlock()
		received++
		// read 10 messages
		if received == 10 {
			cancel()
		}
	})
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}
	return nil
}
