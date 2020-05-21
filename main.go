package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"cloud.google.com/go/pubsub"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func main() {
	projectID := os.Getenv("PROJECT_ID")
	topicID := os.Getenv("TOPIC_ID")

	if len(projectID) == 0 || len(topicID) == 0 {
		log.Fatal("please provide proper config")
	}

	config := controllerruntime.GetConfigOrDie()

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{
		LabelSelector: "testing.kyma-project.io/created-by-octopus=true",
	})
	if err != nil {
		panic(err.Error())
	}

	for _, pod := range pods.Items {
		container, err := getTestContainerName(pod)
		if err != nil {
			log.Fatal(err)
		}

		req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Namespace, &corev1.PodLogOptions{
			Container: container,
		})

		var b bytes.Buffer

		if err := DefaultConsumeRequest(req, &b); err != nil {
			log.Fatal(err.Error())
		}

		val, ok := pod.Labels["testing.kyma-project.io/def-name"]
		if !ok {
			log.Fatal("there's no `testing.kyma-project.io/def-name` on a pod")
		}

		attr := map[string]string{
			"name": val,
		}

		err = publish(os.Stdout, projectID, topicID, attr, b.Bytes())
		if err != nil {
			log.Fatal(err.Error())
		}
	}
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
		return fmt.Errorf("pubsub.NewClient: %v", err)
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
	_, err = fmt.Fprintf(w, "Published a message; msg ID: %v\n", id)
	return err
}

func newRestClientConfig(kubeconfigPath string) (*restclient.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	return restclient.InClusterConfig()
}

// DefaultConsumeRequest reads the data from request and writes into
// the out writer. It buffers data from requests until the newline or io.EOF
// occurs in the data, so it doesn't interleave logs sub-line
// when running concurrently.
//
// A successful read returns err == nil, not err == io.EOF.
// Because the function is defined to read from request until io.EOF, it does
// not treat an io.EOF as an error to be reported.
func DefaultConsumeRequest(request restclient.ResponseWrapper, out io.Writer) error {
	readCloser, err := request.Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)
	for {
		bytes, err := r.ReadBytes('\n')
		if _, err := out.Write(bytes); err != nil {
			return err
		}

		if err != nil {
			if err != io.EOF {
				return err
			}
			return nil
		}
	}
}
