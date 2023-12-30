//go:build e2e

package kueueleuleu_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/norbjd/kueueleuleu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var podSpecSleep = corev1.PodSpec{
	InitContainers: []corev1.Container{
		{
			Name:    "init-sleep-10",
			Image:   "alpine",
			Command: []string{"sleep"},
			Args:    []string{"10"},
		},
	},
	Containers: []corev1.Container{
		{
			Name:    "step1-sleep-5",
			Image:   "alpine",
			Command: []string{"sleep"},
			Args:    []string{"5"},
		},
		{
			Name:    "step2-sleep-10",
			Image:   "alpine",
			Command: []string{"sleep"},
			Args:    []string{"10"},
		},
		{
			Name:    "step3-sleep-20",
			Image:   "alpine",
			Command: []string{"sleep"},
			Args:    []string{"20"},
		},
	},
	RestartPolicy: "Never",
}

var podSpecWhalesayValid = corev1.PodSpec{
	Containers: []corev1.Container{
		{
			Name:    "say-hello",
			Image:   "docker/whalesay",
			Command: []string{"cowsay"},
			Args:    []string{"hello"},
		},
		{
			Name:    "say-nothing",
			Image:   "docker/whalesay",
			Command: []string{"cowsay"},
		},
	},
	RestartPolicy: "Never",
}

var podSpecWhalesayInvalid = corev1.PodSpec{
	Containers: []corev1.Container{
		{
			Name:    "say-hello",
			Image:   "docker/whalesay",
			Command: []string{"cowsay"},
			Args:    []string{"hello"},
		},
		{
			Name:  "say-goodbye",
			Image: "docker/whalesay",
			// here, Command is not set, so kueueleuleu should return an error
			Args: []string{"goodbye"},
		},
	},
	RestartPolicy: "Never",
}

var whalesay = ` _ 
<   >
 - 
    \
     \
      \     
                    ##        .            
              ## ## ##       ==            
           ## ## ## ##      ===            
       /""""""""""""""""___/ ===        
  ~~~ {~~ ~~~~ ~~~ ~~~~ ~~ ~ /  ===- ~~~   
       \______ o          __/            
        \    \        __/             
          \____\______/   
`

var debug bool

func init() {
	debugEnvVar := os.Getenv("DEBUG")
	debug = debugEnvVar != "" && debugEnvVar != "0" && debugEnvVar != "false"
}

func getKubeClient(t *testing.T) kubernetes.Interface {
	t.Helper()

	kubeconfigPath := os.Getenv("KUBECONFIG")

	if kubeconfigPath == "" {
		t.Fatal("Env var KUBECONFIG must be set")
	} else if _, err := os.Stat(kubeconfigPath); err != nil {
		t.Fatalf("Cannot retrieve file %s", kubeconfigPath)
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		t.Fatal(err)
	}

	config.QPS = 200
	config.Burst = 200

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	return kubeClient
}

func Test_CreatePod(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("dummy-%s", uuid.NewUUID()),
		},
		Spec: podSpec,
	}

	assert.False(t, kueueleuleu.IsKueueleuleu(pod.ObjectMeta))

	kueueleuleuPod, err := kueueleuleu.ConvertPod(pod)
	require.NoError(t, err)
	assert.True(t, kueueleuleu.IsKueueleuleu(kueueleuleuPod.ObjectMeta))

	kubeClient := getKubeClient(t)
	ctx := context.Background()

	podCreated, err := kubeClient.CoreV1().
		Pods("default").
		Create(ctx, &kueueleuleuPod, metav1.CreateOptions{})
	if err != nil {
		t.Log(podCreated.Spec)
	}

	require.NoError(t, err)

	defer func() {
		err = kubeClient.CoreV1().Pods("default").Delete(ctx, podCreated.Name, metav1.DeleteOptions{
			PropagationPolicy: toPtr(metav1.DeletePropagationForeground),
		})
		require.NoError(t, err)
	}()

	waitUntilPodSucceeds(ctx, t, kubeClient, podCreated, 30*time.Second, debug)
}

func Test_CreatePodSleep(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("sleep-%s", uuid.NewUUID()),
		},
		Spec: podSpecSleep,
	}

	kueueleuleuPod, err := kueueleuleu.ConvertPod(pod)
	require.NoError(t, err)
	assert.True(t, kueueleuleu.IsKueueleuleu(kueueleuleuPod.ObjectMeta))

	kubeClient := getKubeClient(t)
	ctx := context.Background()

	podCreated, err := kubeClient.CoreV1().
		Pods("default").
		Create(ctx, &kueueleuleuPod, metav1.CreateOptions{})
	require.NoError(t, err)

	defer func() {
		err = kubeClient.CoreV1().Pods("default").Delete(ctx, podCreated.Name, metav1.DeleteOptions{
			PropagationPolicy: toPtr(metav1.DeletePropagationForeground),
		})
		require.NoError(t, err)
	}()

	results := waitUntilPodSucceeds(ctx, t, kubeClient, podCreated, 2*time.Minute, debug)
	for _, result := range results {
		t.Log(result)
	}

	require.GreaterOrEqual(t, len(results), 5)

	initSleep10StartedAt := results[0].t
	step1Sleep5StartedAt := results[1].t
	step2Sleep10StartedAt := results[2].t
	step3Sleep20StartetAt := results[3].t
	succeededAt := results[len(results)-1].t

	initSleep10StepDuration := step1Sleep5StartedAt - initSleep10StartedAt
	step1Sleep5Duration := step2Sleep10StartedAt - step1Sleep5StartedAt
	step2Sleep10Duration := step3Sleep20StartetAt - step2Sleep10StartedAt
	step3Sleep20Duration := succeededAt - step3Sleep20StartetAt

	assert.GreaterOrEqual(t, initSleep10StepDuration, 10*time.Second)
	assert.GreaterOrEqual(t, step1Sleep5Duration, 5*time.Second)
	assert.GreaterOrEqual(t, step2Sleep10Duration, 10*time.Second)
	assert.GreaterOrEqual(t, step3Sleep20Duration, 20*time.Second)
}

