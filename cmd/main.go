package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/vrischmann/envconfig"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var logf logr.Logger

const (
	octopusSelector     = "testing.kyma-project.io/created-by-octopus=true"
	octopusTestLabelKey = "testing.kyma-project.io/def-name"
)

func init() {
	ctrllog.SetLogger(zap.New())
	logf = ctrllog.Log.WithName("log-exporter")
}

type config struct {
	ProjectID string
	TopicID   string
}

func main() {
	logf.Error(mainerr(), "while running application")
	os.Exit(1)
}

func mainerr() error {
	conf := &config{}
	err := envconfig.InitWithPrefix(conf, "APP")
	if err != nil {
		return errors.Wrap(err, "while loading env config")
	}

	clientset, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return errors.Wrap(err, "while creating clientset")
	}

	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{
		LabelSelector: octopusSelector,
	})

	if err != nil {
		return errors.Wrapf(err, "while listing pods by %s selector", octopusSelector)
	}

	for _, pod := range pods.Items {
		container, err := getTestContainerName(pod)
		if err != nil {
			return errors.Wrapf(err, "while extracting test container name from pod %s in namespace %s", pod.GetName(), pod.GetNamespace())
		}

		req := clientset.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &corev1.PodLogOptions{
			Container: container,
		})

		data, err := ConsumeRequest(req)
		if err != nil {
			return fmt.Errorf("while reading request from container %s in pod %s in namespace %s", container, pod.GetName(), pod.GetNamespace())
		}

		val, ok := pod.Labels[octopusTestLabelKey]
		if !ok {
			return fmt.Errorf("there's no `%s` label on a pod %s in namespace %s", octopusTestLabelKey, pod.GetName(), pod.GetNamespace())
		}

		attributes := map[string]string{
			"name": val,
		}

		if err := publish(os.Stdout, conf.ProjectID, conf.TopicID, attributes, data); err != nil {
			return fmt.Errorf("while publishing message to topic %s for pod %s from namespace %s", conf.TopicID, pod.GetName(), pod.GetNamespace())
		}
	}
	return nil
}

func getTestContainerName(pod corev1.Pod) (string, error) {
	names := []string{}
	for _, cont := range pod.Spec.Containers {
		if cont.Name != "istio-proxy" {
			names = append(names, cont.Name)
		}
	}

	if len(names) != 1 {
		// todo extract to var
		return "", fmt.Errorf("found more than 1 non-istio containers in pod %s in namespace %s", pod.Name, pod.Namespace)
	}

	return names[0], nil
}

func publish(w io.Writer, projectID, topicID string, attributes map[string]string, msg []byte) error {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return errors.Wrapf(err, "while creating new pubsub client")
	}

	t := client.Topic(topicID)
	result := t.Publish(ctx, &pubsub.Message{
		Data:       msg,
		Attributes: attributes,
	})
	// Block until the result is returned and a server-generated
	// ID is returned for the published message.
	id, err := result.Get(ctx)
	if err != nil {
		return fmt.Errorf("Get: %v", err)
	}

	logf.Info("Published a message", "message", string(msg)[:50], "id", id)
	return nil
}

// ConsumeRequest reads the data from request and writes into
// the out writer. It buffers data from requests until the newline or io.EOF
// occurs in the data, so it doesn't interleave logs sub-line
// when running concurrently.
//
// A successful read returns err == nil, not err == io.EOF.
// Because the function is defined to read from request until io.EOF, it does
// not treat an io.EOF as an error to be reported.
func ConsumeRequest(request restclient.ResponseWrapper) ([]byte, error) {
	var b bytes.Buffer
	readCloser, err := request.Stream()
	if err != nil {
		return []byte{}, err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)
	for {
		bytesln, err := r.ReadBytes('\n')
		if _, err := b.Write(bytesln); err != nil {
			return []byte{}, err
		}

		if err != nil {
			if err != io.EOF {
				return []byte{}, err
			}
			return b.Bytes(), nil
		}
	}
}
