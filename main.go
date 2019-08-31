package main

import (
	"flag"
	"os"
	"path/filepath"

	"k8s.io/klog"
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
