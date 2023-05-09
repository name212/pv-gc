package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func initConfig(kubeconfig string) kubernetes.Interface {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	return c
}

func getHTTPServer(bind string) *http.Server {
	router := http.NewServeMux()
	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`Index`))
	})

	return &http.Server{Addr: bind, Handler: router, ReadHeaderTimeout: 30 * time.Second}
}

func main() {
	kubeconfig := flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	reconcileTime := flag.Duration("reconcile-time", 5*time.Minute, "Reconcile time")
	waitBeforeDeleteTime := flag.Duration("wait-before-delete-time", 5*time.Minute, "Wait before delete pv")
	bindAddress := flag.String("bind-address", "9090", "Health check server bind address")

	flag.Parse()

	kubeCl := initConfig(*kubeconfig)
	httpServer := getHTTPServer(*bindAddress)

	rootCtx, cancel := context.WithCancel(context.Background())

	doneCh := make(chan struct{})

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("Signal received: %v. Exiting.\n", <-signalChan)
		cancel()
		fmt.Printf("Waiting for stop reconcile loop...")
		<-doneCh

		ctx, ccancel := context.WithTimeout(rootCtx, 10*time.Second)
		defer ccancel()

		fmt.Printf("Shutdown ...")

		err := httpServer.Shutdown(ctx)
		if err != nil {
			fmt.Printf("Error occurred while closing the server: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	r := newReconciller(kubeCl, *reconcileTime, *waitBeforeDeleteTime)

	go r.reconcileLoop(rootCtx, doneCh)

	err := httpServer.ListenAndServe()
	if err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
