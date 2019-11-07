package controllers

import (
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/moshloop/platform-operator/pkg/k8s"
)

func CleanupOperator(client k8s.Client, watchDuration time.Duration) {

	for {
		clientset, err := client.GetClientset()

		if err != nil {
			log.Errorf("Failed to get kubecluent %v", err)
		}

		list, err := clientset.CoreV1().Namespaces().List(metav1.ListOptions{})
		if err != nil {
			log.Errorf("Failed to list namespaces %v", err)
		}
		for _, ns := range list.Items {
			expiry, ok := ns.Labels["auto-delete"]
			if !ok {
				continue
			}
			if ns.Status.Phase == v1.NamespaceTerminating {
				log.Debugf("ns/%s is already terminating", ns.Name)
				continue
			}
			duration, err := time.ParseDuration(expiry)
			if err != nil {
				log.Errorf("Invalid duration for namespace %s: %s", ns.Name, expiry)
				continue
			}
			expiresOn := ns.GetCreationTimestamp().Add(duration)
			log.Infof("Processing %s with expiry of %v due to expire in (%v),", ns.Name, expiry, duration-time.Now().Sub(expiresOn))

			if expiresOn.Before(time.Now()) {
				log.Infof("Deleting namespace %s", ns.Name)
				if err := clientset.CoreV1().Namespaces().Delete(ns.Name, nil); err != nil {
					log.Errorf("Failed to delete namespace %s: %v", ns.Name, err)
				}
			}
		}
		time.Sleep(watchDuration)
	}
}
