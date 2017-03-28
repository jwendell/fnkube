package fnkube

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/golang/glog"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	batch "k8s.io/client-go/pkg/apis/batch/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"math"

	"github.com/jwendell/fnkube/pkg/rand"
)

// AuthInfo holds...
type AuthInfo struct {
	MasterURL string
	Username  string
	Password  string
	Token     string
	Insecure  bool
}

// Options holds...
type Options struct {
	Auth       AuthInfo
	ConfigFile string
	Namespace  string
	Image      string
	Command    []string
	Timeout    int
	Cleanup    bool
}

type s1 struct {
	options          *Options
	client           *kubernetes.Clientset
	jobName          string
	createdNamespace bool
	stdout           string
	stderr           string
}

const fnkubePrefix = "fnkube"

// Run dfdfkj
func Run(options *Options) (stdout, stderr string, err error) {
	s := &s1{options: options}
	err = s.run()

	return s.stdout, s.stderr, err
}

func (s *s1) run() (err error) {
	err = s.validateConfig()
	if err != nil {
		return
	}

	err = s.createClient()
	if err != nil {
		return
	}

	err = s.validateNamespace()
	if err != nil {
		return
	}

	err = s.createJob()
	if err != nil {
		return
	}

	err = s.watch()
	if err != nil {
		return
	}

	return
}

func defaultConfigFile() string {
	filename := path.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(filename); err == nil {
		glog.Infof("Using %s as ConfigFile", filename)
		return filename
	}

	return ""
}

func (s *s1) validateNamespace() error {
	if len(s.options.Namespace) == 0 {
		s.options.Namespace = fmt.Sprintf("%s-ns-%s", fnkubePrefix, rand.RandomString(5))
		s.createdNamespace = true
		glog.Infof("No namespace provided, attempting to create a new one called %s\n", s.options.Namespace)
	} else {
		if _, err := s.client.Namespaces().Get(s.options.Namespace); err != nil {
			glog.Infof("Provided namespace %s does not exist. Attempting to create it.", s.options.Namespace)
			s.createdNamespace = true
		}
	}

	if s.createdNamespace {
		_, err := s.client.Namespaces().Create(&v1.Namespace{ObjectMeta: v1.ObjectMeta{Name: s.options.Namespace}})
		if err != nil {
			return fmt.Errorf("Error creating namespace %s: %s", s.options.Namespace, err)
		}
	}

	return nil
}

func (s *s1) validateConfig() error {
	if len(s.options.ConfigFile) == 0 && len(s.options.Auth.MasterURL) == 0 {
		s.options.ConfigFile = defaultConfigFile()
		if len(s.options.ConfigFile) == 0 {
			return errors.New("ConfigFile or MasterURL must be provided")
		}
	}

	if s.options.Timeout == 0 {
		s.options.Timeout = math.MaxInt32
	}

	return nil
}

func (s *s1) createClient() error {
	config, err := clientcmd.BuildConfigFromFlags(s.options.Auth.MasterURL, s.options.ConfigFile)
	if err != nil {
		return err
	}
	config.Insecure = s.options.Auth.Insecure

	s.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error creating the client: %s", err)
	}

	return nil
}

func (s *s1) createJob() error {
	glog.V(1).Infoln("Creating job")

	suffix := rand.RandomString(5)
	s.jobName = fmt.Sprintf("%s-job-%s", fnkubePrefix, suffix)

	j := &batch.Job{
		ObjectMeta: v1.ObjectMeta{Name: s.jobName},
		Spec: batch.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:    fmt.Sprintf("%s-container-%s", fnkubePrefix, suffix),
						Image:   s.options.Image,
						Command: s.options.Command,
					}},
					RestartPolicy: "Never",
				},
			},
		},
	}
	_, err := s.client.Batch().Jobs(s.options.Namespace).Create(j)
	return err
}

func jobCompleted(job *batch.Job) bool {
	return job.Status.Succeeded > 0
}

func (s *s1) watch() error {
	stop := make(chan struct{})
	watchlist := cache.NewListWatchFromClient(s.client.Batch().RESTClient(), "jobs", s.options.Namespace,
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&batch.Job{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(oldObj, newObj interface{}) {
				job := newObj.(*batch.Job)
				if jobCompleted(job) {
					stop <- struct{}{}
				}
			},
		},
	)

	glog.V(1).Infoln("Waiting for the job to complete")
	go controller.Run(stop)

	select {
	case <-stop:
		s.output()
	case <-time.After(time.Duration(s.options.Timeout) * time.Second):
		stop <- struct{}{}
		glog.Errorf("Timeout while waiting for the Job to be completed\n")
	}
	return s.deleteStuff()
}

func (s *s1) output() error {
	options := v1.ListOptions{LabelSelector: "job-name=" + s.jobName}
	pods, err := s.client.Pods(s.options.Namespace).List(options)

	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		err = s.stdOut(pod)
		if err != nil {
			return fmt.Errorf("Error getting log for pod %s: %s", pod.ObjectMeta.Name, err)
		}
	}
	return nil
}

func (s *s1) stdOut(pod v1.Pod) error {
	r := s.client.Pods(s.options.Namespace).GetLogs(pod.ObjectMeta.Name, &v1.PodLogOptions{})
	readCloser, err := r.Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(readCloser)
	if err != nil {
		return err
	}

	s.stdout = buf.String()
	return nil
}

func (s *s1) deleteStuff() error {
	if !s.options.Cleanup {
		glog.Infoln("Ignoring cleanup upon request")
		return nil
	}

	glog.V(1).Infoln("Cleaning stuff up...")
	err := s.client.Batch().Jobs(s.options.Namespace).Delete(s.jobName, &v1.DeleteOptions{})
	if err != nil {
		return err
	}

	options := v1.ListOptions{LabelSelector: "job-name=" + s.jobName}
	pods, err := s.client.Pods(s.options.Namespace).List(options)
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err := s.client.Pods(s.options.Namespace).Delete(pod.ObjectMeta.Name, &v1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	if s.createdNamespace {
		err := s.client.Namespaces().Delete(s.options.Namespace, &v1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
