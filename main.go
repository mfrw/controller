package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	clientset "github.com/mfrw/controller/pkg/generated/clientset/versioned"
	informers "github.com/mfrw/controller/pkg/generated/informers/externalversions"
)

var (
	masterURL  string
	kubeconfig string
)

func main() {
	flag.StringVar(&kubeconfig, "kubeconfig", defaultKubeconfig(), "Path to kubeconfig")
	flag.StringVar(&masterURL, "master", "", "Addr of k8s API Server")

	klog.InitFlags(nil)

	flag.Parse()

	cfg, err := rest.InClusterConfig()
	if err != nil {
		cfg, err = clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
		if err != nil {
			klog.Fatalf("Error building kubeconfig: %s", err.Error())
		}
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	cnatClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building cnat clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*10)
	cnatInformerFactory := informers.NewSharedInformerFactory(cnatClient, time.Minute*10)

	controller := NewController(kubeClient, cnatClient, cnatInformerFactory.Cnat().V1alpha1().Ats(), kubeInformerFactory.Core().V1().Pods())

	kubeInformerFactory.Start(wait.NeverStop)
	cnatInformerFactory.Start(wait.NeverStop)

	if err := controller.Run(2, wait.NeverStop); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}

}

func defaultKubeconfig() string {
	fname := os.Getenv("KUBECONFIG")
	if fname != "" {
		return fname
	}
	home, err := os.UserHomeDir()
	if err != nil {
		klog.Warning("failed to get home directory: %v", err)
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}
