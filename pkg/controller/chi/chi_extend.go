package chi

import (
	"bytes"
	"context"
	"fmt"
	log "github.com/squids-io/clickhouse-operator/pkg/announcer"
	chop "github.com/squids-io/clickhouse-operator/pkg/apis/clickhouse.altinity.com/v1"
	chiopConfig "github.com/squids-io/clickhouse-operator/pkg/chop"
	chopmodel "github.com/squids-io/clickhouse-operator/pkg/model"
	"github.com/squids-io/clickhouse-operator/pkg/util"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	kuberest "k8s.io/client-go/rest"
	kubeclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

func (c *Controller) RunClickhouseStop(ctx context.Context, host *chop.ChiHost) error {
	if util.IsContextDone(ctx) {
		log.V(2).Info("ctx is done")
		return nil
	}
	var livenessPort int32
	name := chopmodel.CreateStatefulSetName(host)
	podName := fmt.Sprintf("%s-%d", name, 0)
	namespace := host.Address.Namespace
	log.V(1).M(host).Info("Start push metadata %s", podName)
	_ = c.pushMetadata(host, namespace, podName)
	// removeLivenessProbeProb
	log.V(1).M(host).Info("Start remove pod %s liveness Probe", podName)
	portName, err := c.removeProbeProb(ctx, host, name, namespace)
	if err != nil {
		return err
	}
	log.V(1).M(host).Info("removeProbeProb success")
	containerPort := host.StatefulSet.Spec.Template.Spec.Containers[0].Ports
	for i := 0; i < len(containerPort); i++ {
		port := containerPort[i]
		if port.Name == portName {
			livenessPort = port.ContainerPort
		}
	}
	podHost := host.StatefulSet.Spec.ServiceName
	log.V(1).M(host).Info("start poll Probe %s", podName)
	podHost1 := fmt.Sprintf("%s.%s.svc.cluster.local", podHost, namespace)
	log.V(1).M(host).Info("hostName is %s", podHost1)
	if pollLivenessProbe(podHost1, livenessPort, nil) {
		log.V(1).M(host).Info("Probe remove Success %s", podName)
		_ = c.runStop(host, namespace, podName)
		if c.checkContainerStatus(ctx, namespace, podName, nil) {
			log.V(1).M(host).Info("Clickhouse %s is Completed", podName)
			var zero int32 = 0
			host.StatefulSet.Spec.Replicas = &zero
			host.StatefulSet.ResourceVersion = ""
			if _, err = c.kubeClient.AppsV1().StatefulSets(namespace).Update(ctx, host.StatefulSet, newUpdateOptions()); err != nil {
				log.V(1).M(host).Error("CHI_EXTEND UNABLE to update StatefulSet %s/%s, err: %s", namespace, name, err)
				return err
			}
		}
	}

	return nil
}

func (c *Controller) runStop(host *chop.ChiHost, namespace string, podName string) error {
	cmd := []string{
		"/bin/sh",
		"-c",
		"clickhouse stop &",
	}
	log.V(1).M(host).E().Info(fmt.Sprintf("STOP Clickhouse %s/%s", namespace, podName))
	stdOut, stdErr, err := c.Exec(namespace, podName, "clickhouse", cmd)
	if err != nil {
		// err中添加报错信息
		log.V(1).M(host).Error("UNABLE Stop Clickhouse %s/%s, err: %s/%s", namespace, podName, stdErr, err)
		return err
	}
	log.V(1).M(host).E().Info(stdOut)
	return err
}

func (c *Controller) pushMetadata(host *chop.ChiHost, namespace string, podName string) error {
	cmd := []string{
		"/bin/sh",
		"-c",
		"/chi/sync.sh",
	}
	log.V(1).M(host).E().Info(fmt.Sprintf("Push Clickhouse %s/%s", namespace, podName))
	stdOut, stdErr, err := c.Exec(namespace, podName, "sync", cmd)
	if err != nil {
		// err中添加报错信息
		log.V(1).M(host).Error("UNABLE Push Clickhouse %s/%s, err: %s/%s", namespace, podName, stdErr, err)
		return err
	}
	log.V(1).M(host).E().Info(stdOut)
	return err
}

func (c *Controller) removeProbeProb(ctx context.Context, host *chop.ChiHost, name string, namespace string) (string, error) {
	if util.IsContextDone(ctx) {
		log.V(2).Info("ctx is done")
		return "", nil
	}
	container, ok := getClickHouseContainer(host.StatefulSet)
	if !ok {
		return "", nil
	}
	portName := container.Ports[0].Name
	if container.LivenessProbe == nil && container.ReadinessProbe == nil {
		return portName, nil
	}
	container.LivenessProbe = nil
	container.ReadinessProbe = nil
	if _, err := c.kubeClient.AppsV1().StatefulSets(namespace).Update(ctx, host.StatefulSet, newUpdateOptions()); err != nil {
		log.V(1).M(host).Error("UNABLE to update StatefulSet %s/%s", namespace, name)
		return "", err
	}
	_ = c.waitHostReady(ctx, host)
	return portName, nil
}

func (c *Controller) checkContainerStatus(ctx context.Context, namespace string, podName string, opts *StatefulSetPollOptions) bool {
	opts = opts.Ensure().FromConfig(chiopConfig.Config())
	start := time.Now()
	for {
		time.Sleep(3 * time.Second)
		pod, _ := c.kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		for i := 0; i < len(pod.Status.ContainerStatuses); i++ {
			if pod.Status.ContainerStatuses[i].Name == "clickhouse" {
				if pod.Status.ContainerStatuses[i].State.Terminated != nil && pod.Status.ContainerStatuses[i].State.Terminated.Reason == "Completed" {
					return true
				}
			}
		}
		if time.Since(start) >= opts.Timeout {
			// Timeout reached, no good result available, time to quit
			log.V(1).E().Error("Check Clickhouse Status %s- TIMEOUT reached", podName)
			return false
		}
	}
}

func pollLivenessProbe(host string, livenessPort int32, opts *StatefulSetPollOptions) bool {
	opts = opts.Ensure().FromConfig(chiopConfig.Config())
	start := time.Now()
	for {
		time.Sleep(10 * time.Second)
		log.V(1).Info("Check Live Probe[ http://%s:%d/ping ]", host, livenessPort)
		resp, _ := http.Get(fmt.Sprintf("http://%s:%d/ping", host, livenessPort))
		if resp == nil {
			continue
		}
		defer resp.Body.Close()
		if resp.Status == "200 OK" {
			return true
		}
		if time.Since(start) >= opts.Timeout {
			// Timeout reached, no good result available, time to quit
			log.V(1).E().Error("Check Live Probe %s/%s - TIMEOUT reached", host, livenessPort)
			return false
		}
	}
}

func (c *Controller) Exec(namespace string, podName string, containerName string, command []string) (string, string, error) {

	var cmdStr string
	for _, c := range command {
		cmdStr = cmdStr + " " + c
	}
	log.V(1).E().Info("exec command: " + cmdStr)
	var (
		execOut bytes.Buffer
		execErr bytes.Buffer
	)

	req := c.kubeClient.CoreV1().RESTClient().Post().Resource("pods").
		Name(podName).Namespace(namespace).SubResource("exec").Timeout(time.Duration(30) * time.Minute)
	req.VersionedParams(&v1.PodExecOptions{
		Stdout:    true,
		Stderr:    true,
		Container: containerName,
		Command:   command,
	}, scheme.ParameterCodec)
	config, _ := getKubeConfig("", "")
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &execOut,
		Stderr: &execErr,
		Tty:    false,
	})
	log.Info(fmt.Sprintf("exec err: %v", err))
	log.V(1).Info(fmt.Sprintf("stdErr: %s", execErr.String()))
	log.V(1).Info(fmt.Sprintf("stdOut: %s", execOut.String()))
	if err != nil {
		return "", execErr.String(), err
	}
	return execOut.String(), execErr.String(), nil
}

