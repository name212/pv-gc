package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	toDeleteAnnotationKey = "pv-gc.flant.com/delete-after"
)

func patchPV(ctx context.Context, kubeCl kubernetes.Interface, pvName string, patch []byte) error {
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		return !errors.IsNotFound(err)
	}, func() error {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := kubeCl.CoreV1().PersistentVolumes().Patch(cctx, pvName, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	})
}

func addAnnotation(ctx context.Context, kubeCl kubernetes.Interface, pvName string, t time.Time) {
	patchObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				toDeleteAnnotationKey: t.Format(time.RFC3339),
			},
		},
	}

	patch, err := json.Marshal(patchObj)
	if err != nil {
		log.Errorf("Cannot marshal patch: %v", err)
		return
	}

	err = patchPV(ctx, kubeCl, pvName, patch)

	if err != nil {
		log.Errorf("Cannot add %s annotation to %s: %v", toDeleteAnnotationKey, pvName, err)
		return
	}

	log.Infof("Added %s annotation to %s", toDeleteAnnotationKey, pvName)
}

func deleteAnnotation(ctx context.Context, kubeCl kubernetes.Interface, pvName string) {
	patchObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				toDeleteAnnotationKey: nil,
			},
		},
	}

	patch, err := json.Marshal(patchObj)
	if err != nil {
		log.Errorf("Cannot marshal patch: %v", err)
		return
	}

	err = patchPV(ctx, kubeCl, pvName, patch)

	if err != nil {
		log.Errorf("Cannot delete annotation %s to %s: %v", toDeleteAnnotationKey, pvName, err)
		return
	}

	log.Infof("Annotation %s deleted from %s", toDeleteAnnotationKey, pvName)
}

func listPV(ctx context.Context, kubeCl kubernetes.Interface) ([]apiv1.PersistentVolume, error) {
	var pvs []apiv1.PersistentVolume
	err := retry.OnError(wait.Backoff{
		Steps:    5,
		Factor:   2.0,
		Jitter:   0.1,
		Duration: 1 * time.Second,
		Cap:      10 * time.Second,
	}, func(err error) bool {
		return true
	}, func() error {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		ps, err := kubeCl.CoreV1().PersistentVolumes().List(cctx, metav1.ListOptions{})
		pvs = ps.Items
		return err
	})

	if err != nil {
		return nil, err
	}

	return pvs, nil
}

func deletePV(ctx context.Context, kubeCl kubernetes.Interface, pvName string) {
	err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return !errors.IsNotFound(err)
	}, func() error {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		err := kubeCl.CoreV1().PersistentVolumes().Delete(cctx, pvName, metav1.DeleteOptions{})
		return err
	})

	if err != nil {
		log.Errorf("cannot delete pv %s: %v", pvName, err)
		return
	}

	log.Infof("PV %s deleted", pvName)
}
