package p

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/slack-go/slack"
)

type Attributes struct {
	Name             string `json:"name"`
	Status           string `json:"status"`
	ClusterTestSuite string `json:"clusterTestSuite"`
	CompletionTime   string `json:"completionTime"`
	Platform         string `json:"platform"`
}

type PubSubMessage struct {
	Data       string     `json:"data"`
	Attributes Attributes `json:"attributes"`
}

const channelID = "C014YQ2R44E"
const tokenEnvKey = "SLACK_TOKEN"

// HelloPubSub consumes a Pub/Sub message.
func HelloPubSub(ctx context.Context, m PubSubMessage) error {
	token := os.Getenv(tokenEnvKey)
	if token == "" {
		return fmt.Errorf("%s env is required", tokenEnvKey)
	}

	api := slack.New(token)

	decodedData, err := b64.StdEncoding.DecodeString(m.Data)
	if err != nil {
		return errors.Wrap(err, "while decoding event message")
	}

	hist, err := api.GetChannelHistory(channelID, slack.HistoryParameters{
		Count: 100,
	})
	if err != nil {
		return err
	}

	parentMessage := fmt.Sprintf("ClusterTestSuite %s, completionTime %s, platform %s", m.Attributes.ClusterTestSuite, m.Attributes.CompletionTime, m.Attributes.Platform)

	parentMsgTimestamp, exists := parentMessageExists(*hist, parentMessage)

	if !exists {
		_, _, err = api.PostMessage(channelID, slack.MsgOptionText(parentMessage, false))
		if err != nil {
			return errors.Wrap(err, "while creating slack thread")
		}
	}

	time.Sleep(2 * time.Second)

	_, err = api.UploadFile(slack.FileUploadParameters{
		Content:        string(decodedData),
		Filename:       "logs.txt",
		Title:          "Test logs",
		InitialComment: fmt.Sprintf("Test %s, status: %s", m.Attributes.Name, m.Attributes.Status),
		Channels: []string{
			"#dontworkbruh",
		},
		ThreadTimestamp: parentMsgTimestamp,
	})

	return err
}

func parentMessageExists(hist slack.History, parentMsg string) (string, bool) {
	for _, msg := range hist.Messages {
		if msg.Text == parentMsg {
			return msg.Timestamp, true
		}
	}
	return "", false
}
