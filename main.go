package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"os/user"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pborman/uuid"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

type Config struct {
	Time           uint
	ConCurrencyNum uint

	Interval uint
}

var config Config

func i64(i int) *int64 {
	j := int64(i)
	return &j
}

func init() {
	rand.Seed(time.Now().UnixNano())

	flag.UintVar(&config.Time, "time", 10, "run time duration(second)")
	flag.UintVar(&config.ConCurrencyNum, "concurrency", 10, "concurrency number")

	flag.UintVar(&config.Interval, "interval", 10, "run interval")
	flag.Parse()
}

func work(i int, pods v1.PodInterface, namespace string, wg *sync.WaitGroup, sec time.Duration, num *int32, timee *int64) {
	defer wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), sec)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			log.Printf("Goroutine %d done, returning", i)
			return
		default:
		}
		podName := fmt.Sprintf("work-pod-%s", uuid.New())
		pod := &api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
				Labels: map[string]string{
					"work": "work-pod",
				},
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					api.Container{
						Name:            "alpine-echo",
						Image:           "alpine:3.3",
						Command:         []string{"echo", fmt.Sprintf(`"hello k8stress pod %s"`, podName)},
						ImagePullPolicy: api.PullIfNotPresent,
					},
				},
				SchedulerName: "mock",
			},
		}
		start := time.Now()
		_, err := pods.Create(context.Background(), pod, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error creating pod #%d %s (%s)", i, podName, err)
			return
		}
		//log.Printf("New pod %s created: %+v", podName, *newPod)
		//log.Printf("New pod %s created", podName)
		//log.Printf("Deleting pod %s", podName)
		if err := pods.Delete(context.Background(), podName, metav1.DeleteOptions{GracePeriodSeconds: i64(0)}); err != nil {
			log.Printf("Error deleting pod #%d %s (%s)", i, podName, err)
		}

		log.Printf("延迟时间: i = %d, %+v\n", i, time.Since(start))
		atomic.AddInt32(num, 2)
		atomic.AddInt64(timee, time.Since(start).Milliseconds())
		//time.Sleep(5 * time.Millisecond)
	}
}

func main() {
	// 读取配置文件
	home := GetHomePath()
	k8sConfig, err := clientcmd.BuildConfigFromFlags("", fmt.Sprintf("%s/.kube/config", home)) // 使用 kubectl 默认配置 ~/.kube/config
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	k8sConfig.Burst = 1e6
	k8sConfig.QPS = 1e6
	// 创建一个k8s客户端
	clientSet, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	signalChan := make(chan os.Signal, 2)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-signalChan:
			log.Println("signal quit")
			return
		case <-time.After(time.Duration(config.Interval) * time.Millisecond):
		}

		var (
			num  int32
			timeDuration int64
			wg   = &sync.WaitGroup{}
		)

		for i := 0; i < int(config.ConCurrencyNum); i++ {
			wg.Add(1)
			go work(i, clientSet.CoreV1().Pods("evanlixin"), "evanlixin", wg, time.Duration(config.Time) * time.Second, &num, &timeDuration)
		}

		log.Printf("Done creating %d goroutines", 1)
		wg.Wait()
		log.Printf("%v\n", num)
		log.Printf("Done")
		log.Printf("\n\n\n\n\n")
	}
}

func GetHomePath() string {
	u, err := user.Current()
	if err == nil {
		return u.HomeDir
	}
	return ""
}
