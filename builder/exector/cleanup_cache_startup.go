package exector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Called before task discovery. A restarted process cannot inherit a previous
// instance's readiness, even if its cache mount did not change.
func registerCacheBuilderStartup(ctx context.Context, database *gorm.DB, client kubernetes.Interface, namespace, podName string) error {
	if database == nil {
		return guard.ErrCoordinationUnavailable
	}
	stores, err := guard.DiscoverStores(database)
	if err != nil {
		return err
	}
	bindings := []guard.StorageRegistration{}
	for _, store := range stores {
		binding, err := guard.StorageBinding(database, store.StorageID, store.Generation)
		if err != nil {
			return err
		}
		domain := sha256.Sum256([]byte("managed-build-cache\x00" + binding.VolumeUID))
		if binding.RootPath != "/cache/build" || binding.StorageID != hex.EncodeToString(domain[:]) {
			continue
		}
		if err := guard.WithdrawManagedCacheReadiness(database, binding); err != nil {
			return err
		}
		bindings = append(bindings, binding)
	}
	if len(bindings) == 0 {
		return nil
	}
	unverified := func() error {
		ids := make([]string, 0, len(bindings))
		for _, binding := range bindings {
			ids = append(ids, binding.StorageID)
		}
		var active int64
		if err := database.Model(&model.CleanupOperation{}).Where("storage_id IN (?) AND state <> ? AND kind <> ?", ids, "finished", "producer").Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return guard.ErrCoordinationBusy
		}
		logrus.Warn("Cache cleanup remains unverified; startup has withdrawn deletion readiness")
		return nil
	}
	if client == nil {
		return unverified()
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return unverified()
	}
	observed, err := kubeidentity.InspectManagedBuildCache(ctx, client, namespace, podName, string(pod.UID))
	if err != nil {
		return unverified()
	}
	for _, binding := range bindings {
		if binding.VolumeUID != observed.Mount.VolumeUID {
			continue
		}
		participant, err := kubeidentity.InspectCacheBuilder(ctx, client, namespace, podName, string(pod.UID), binding)
		if err != nil {
			return err
		}
		if err := guard.RegisterParticipant(database, participant); err != nil {
			return err
		}
	}
	return nil
}
