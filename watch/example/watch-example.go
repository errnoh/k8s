package main

import (
	"flag"
	"strings"
	"time"

	watch "github.com/errnoh/k8s/watch"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	conf_name       = flag.String("name", "watch-example", "Name of this controller")
	conf_namespaces = flag.String("namespace", "default", "Namespace to follow")
	conf_selector   = flag.String("selector", "", "Label selector, defaults to everything")

	conf_apiserver  = flag.String("apiserver", "", "Override apiserver, leave empty if running in cluster")
	conf_kubeconfig = flag.String("kubeconfig", "", "Use custom kubeconfig, leave empty if running in cluster")
)

type Client *kubernetes.Clientset

func main() {
	flag.Parse()
	glog.Info("Starting")

	var (
		kubeClient *kubernetes.Clientset
		err        error
		options    metav1.ListOptions
	)

	if *conf_selector != "" {
		options = metav1.ListOptions{LabelSelector: *conf_selector}
	}

	if *conf_apiserver == "" && *conf_kubeconfig == "" {
		kubeClient, err = watch.ClientFromCluster()
	} else {
		kubeClient, err = watch.ClientFromFile(*conf_apiserver, *conf_kubeconfig)
	}

	if err != nil {
		glog.Fatalf("Error when trying to create client: %s", err)
	}

	watch.Register("local", kubeClient, strings.Split(*conf_namespaces, ",")...)
	watch.DaemonSets(options)
	watch.Deployments(options)
	watch.Ingresses(options)
	watch.PodSecurityPolicies(options)
	watch.ReplicaSets(options)
	watch.ComponentStatuses(options)
	watch.ConfigMaps(options)
	watch.Endpoints(options)
	watch.LimitRanges(options)
	watch.Namespaces(options)
	watch.Nodes(options)
	watch.PersistentVolumeClaims(options)
	watch.PersistentVolumes(options)
	watch.Pods(options)
	watch.ServiceAccounts(options)
	watch.Services(options)

	go func() {
		for item := range watch.Channel() {
			switch val := item.Event.Object.(type) {
			case *extv1beta1.Deployment:
				for _, container := range val.Spec.Template.Spec.Containers {
					glog.Infof("[%s] Deployment %s container %s is running image %s", val.Namespace, val.Name, container.Name, container.Image)

				}
			case *extv1beta1.DaemonSet:
				glog.Info("DaemonSet")
			case *extv1beta1.Ingress:
				glog.Info("Ingresses")
			case *extv1beta1.PodSecurityPolicy:
				glog.Info("PodSecurityPolicies")
			case *extv1beta1.ReplicaSet:
				glog.Info("ReplicaSets")
			case *corev1.ComponentStatus:
				glog.Info("ComponentStatus")
			case *corev1.ConfigMap:
				glog.Info("ConfigMap")
			case *corev1.Endpoints:
				glog.Info("Endpoints")
			case *corev1.LimitRange:
				glog.Info("LimitRange")
			case *corev1.Namespace:
				glog.Info("Namespace")
			case *corev1.Node:
				glog.Infof("Node %s %s: %s", val.Spec.PodCIDR, val.Spec.ProviderID, val.Status.String())
			case *corev1.PersistentVolumeClaim:
				glog.Info("PersistentVolumeClaim")
			case *corev1.PersistentVolume:
				glog.Info("PersistentVolume")
			case *corev1.Pod:
				glog.Info("Pod")
			case *corev1.ServiceAccount:
				glog.Info("ServiceAccount")
			case *corev1.Service:
				glog.Info("Service")
			case watch.ErrWatcher:
				glog.Error(val.Error())
				// One might for example want to watch.Close() here, or try to recreate the watcher.
			case watch.CloseComplete:
				glog.Info("Finished closing all listeners")
			default:
				glog.Info("default")
			}
		}
		glog.Error("Listener goroutine exited")
	}()

	time.Sleep(time.Second * 30)
	watch.Close()
	time.Sleep(time.Second * 5)
	glog.Info("Stopping")
}
