package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Minute * 1
	storageNamespace     = os.Getenv("STORAGE_NAMESPACE")
	esServerUrls         = "http://elasticsearch." + storageNamespace + ".svc:9200"
	cassandraServiceName = "cassandra." + storageNamespace + ".svc"
)

// GetPod returns pod name
func GetPod(namespace, namePrefix, containsImage string, kubeclient kubernetes.Interface) (corev1.Pod, error) {
	pods, err := kubeclient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return corev1.Pod{}, err
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, namePrefix) {
			for _, c := range pod.Spec.Containers {
				if strings.Contains(c.Image, containsImage) {
					return pod, nil
				}
			}
		}
	}
	return corev1.Pod{}, fmt.Errorf("could not find pod with image %s", containsImage)
}

func prepare(t *testing.T) *framework.TestCtx {
	ctx := framework.NewTestCtx(t)
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: timeout, RetryInterval: retryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Initialized cluster resources. Namespace: " + namespace)

	// get global framework variables
	f := framework.Global
	// wait for the operator to be ready
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "jaeger-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	return ctx
}

type resp struct {
	Data []trace `json:"data"`
}

type trace struct {
	TraceID string `json:"traceID"`
	Spans   []span `json:"spans"`
}

type span struct {
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}
