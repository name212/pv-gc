package main

import (
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
)

type pvProcessor struct {
	toRemoveAnnotation []string
	toAddAnnotation    []string
	toDelete           []string
	now                time.Time
}

func newPVProcessor(now time.Time) *pvProcessor {
	return &pvProcessor{
		toRemoveAnnotation: make([]string, 0),
		toAddAnnotation:    make([]string, 0),
		toDelete:           make([]string, 0),
		now:                now,
	}
}

func (v *pvProcessor) process(p apiv1.PersistentVolume) {
	log.Infof("Start rocess PV: %s", p.GetName())
	defer log.Infof("End process PV: %s", p.GetName())

	if p.Spec.PersistentVolumeReclaimPolicy != apiv1.PersistentVolumeReclaimRetain {
		log.Infof("PV %s can not be retained. Skip", p.GetName())
		return
	}

	// pv already started to delete
	if !p.DeletionTimestamp.IsZero() {
		log.Infof("PV %s already started deleting. Skip", p.GetName())
		return
	}

	markToDeleteTimeStr := p.GetAnnotations()[toDeleteAnnotationKey]
	log.Infof("PV %s in phase %s; annotation %s=%s", p.GetName(), p.Status.Phase, toDeleteAnnotationKey, markToDeleteTimeStr)
	switch p.Status.Phase {
	case apiv1.VolumeReleased:
		if markToDeleteTimeStr == "" {
			v.toAddAnnotation = append(v.toAddAnnotation, p.GetName())
			log.Infof("PV %s was released. Add to mark for deletion", p.GetName())
		} else {
			t, err := time.Parse(time.RFC3339, markToDeleteTimeStr)
			if err != nil {
				log.Errorf("Incorrect time value for annotation key %s: %s. Error: %v", toDeleteAnnotationKey, markToDeleteTimeStr, err)
				v.toAddAnnotation = append(v.toAddAnnotation, p.GetName())
				return
			}

			if v.now.After(t) {
				log.Infof("PV %s was released. Add for delete PV! nowTime=%s %s", p.GetName(), v.now, markToDeleteTimeStr)
				v.toDelete = append(v.toDelete, p.GetName())
			}
		}
	case apiv1.VolumeBound:
		if markToDeleteTimeStr != "" {
			log.Infof("PV %s is bound and previusly marked for deletion in %s. Add to unmark for deletion", p.GetName(), markToDeleteTimeStr)
			v.toRemoveAnnotation = append(v.toRemoveAnnotation, p.GetName())
		}
	}
}
