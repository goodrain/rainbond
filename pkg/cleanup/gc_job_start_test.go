package cleanup

import (
	"context"
	"errors"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	typedbatch "k8s.io/client-go/kubernetes/typed/batch/v1"
)

type gcStartTestClient struct {
	typedbatch.JobInterface
	patches      int
	loseResponse bool
}

func (c *gcStartTestClient) Create(ctx context.Context, j *batchv1.Job, o metav1.CreateOptions) (*batchv1.Job, error) {
	copy := j.DeepCopy()
	if len(o.DryRun) > 0 {
		return copy, nil
	}
	copy.UID = "original"
	copy.ResourceVersion = "1"
	return c.JobInterface.Create(ctx, copy, o)
}
func (c *gcStartTestClient) Patch(ctx context.Context, name string, kind types.PatchType, data []byte, o metav1.PatchOptions, subresources ...string) (*batchv1.Job, error) {
	c.patches++
	job, err := c.JobInterface.Patch(ctx, name, kind, data, o, subresources...)
	if err == nil && c.loseResponse {
		return nil, errors.New("lost patch response")
	}
	return job, err
}

// capability_id: rainbond.cleanup.gc-job-start
func TestGCJobStartWaitsForWritersAndNeverReplaysLostStart(t *testing.T) {
	db, _ := coordinationDB(t)
	writer := operation("writer", "producer", "*")
	r := operation("gc-start", "gc", "*")
	if _, err := AcquireOperation(db, writer); err != nil {
		t.Fatal(err)
	}
	if _, err := RequestMaintenance(db, r); err != nil {
		t.Fatal(err)
	}
	client := &gcStartTestClient{JobInterface: fake.NewSimpleClientset().BatchV1().Jobs("system"), loseResponse: true}
	if _, err := SubmitSuspendedGCJob(context.Background(), db, client, r, suspendedGCFixture()); err != nil {
		t.Fatal(err)
	}
	if _, err := StartGCJob(context.Background(), db, client, r); !errors.Is(err, ErrCoordinationBusy) || client.patches != 0 {
		t.Fatal("started while writes active", err)
	}
	if err := FinishOperation(db, writer, true); err != nil {
		t.Fatal(err)
	}
	if _, err := StartGCJob(context.Background(), db, client, r); !errors.Is(err, ErrCoordinationUncertain) {
		t.Fatal(err)
	}
	job, err := StartGCJob(context.Background(), db, client, r)
	if err != nil || *job.Spec.Suspend || client.patches != 1 {
		t.Fatal("lost start was replayed", client.patches, err)
	}
	if state, err := InspectOperation(db, r); err != nil || state != "draining" {
		t.Fatal("starting Job granted native execution", state, err)
	}
}
func TestGCJobStartRejectsCanceledOperation(t *testing.T) {
	db, _ := coordinationDB(t)
	r := operation("gc-canceled", "gc", "*")
	if _, err := RequestMaintenance(db, r); err != nil {
		t.Fatal(err)
	}
	client := &gcStartTestClient{JobInterface: fake.NewSimpleClientset().BatchV1().Jobs("system")}
	if _, err := SubmitSuspendedGCJob(context.Background(), db, client, r, suspendedGCFixture()); err != nil {
		t.Fatal(err)
	}
	if err := CancelMaintenanceDrain(db, r); err != nil {
		t.Fatal(err)
	}
	if _, err := StartGCJob(context.Background(), db, client, r); err == nil || client.patches != 0 {
		t.Fatal("canceled operation started", err)
	}
}
