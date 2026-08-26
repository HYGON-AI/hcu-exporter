// Copyright (c) 2026 Hygon Information Technology Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package podresources

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"net"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
)

type PodResources interface {
	GetDeviceToPodInfo(node string) (map[string]PodInfo, error)
}

type podResources struct {
	timeout  time.Duration
	socket   string
	reources []string
	maxSize  int
}

func NewPodResourcesClient(timeout time.Duration, socket string, resources []string, maxSize int) PodResources {
	return &podResources{
		timeout:  timeout,
		socket:   socket,
		reources: resources,
		maxSize:  maxSize,
	}
}

type PodInfo struct {
	Pod         string
	Namespace   string
	Container   string
	DeviceId    string
	MinorNumber string
	Name        string
	UID         string
}

var (
	Kubeconfig = "/root/.kube/config"
)

func connectToServer(socket string, timeout time.Duration, maxSize int) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, socket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxSize)),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failure connecting to %s: %v", socket, err)
	}
	return conn, nil
}

func listPods(socket string, timeout time.Duration, maxSize int) (*podresourcesapi.ListPodResourcesResponse, error) {
	conn, err := connectToServer(socket, timeout, maxSize)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := podresourcesapi.NewPodResourcesListerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := client.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failure getting pod resources %v", err)
	}
	return resp, nil
}

func BuildConfig(kubeconfig string) (*rest.Config, error) {
	// Check if the kubeconfig file exists
	if _, err := os.Stat(kubeconfig); err == nil {
		// If the kubeconfig file exists, use it to build the configuration
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	// If the kubeconfig file does not exist, use InClusterConfig
	return rest.InClusterConfig()
}

func getNonCompletedAndNonFailedPods(node string) (map[string]*v1.Pod, error) {
	config, err := BuildConfig(Kubeconfig)
	if err != nil {
		fmt.Errorf("%v", err)
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Errorf("%v", err)
	}
	if err != nil {
		fmt.Errorf("%v", err)
	}
	// filter pods for this node.
	selector := fmt.Sprintf("spec.nodeName=%s", node)
	podListOptions := metav1.ListOptions{
		FieldSelector: selector,
	}
	podList, err := client.CoreV1().Pods("").List(context.Background(), podListOptions)
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %v", err)
	}
	filterPods := make(map[string]*v1.Pod)
	for _, pod := range podList.Items {
		if pod.Status.Phase != v1.PodFailed && pod.Status.Phase != v1.PodSucceeded {
			filterPods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = &pod
		} else {
			filterPods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = nil

		}
	}
	return filterPods, nil
}

func contains(set []string, target string) bool {
	for _, str := range set {
		if str == target {
			return true
		}
	}
	return false
}

func getDeviceToPodInfo(pods podresourcesapi.ListPodResourcesResponse, filteredPods map[string]*v1.Pod, resources []string) map[string]PodInfo {
	m := make(map[string]PodInfo)
	for _, pod := range pods.GetPodResources() {
		podIdentifier := fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())
		if filteredPods[podIdentifier] == nil {
			continue
		}
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if !contains(resources, device.GetResourceName()) {
					continue
				}
				podInfo := PodInfo{
					Pod:       pod.GetName(),
					Namespace: pod.GetNamespace(),
					Container: container.GetName(),
					UID:       string(filteredPods[podIdentifier].UID),
				}
				for _, uuid := range device.GetDeviceIds() {
					m[uuid] = podInfo
				}
			}
		}
	}
	return m
}

func (k *podResources) GetDeviceToPodInfo(node string) (map[string]PodInfo, error) {
	pods, err := listPods(k.socket, k.timeout, k.maxSize)
	if err != nil {
		return nil, fmt.Errorf("list pods: %v", err)
	}
	runningPods, err := getNonCompletedAndNonFailedPods(node)
	if err != nil {
		return nil, fmt.Errorf("failed to get running pods: %v", err)
	}
	info := getDeviceToPodInfo(*pods, runningPods, k.reources)
	return info, nil
}
