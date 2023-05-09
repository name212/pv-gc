package main

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

type reconciller struct {
	kubeCl           kubernetes.Interface
	reconcileTime    time.Duration
	waitBeforeDelete time.Duration
}

func newReconciller(kubeCl kubernetes.Interface, reconcileTime time.Duration, waitBeforeDelete time.Duration) *reconciller {
	return &reconciller{
		kubeCl:           kubeCl,
		reconcileTime:    reconcileTime,
		waitBeforeDelete: waitBeforeDelete,
	}
}

func (r *reconciller) reconcile(ctx context.Context) {
	log.Infof("Start reconcile")
	defer log.Infof("Reconcile finished")

	pvs, err := listPV(ctx, r.kubeCl)
	if err != nil {
		log.Errorf("Error geting volumes: %v\n", err)
		return
	}

	if len(pvs) == 0 {
		log.Infof("Not found volumes. Skip")
		return
	}

	processor := newPVProcessor(time.Now())

	for _, p := range pvs {
		processor.process(p)
	}

	wg := sync.WaitGroup{}
	for _, p := range processor.toRemoveAnnotation {
		log.Infof("Should delete annotation on PV %s", p)
		pvName := p
		wg.Add(1)

		go func() {
			defer wg.Done()
			deleteAnnotation(ctx, r.kubeCl, pvName)
		}()
	}

	deleteTime := time.Now().Add(r.waitBeforeDelete)

	for _, p := range processor.toAddAnnotation {
		log.Infof("Should add annotation on PV %s", p)

		pvName := p
		wg.Add(1)

		go func() {
			defer wg.Done()
			addAnnotation(ctx, r.kubeCl, pvName, deleteTime)
		}()
	}

	for _, p := range processor.toDelete {
		log.Infof("Should delete PV %s", p)

		pvName := p
		wg.Add(1)

		go func() {
			defer wg.Done()
			deletePV(ctx, r.kubeCl, pvName)
		}()
	}

	wg.Wait()
}

func (r *reconciller) reconcileLoop(ctx context.Context, doneCh chan struct{}) {
	r.reconcile(ctx)

	tk := time.NewTicker(r.reconcileTime)
	for {
		select {
		case <-tk.C:
			r.reconcile(ctx)
		case <-ctx.Done():
			doneCh <- struct{}{}
			return
		}
	}
}
