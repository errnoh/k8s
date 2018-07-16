package watch

import (
	"fmt"
	"sync"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	watchchan = make(chan Event)
	closechan = make(chan struct{})
	wg        = new(sync.WaitGroup)
	clients   []*client
	running   = true
)

type client struct {
	client     *kubernetes.Clientset
	namespaces []string
	metadata   Metadata
}

func Register(identifier string, cl *kubernetes.Clientset, namespace ...string) {
	if len(namespace) == 0 || (len(namespace) == 1 && len(namespace[0]) == 0) {
		glog.Fatal("Trying to start watcher with 0 namespaces")
		// TODO: Maybe default to "default" (or even better: own namespace?)
	}
	c := &client{cl, make([]string, 0, 10), Metadata{Identifier: identifier}}
	if namespaces, err := c.client.CoreV1().Namespaces().List(metav1.ListOptions{}); err == nil {
	loop:
		for _, ns := range namespace {
			for _, item := range namespaces.Items {
				if ns == item.Name {
					c.namespaces = append(c.namespaces, ns)
					continue loop
				}
			}
			glog.Warningf("Couldn't find namespace '%s'", ns)
		}
	} else {
		glog.Fatalf("Unable to list namespaces: %s", err)
	}
	clients = append(clients, c)
}

func Channel() <-chan Event {
	return (<-chan Event)(watchchan)
}

type Event struct {
	Event    watch.Event
	Metadata Metadata
}

type Metadata struct {
	Function, Identifier, Namespace string
}

type ErrWatcher Metadata

func (err ErrWatcher) Error() string {
	return fmt.Sprintf("%s/%s/%s: Watcher stopped", err.Function, err.Identifier, err.Namespace)
}

func (err ErrWatcher) GetObjectKind() schema.ObjectKind {
	return nil
}

func (err ErrWatcher) DeepCopyObject() runtime.Object {
	return err
}

type watcherFn func(*client, string, metav1.ListOptions) (watch.Interface, error)

func DaemonSets(opts metav1.ListOptions) {
	daemonSets := func(w *client, ns string, opts metav1.ListOptions) (watch.Interface, error) {
		return w.client.ExtensionsV1beta1().DaemonSets(ns).Watch(opts)
	}

	watchers(opts, "DaemonSets", daemonSets)
}

func Deployments(opts metav1.ListOptions) {
	deployments := func(w *client, ns string, opts metav1.ListOptions) (watch.Interface, error) {
		return w.client.ExtensionsV1beta1().Deployments(ns).Watch(opts)
	}

	watchers(opts, "Deployments", deployments)
}

func Ingresses(opts metav1.ListOptions) {
	ingresses := func(w *client, ns string, opts metav1.ListOptions) (watch.Interface, error) {
		return w.client.ExtensionsV1beta1().Ingresses(ns).Watch(opts)
	}

	watchers(opts, "Ingresses", ingresses)
}

func PodSecurityPolicies(opts metav1.ListOptions) {
	podSecurityPolicies := func(w *client, _ string, opts metav1.ListOptions) (watch.Interface, error) {
		return w.client.ExtensionsV1beta1().PodSecurityPolicies().Watch(opts)
	}

	watchers(opts, "PodSecurityPolicies", podSecurityPolicies)
}

func ReplicaSets(opts metav1.ListOptions) {
	replicaSets := func(w *client, ns string, opts metav1.ListOptions) (watch.Interface, error) {
		return w.client.ExtensionsV1beta1().ReplicaSets(ns).Watch(opts)
	}

	watchers(opts, "ReplicaSets", replicaSets)
}

func watchers(opts metav1.ListOptions, metadataFunction string, fn watcherFn) {
	for _, w := range clients {
		for _, ns := range w.namespaces {
			go func(w *client, ns string) {
				// watch.Interface
				if watcher, err := fn(w, ns, opts); err == nil { // w.client.ExtensionsV1beta1().Deployments(ns).Watch(opts); err == nil {
					wg.Add(1)
					metadata := Metadata{
						Function:   metadataFunction,
						Identifier: w.metadata.Identifier,
						Namespace:  ns,
					}
				loop:
					for {
						select {
						case <-closechan:
							watcher.Stop()
							for _ = range watcher.ResultChan() {
							}
							break loop
						case v, ok := <-watcher.ResultChan():
							if ok {
								watchchan <- Event{Event: v, Metadata: metadata}
							} else {
								break loop
							}
						}
					}
					glog.Errorf("Watcher for namespace '%s' exited", ns)
					watchchan <- Event{Event: watch.Event{Type: "Error", Object: ErrWatcher(metadata)}, Metadata: metadata}
					wg.Done()
					return
				} else {
					glog.Warningf("Unable to create watcher for namespace '%s': %s", ns, err)
				}
			}(w, ns)
		}
	}
}

type CloseComplete struct{}

func (cc CloseComplete) GetObjectKind() schema.ObjectKind {
	return nil
}

func (cc CloseComplete) DeepCopyObject() runtime.Object {
	return cc
}

func Close() {
	if running {
		glog.Warningf("Shutting down active listeners")
		close(closechan)
		wg.Wait()
		running = false
		watchchan <- Event{Event: watch.Event{Type: "Error", Object: CloseComplete{}}, Metadata: Metadata{Function: "Close"}}
	}
}

func ClientFromCluster() (client *kubernetes.Clientset, err error) {
	var kubeconfig *rest.Config

	if kubeconfig, err = rest.InClusterConfig(); err != nil {
		glog.Error(err)
		return
	}
	client, err = clientFrom(kubeconfig)
	return
}

func ClientFromFile(masterURL, kubeconfigPath string) (client *kubernetes.Clientset, err error) {
	var kubeconfig *rest.Config
	// TODO: Check if both variables are needed or is it ok to leave one empty?
	//       Also a bit messy, maybe drop the override param
	kubeconfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{
				Server: masterURL,
			},
		}).ClientConfig()
	if err != nil {
		glog.Error(err)
		return
	}
	if masterURL != "" {
		kubeconfig.Host = masterURL
	}
	client, err = clientFrom(kubeconfig)
	return
}

func clientFrom(cfg *rest.Config) (client *kubernetes.Clientset, err error) {
	var v *version.Info

	// defaults are rather low, these are from prometheus while for example nginx uses even higher values.
	cfg.QPS = 100
	cfg.Burst = 100
	cfg.ContentType = "application/vnd.kubernetes.protobuf"

	glog.Infof("Creating API client for %s", cfg.Host)

	if client, err = kubernetes.NewForConfig(cfg); err != nil {
		return
	}

	if v, err = client.Discovery().ServerVersion(); err != nil {
		return
	}

	glog.Infof("Kubernetes %v.%v (%v) - git (%v) commit %v - platform %v",
		v.Major, v.Minor, v.GitVersion, v.GitTreeState, v.GitCommit, v.Platform)

	return
}