func getKubeConfig(kubeConfigFile, masterURL string) (*kuberest.Config, error) {
	if len(kubeConfigFile) > 0 {
		// kube config file specified as CLI flag
		return kubeclientcmd.BuildConfigFromFlags(masterURL, kubeConfigFile)
	}

	if len(os.Getenv("KUBECONFIG")) > 0 {
		// kube config file specified as ENV var
		return kubeclientcmd.BuildConfigFromFlags(masterURL, os.Getenv("KUBECONFIG"))
	}

	if conf, err := kuberest.InClusterConfig(); err == nil {
		// in-cluster configuration found
		return conf, nil
	}

	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	// OS user found. Parse ~/.kube/config file
	conf, err := kubeclientcmd.BuildConfigFromFlags("", filepath.Join(usr.HomeDir, ".kube", "config"))
	if err != nil {
		return nil, fmt.Errorf("~/.kube/config not found")
	}

	// ~/.kube/config found
	return conf, nil
}

func getClickHouseContainer(statefulSet *apps.StatefulSet) (*corev1.Container, bool) {
	// Find by name
	for i := range statefulSet.Spec.Template.Spec.Containers {
		container := &statefulSet.Spec.Template.Spec.Containers[i]
		if container.Name == chopmodel.ClickHouseContainerName {
			return container, true
		}
	}

	// Find by index
	if len(statefulSet.Spec.Template.Spec.Containers) > 0 {
		return &statefulSet.Spec.Template.Spec.Containers[0], true
	}

	return nil, false
}
