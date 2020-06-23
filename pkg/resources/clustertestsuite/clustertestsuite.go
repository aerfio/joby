package clustertestsuite

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	ctsTypes "github.com/aerfio/joblogs/pkg/resources/clustertestsuite/types"

	"github.com/aerfio/joblogs/pkg/resources"

	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterTestSuite struct {
	resCli      *resource.Resource
	waitTimeout time.Duration
}

func New(dynamicCli dynamic.Interface, waitTimeout time.Duration) *ClusterTestSuite {
	return &ClusterTestSuite{
		resCli:      resource.New(dynamicCli, ctsTypes.SchemeGroupVersion.WithResource("clustertestsuites"), ""),
		waitTimeout: waitTimeout,
	}
}

func (cts ClusterTestSuite) List() (ctsTypes.ClusterTestSuiteList, error) {
	ul, err := cts.resCli.ResCli.List(metav1.ListOptions{})
	if err != nil {
		return ctsTypes.ClusterTestSuiteList{}, err
	}

	clusterTestSuites, err := convertFromUnstructuredToClusterTestSuiteList(&unstructured.Unstructured{Object: ul.UnstructuredContent()})
	if err != nil {
		return ctsTypes.ClusterTestSuiteList{}, err
	}

	return clusterTestSuites, nil
}

func convertFromUnstructuredToClusterTestSuiteList(u *unstructured.Unstructured) (ctsTypes.ClusterTestSuiteList, error) {
	cts := ctsTypes.ClusterTestSuiteList{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &cts)
	return cts, err
}
