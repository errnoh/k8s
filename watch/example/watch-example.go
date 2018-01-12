package main

import (
	"flag"
	"strings"
	"time"

	watch "github.com/errnoh/k8s/watch"
	"github.com/golang/glog"
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
	watch.Deployments(options)

	go func() {
		for item := range watch.Channel() {
			switch val := item.Event.Object.(type) {
			case *extv1beta1.Deployment:
				for _, container := range val.Spec.Template.Spec.Containers {
					glog.Infof("[%s] Deployment %s container %s is running image %s", val.Namespace, val.Name, container.Name, container.Image)

				}
			}
		}
		glog.Error("Listener goroutine exited")
	}()

	time.Sleep(time.Minute)
	watch.Close()
	glog.Info("Stopping")
}