func Test_CreatePodWhalesayValid(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("whalesay-%s", uuid.NewUUID()),
		},
		Spec: podSpecWhalesayValid,
	}

	kueueleuleuPod, err := kueueleuleu.ConvertPod(pod)
	require.NoError(t, err)
	assert.True(t, kueueleuleu.IsKueueleuleu(kueueleuleuPod.ObjectMeta))

	kubeClient := getKubeClient(t)
	ctx := context.Background()

	podCreated, err := kubeClient.CoreV1().
		Pods("default").
		Create(ctx, &kueueleuleuPod, metav1.CreateOptions{})
	require.NoError(t, err)

	defer func() {
		err = kubeClient.CoreV1().Pods("default").Delete(ctx, podCreated.Name, metav1.DeleteOptions{
			PropagationPolicy: toPtr(metav1.DeletePropagationForeground),
		})
		require.NoError(t, err)
	}()

	_ = waitUntilPodSucceeds(ctx, t, kubeClient, podCreated, 2*time.Minute, debug)

	getPod, err := kubeClient.CoreV1().Pods("default").Get(context.Background(), podCreated.Name, metav1.GetOptions{})
	require.NoError(t, err)

	// this works because the first container name (say-hello) is before the second one (say-nothing) in alphabetical order
	// container statuses order does not honor containers order
	firstContainer := getPod.Status.ContainerStatuses[0]
	secondContainer := getPod.Status.ContainerStatuses[1]

	// sometimes, both finish at the same second because we don't have sub-second granularity, hence the LessOrEqual
	assert.LessOrEqual(t, firstContainer.State.Terminated.FinishedAt.Format(time.RFC3339Nano),
		secondContainer.State.Terminated.FinishedAt.Format(time.RFC3339Nano))

	logsReq := kubeClient.CoreV1().Pods("default").GetLogs(podCreated.Name, &corev1.PodLogOptions{
		Container: "say-nothing",
	})
	podLogs, err := logsReq.Stream(context.Background())
	require.NoError(t, err)

	defer podLogs.Close()
	logs, err := io.ReadAll(podLogs)
	require.NoError(t, err)

	assert.Equal(t, whalesay, string(logs))
}

func Test_CreatePodWhalesayInvalid(t *testing.T) {
	t.Parallel()

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("whalesay-%s", uuid.NewUUID()),
		},
		Spec: podSpecWhalesayInvalid,
	}

	_, err := kueueleuleu.ConvertPod(pod)
	require.ErrorIs(t, err, kueueleuleu.ErrContainerDoesNotHaveACommand)
	require.ErrorContains(t, err, "say-goodbye")
}

type podEvent struct {
	t                    time.Duration
	runningContainerName string
	podSucceeded         bool
}

func (e podEvent) String() string {
	s := fmt.Sprintf("[%s] %s", e.t.Round(time.Second), e.runningContainerName)
	if e.podSucceeded {
		s += " (succeeded)"
	}

	return s
}

func waitUntilPodSucceeds(ctx context.Context, t *testing.T,
	kubeClient kubernetes.Interface, pod *corev1.Pod, timeout time.Duration, debug bool,
) []podEvent {
	t.Helper()

	results := make([]podEvent, 0)

	watcher, err := kubeClient.CoreV1().Pods(pod.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:  fields.OneTermEqualSelector(metav1.ObjectNameField, pod.Name).String(),
		TimeoutSeconds: toPtr(int64(timeout.Seconds())),
		Limit:          10,
	})
	require.NoError(t, err)

	currentPod := *pod

	runningContainerName := ""

	start := time.Now()

	for event := range watcher.ResultChan() {
		eventPod, _ := event.Object.(*corev1.Pod)

		if debug {
			t.Log(cmp.Diff(currentPod, *eventPod))
		}

		currentPod = *eventPod

		//nolint:exhaustive
		switch eventPod.Status.Phase {
		case corev1.PodSucceeded:
			result := podEvent{t: time.Since(start), runningContainerName: "", podSucceeded: true}
			results = append(results, result)

			return results
		case corev1.PodFailed:
			t.Fatal("pod has failed")
		default:
			break
		}

		currentlyRunningContainerName, _ := kueueleuleu.GetRunningContainerName(*eventPod)
		if currentlyRunningContainerName != runningContainerName {
			runningContainerName = currentlyRunningContainerName
		} else {
			continue
		}

		result := podEvent{t: time.Since(start), runningContainerName: currentlyRunningContainerName, podSucceeded: false}
		results = append(results, result)
	}

	t.Fatalf("timeout exceeded, pod was not succeeded after %s", timeout)

	return nil
}
