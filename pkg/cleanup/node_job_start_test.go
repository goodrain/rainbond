package cleanup

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db/model"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.cleanup.node-job-start
func TestNodeJobStartChecksProtectionAndReconcilesLostReply(t *testing.T) {
	database, _ := coordinationDB(t)
	binding, err := ProvisionManagedCacheStorage(database, "cache", "/cache/build")
	if err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready")
	intent := NodeJobIntent{Namespace: "system", NodeName: "node", NodeUID: "uid", Entry: "entry", Fingerprint: strings.Repeat("a", 64)}
	r := operation("start-node", "delete", nodeJobScope(intent))
	r.StorageID = binding.StorageID
	r.Generation = binding.Generation
	r.Target = nodeJobTarget(intent)
	if _, err := AcquireOperation(database, r); err != nil {
		t.Fatal(err)
	}
	template := suspendedGCFixture()
	template.Spec.Template.Spec.NodeName = "node"
	template.Spec.Template.Spec.Containers[0].Command = []string{"/app/node-cleanup"}
	client := &gcStartTestClient{JobInterface: fake.NewSimpleClientset().BatchV1().Jobs("system"), loseResponse: true}
	if _, err := SubmitSuspendedNodeJob(context.Background(), database, client, r, intent, template); err != nil {
		t.Fatal(err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "collecting")
	if _, err := StartNodeJob(context.Background(), database, client, r); !errors.Is(err, ErrCoordinationBusy) || client.patches != 0 {
		t.Fatal("unready storage started", err)
	}
	database.Model(&model.CleanupStorage{}).Where("storage_id = ?", binding.StorageID).Update("mode", "ready")
	writer := model.CleanupOperation{OperationID: "late-writer", StorageID: r.StorageID, Generation: r.Generation, Owner: "writer", Kind: "producer", Scope: "*", Fingerprint: "late", State: "active"}
	if err := database.Create(&writer).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := StartNodeJob(context.Background(), database, client, r); !errors.Is(err, ErrCoordinationBusy) || client.patches != 0 {
		t.Fatal("active writer ignored", err)
	}
	database.Delete(&writer)
	if _, err := StartNodeJob(context.Background(), database, client, r); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal(err)
	}
	job, err := StartNodeJob(context.Background(), database, client, r)
	if err != nil || *job.Spec.Suspend || client.patches != 1 {
		t.Fatal("lost start replayed", client.patches, err)
	}
	if state, err := InspectOperation(database, r); err != nil || state != "active" {
		t.Fatal("job start granted deletion", state, err)
	}
}
