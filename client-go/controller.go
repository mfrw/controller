package main

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	cnatv1alpha "github.com/mfrw/controller/pkg/apis/cnat/v1alpha1"
	clientset "github.com/mfrw/controller/pkg/generated/clientset/versioned"
	cnatscheme "github.com/mfrw/controller/pkg/generated/clientset/versioned/scheme"
	informers "github.com/mfrw/controller/pkg/generated/informers/externalversions/cnat/v1alpha1"
	listers "github.com/mfrw/controller/pkg/generated/listers/cnat/v1alpha1"
)

const controllerAgentName = "cnat-controller"

type Controller struct {
	kubeClientset kubernetes.Interface
	cnatClientset clientset.Interface

	atLister  listers.AtLister
	atsSynced cache.InformerSynced

	podLister  corev1lister.PodLister
	podsSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
	recorder  record.EventRecorder
}

func NewController(
	kubeClientset kubernetes.Interface,
	cnatClientset clientset.Interface,
	atInformer informers.AtInformer,
	podInformer corev1informer.PodInformer,

) *Controller {
	utilruntime.Must(cnatscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event boradcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeClientset: kubeClientset,
		cnatClientset: cnatClientset,
		atLister:      atInformer.Lister(),
		atsSynced:     atInformer.Informer().HasSynced,
		podLister:     podInformer.Lister(),
		podsSynced:    podInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Ats"),
		recorder:      recorder,
	}

	klog.Info("Setting up event handlers")

	atInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueAt,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueAt(new)
		},
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueuePod(new)
		},
	})
	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.atsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced); !ok {
		return fmt.Errorf("failed to wait for cached to sync")
	}

	klog.Info("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")
	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		if when, err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requening", key, err.Error())
		} else if when != time.Duration(0) {
			c.workqueue.AddAfter(key, when)
		} else {
			c.workqueue.Forget(obj)
		}
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

func (c *Controller) syncHandler(key string) (time.Duration, error) {
	klog.Infof("==== Reconciling At %s", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return time.Duration(0), nil
	}

	instance, err := c.atLister.Ats(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("at '%s' in work queue no longer exists", key))
			return time.Duration(0), nil
		}
		return time.Duration(0), err
	}

	if instance.Status.Phase == "" {
		instance.Status.Phase = cnatv1alpha.PhasePending
	}

	// State Machine for PENDING -> RUNNING -> DONE

	switch instance.Status.Phase {
	case cnatv1alpha.PhasePending:
		klog.Infof("instance %s: phase=PENDING", key)
		return time.Duration(0), nil
	case cnatv1alpha.PhaseRunning:
		klog.Infof("instance %s: phase=RUNNING", key)
		return time.Duration(0), nil
	case cnatv1alpha.PhaseDone:
		klog.Infof("instance %s: phase=DONE", key)
		return time.Duration(0), nil
	default:
		klog.Infof("instance %s: NOP")
		return time.Duration(0), nil
	}

	_, err = c.cnatClientset.CnatV1alpha1().Ats(instance.Namespace).UpdateStatus(instance)
	if err != nil {
		return time.Duration(0), err
	}
	return time.Duration(0), nil
}

func (c *Controller) enqueueAt(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) enqueuePod(obj interface{}) {
	var pod *corev1.Pod
	var ok bool
	if pod, ok = obj.(*corev1.Pod); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding pod, invalid type"))
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding pod tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted pod '%s' from tombstone", pod.GetName())
	}

	if ownerRef := metav1.GetControllerOf(pod); ownerRef != nil {
		if ownerRef.Kind != "At" {
			return
		}
		at, err := c.atLister.Ats(pod.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned pod '%s' of At '%s'", pod.GetSelfLink(), ownerRef.Name)
			return
		}
		klog.Infof("enqueuing At %s/%s because pod changed", at.Namespace, at.Name)
		c.enqueueAt(at)
	}
}

func timeUntilSchedule(schedule string) (time.Duration, error) {
	now := time.Now().UTC()
	layout := "2006-01-02T15:04:05Z"
	s, err := time.Parse(layout, schedule)
	if err != nil {
		return time.Duration(0), err
	}
	return s.Sub(now), nil
}

func newPodForCR(cr *cnatv1alpha.At) *corev1.Pod {
	labels := map[string]string{
		"app": cr.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-pod",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: strings.Split(cr.Spec.Command, " "),
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
}
