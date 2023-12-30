# kueueleuleu

Run containers sequentially inside Kubernetes `Pod`s, `Job`s, `CronJob`s.

The name `kueueleuleu` (\kø lø.lø\) comes from the French [`queue leu-leu`](https://fr.wiktionary.org/wiki/%C3%A0_la_queue_leu-leu), meaning "one after the other" or "[in a single file](https://youtu.be/eIRbCW3vH-Y?t=106)". The initial `k` is obviously for `kubernetes`.

## Motivation

As of now, Kubernetes does not support running containers inside a `Pod` sequentially, despite this being a common thing asked by users:

- [How to run containers sequentially as a Kubernetes job?](https://stackoverflow.com/questions/40713573/how-to-run-containers-sequentially-as-a-kubernetes-job)
- [How to run multiple Kubernetes jobs in sequence?](https://stackoverflow.com/questions/48029943/how-to-run-multiple-kubernetes-jobs-in-sequence)
- [Kubernetes — Sequencing Container Startup, by Paul Dally](https://pauldally.medium.com/sequencing-container-startup-ab30965c067d)
- [How to start one container before staring another in the same pod?](https://groups.google.com/g/kubernetes-users/c/JqvIuUmt5fk)

Without an external way to control the containers run order, all containers inside a `Pod` start and run at the same time.

### Existing known solutions

<details>
<summary>TL;DR: Convert your containers to init containers OR use external workflow solutions.</summary>

#### Use init containers

[Init containers](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#differences-from-regular-containers) always run sequentially:

> If you specify multiple init containers for a Pod, kubelet runs each init container sequentially. Each init container must succeed before the next can run.

Thus, a common workaround to allow running containers sequentially is to convert all containers to init containers, but:

- this requires to change the pod spec ourselves, which is not always possible or might feel "hacky"
- init containers have some restrictions (e.g. does not support probes, handles resource requests/limits differently than regular containers)

#### Use external tools designed for workflows

There are some solutions to define complex containers workflows (and among them, running containers sequentially):

- [Argo Workflows](https://argoproj.github.io/workflows)
- [Tekton](https://tekton.dev)

But, for such a simple use-case (run containers one after the other), they might seem overkill:

- need to install (and maintain) a fully-fledged tool
- need to use specific CRDs (Argo `Workflow` or Tekton `Pipeline`)
</details>

### Kueueleuleu!

Unlike the known solutions mentioned above, this simple library aims to:

- hide the sequential containers orchestration logic from the user
- let users manipulate raw kubernetes objects (`Pod`s, `Job`s, `CronJob`s), so they can use everything natively supported by Kubernetes (volumes, runtime classes, etc.)
- provide an easy way to convert any of these kubernetes objects to `kueueleuleu` objects

## How to use?

This library requires Go >= 1.21.

### Using the CLI

```shell
go install github.com/norbjd/kueueleuleu/cmd/kueueleuleu@latest

# create a simple pod with two containers: by default, both will run at the same time
cat > simplepod.yaml <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: two-steps-pod
spec:
  containers:
    - name: step1
      image: alpine
      command:
        - sh
        - -c
      args:
        - echo "start step1" && sleep 5 && echo "end step1"
    - name: step2
      image: alpine
      command:
        - sh
        - -c
      args:
        - echo "start step2" && sleep 2 && echo "end step2"
  restartPolicy: Never
EOF

# convert this pod to run containers sequentially and apply
# if you like chaining commands, this is similar to: cat simplepod.yaml | ./kueueleuleu -f - | kubectl apply -f -
# if kueueleuleu is not in your PATH, replace with $GOPATH/bin/kueueleuleu (default when running go install)
kueueleuleu -f simplepod.yaml | kubectl apply -f -
```

Once the pod is finished, check the logs (`kubectl logs two-steps-pod --all-containers --timestamps | sort`) to see containers have been executed sequentially (first, `step1`, and then `step2`):

```
2023-12-29T13:32:50.488543841Z 2023/12/29 13:32:50 Entrypoint initialization
2023-12-29T13:32:52.139027305Z start step1
2023-12-29T13:32:57.139823758Z end step1
2023-12-29T13:32:57.392700813Z start step2
2023-12-29T13:32:59.393583491Z end step2
```

> [!NOTE]
> The first log is related to internal logic: the `entrypoint` mentioned is here to order the containers' execution. You can ignore this, unless you are interested in the internals (see "Internals" section).

Without `kueueleuleu` (`kubectl apply -f simplepod.yaml`), containers logs are intertwined because both containers are running at the same time:

```
2023-12-29T13:33:43.942542903Z start step1
2023-12-29T13:33:44.848147388Z start step2
2023-12-29T13:33:46.848812173Z end step2
2023-12-29T13:33:48.943165982Z end step1
```

Conversion also work with `Job`s and `CronJob`s, and even with YAML files containing multiple resources (see `cmd/testdata/*_input.yaml` for examples, and `cmd/testdata/*_output.yaml` for results after using `kueueleuleu`).

### Using the library

First, run `go get github.com/norbjd/kueueleuleu@latest` to download the dependency.

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/norbjd/kueueleuleu"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

func main() {
    // original pod: containers will run at the same time
    twoStepsPod := corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: "two-steps-pod",
        },
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{
                {
                    Name:    "step1",
                    Image:   "alpine",
                    Command: []string{"sh", "-c"},
                    Args:    []string{`echo "start step1" && sleep 5 && echo "end step1"`},
                },
                {
                    Name:    "step2",
                    Image:   "alpine",
                    Command: []string{"sh", "-c"},
                    Args:    []string{`echo "start step2" && sleep 2 && echo "end step2"`},
                },
            },
            RestartPolicy: "Never",
        },
    }

    // convert the pod to run containers sequentially
    twoSequentialStepsPod, err := kueueleuleu.ConvertPod(twoStepsPod)
    if err != nil {
        panic(err)
    }

    // create the pod
    config, _ := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
    kubeClient, _ := kubernetes.NewForConfig(config)

    createdPod, err := kubeClient.CoreV1().Pods("default").Create(
        context.Background(), &twoSequentialStepsPod, metav1.CreateOptions{},
    )
    if err != nil {
        panic(err)
    }

    fmt.Printf("Created: %s\n", createdPod.Name)
}
```

The main advantage of `kueueleuleu` is that you can continue to manipulate standard kubernetes resources (like `Pod`s) in your code. There is only a single helper if you want to know which container inside the pod is really running (as now they are running sequentially):

```go
currentlyRunningContainerName, _ := kueueleuleu.GetRunningContainerName(*createdPod)

fmt.Printf("Running container: %s\n", currentlyRunningContainerName)
```

As for the CLI, the conversion also work with `Job`s and `CronJob`s: just use `kueueleuleu.ConvertJob` or `kueueleuleu.ConvertCronJob`.

## Internals

Under the hood, containers sequential orchestration is managed using [Tekton entrypoint](https://github.com/tektoncd/pipeline/blob/v0.55.0/cmd/entrypoint/README.md). I have just "reverse-engineered" the way Tekton generates `Pod`s from `PipelineRun`. But, unlike Tekton, there is no need to install a separate controller in the cluster or using `CRD`s, which makes `kueueleuleu` lighter to use. In return, `kueueleuleu` cannot be used for complex workflows, and I don't consider supporting these: its **only** job is to run containers **sequentially**.

I have used Tekton entrypoint because it is already doing the job, and I didn't want to rewrite another wrapper to do more or less the same thing.

Another solution I have considered had been to convert containers to init containers (see "Use init containers" section above). But, considering init containers limitations, I thought this solution was not viable.

## Limitations

Containers passed in objects (`Pod`, `Job`, `CronJob`) **MUST** have their `command` field set. Otherwise, `kueueleuleu` will return an error (`container does not have a command`). This is because Tekton entrypoint used under the hood requires a command. Tekton Pipelines (who also uses this entrypoint) manages to "guess" the entrypoint if a command is not provided (`entrypoint hack` refered [here](https://github.com/tektoncd/pipeline/issues/6877#issuecomment-1618082473)). But, I'd rather not implement this here today as it does not always work (e.g. if image is not pushed into a registry, like in `kind` environments, we will fail to guess the entrypoint).
