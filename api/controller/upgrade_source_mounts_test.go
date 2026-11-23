package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/goodrain/rainbond-operator/api/v1alpha1"
	httputil "github.com/goodrain/rainbond/util/http"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpgradeSourceMountsBlockWholeRequest(t *testing.T) {
	for _, image := range []string{"example.invalid/rainbond:new", "example.invalid/rainbond:old"} {
		t.Run(image, func(t *testing.T) {
			console := upgradeSourceComponent("rbd-app-ui", "/app/ui/console/services/app_actions/app_manage.py", "/app/ui/console/views/app_overview.py")
			other := upgradeSourceComponent("rbd-api")
			client := newUpgradeSourceClient(t, console, other)
			response := callUpgradeSourceGuard(context.Background(), client, map[string]string{"rbd-api": "example.invalid/api:new", "rbd-app-ui": image})
			if response.Code != http.StatusConflict || client.patches != 0 {
				t.Fatalf("source overrides must block every component before patch: status=%d, patches=%d, body=%s", response.Code, client.patches, response.Body.String())
			}
			var body httputil.ResponseBody
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			for _, fragment := range []string{"RbdComponent", "spec.volumeMounts", "app_manage.py", "app_overview.py", "兼容", "迁移"} {
				if !strings.Contains(body.Msg, fragment) {
					t.Errorf("conflict must explain %q: %s", fragment, body.Msg)
				}
			}
		})
	}
}

func TestUpgradeSourceMountPaths(t *testing.T) {
	for _, tc := range []struct {
		path    string
		blocked bool
	}{
		{path: "/", blocked: true},
		{path: "/app", blocked: true},
		{path: "/app/ui", blocked: true},
		{path: "/app/ui/console", blocked: true},
		{path: "/app/ui/www", blocked: true},
		{path: "/app/ui/goodrain_web", blocked: true},
		{path: "/app/ui/console/views", blocked: true},
		{path: "/app/ui/console/custom_package", blocked: true},
		{path: "/app/ui/console/views/app_overview.py", blocked: true},
		{path: "/app/ui/www/apiclient/regionapi.py", blocked: true},
		{path: "/app/ui/goodrain_web/settings.pyc", blocked: true},
		{path: "/app/ui/console/__pycache__", blocked: true},
		{path: "/app//ui/www/./apiclient/../models.py", blocked: true},
		{path: "/app/ui/www/static/../apiclient/client.pyc", blocked: true},
		{path: "/app/ui/www/static/hook.py", blocked: true},
		{path: "/app/ui/www/templates/hook.pyc", blocked: true},
		{path: "/app/ui/console/config/hook.py", blocked: true},
		{path: "/app/ui/console/migrations/custom.py"},
		{path: "/app/ui/www/migrations/custom.pyc"},
		{path: "/app//ui/console/migrations/./nested/custom.py"},
		{path: "/app/ui/console/migrations/../views/custom.py", blocked: true},
		{path: "/app/ui/www/migrations/../apiclient/custom.pyc", blocked: true},
		{path: "/app/ui/console/migrations-other/custom.py", blocked: true},
		{path: "/app/ui/goodrain_web/migrations/custom.py", blocked: true},
		{path: "/app/data"},
		{path: "/app/region/ssl"},
		{path: "/var/run/secrets/kubernetes.io/serviceaccount"},
		{path: "/app/ui/console/migrations"},
		{path: "/app/ui/www/migrations/"},
		{path: "/app/ui/www/static"},
		{path: "/app/ui/www/static/dists"},
		{path: "/app/ui/www/templates"},
		{path: "/app/ui/goodrain_web/config"},
		{path: "/app/ui/console/config/custom.yaml"},
		{path: "/app/ui/console/custom.json"},
		{path: "/app/ui/console-copy/views/file.py"},
		{path: "/app/uix"},
		{path: "/app/ui/console/config/../../../../data"},
		{path: "/app/ui/%63onsole/file.py"},
		{path: "app/ui/console/file.py"},
		{path: ""},
	} {
		t.Run(tc.path, func(t *testing.T) {
			paths := consoleSourceMountPaths([]corev1.VolumeMount{{MountPath: tc.path}})
			if got := len(paths) > 0; got != tc.blocked {
				t.Fatalf("source override classification = %v, want %v (%v)", got, tc.blocked, paths)
			}
		})
	}
	paths := consoleSourceMountPaths([]corev1.VolumeMount{
		{MountPath: "/app/ui/console/z.py"}, {MountPath: "/app//ui/console/./a.py"},
		{MountPath: "/app/ui/console/a.py"}, {MountPath: "/app/data"},
	})
	if expected := []string{"/app/ui/console/a.py", "/app/ui/console/z.py"}; !reflect.DeepEqual(paths, expected) {
		t.Fatalf("source paths must be normalized, sorted and deduplicated: got %v", paths)
	}
}

func TestUpgradeSourceMountsAllowStandardAndOtherComponents(t *testing.T) {
	for _, tc := range []struct {
		name      string
		paths     []string
		onlyOther bool
	}{
		{name: "fresh installation"},
		{name: "standard mounts", paths: []string{"/app/data", "/app/region/ssl", "/app/ui/console/migrations", "/app/ui/www/migrations"}},
		{name: "persistent migration files", paths: []string{"/app/ui/console/migrations/custom.py", "/app/ui/www/migrations/custom.pyc"}},
		{name: "assets and config", paths: []string{"/app/ui/www/static", "/app/ui/www/templates", "/app/ui/console/config/custom.yaml"}},
		{name: "other request ignores existing Console override", paths: []string{"/app/ui/console/views/app_overview.py"}, onlyOther: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			console := upgradeSourceComponent("rbd-app-ui", tc.paths...)
			other := upgradeSourceComponent("rbd-api", "/app/ui/console/other.py")
			client := newUpgradeSourceClient(t, console, other)
			images := map[string]string{"rbd-api": "example.invalid/api:new"}
			if !tc.onlyOther {
				images["rbd-app-ui"] = "example.invalid/rainbond:new"
			}
			client.beforePatch = func(ctx context.Context, obj k8sclient.Object, patch k8sclient.Patch) {
				if len(client.gets) != len(images) {
					t.Errorf("all requested CRs must be read before any patch: %v", client.gets)
				}
			}
			response := callUpgradeSourceGuard(context.Background(), client, images)
			if response.Code != http.StatusOK || client.patches != len(images) {
				t.Fatalf("compatible mounts should upgrade: status=%d patches=%d body=%s", response.Code, client.patches, response.Body.String())
			}
			for name, image := range images {
				var stored v1alpha1.RbdComponent
				if err := client.Client.Get(context.Background(), types.NamespacedName{Namespace: "rbd-system", Name: name}, &stored); err != nil {
					t.Fatal(err)
				}
				if stored.Spec.Image != image {
					t.Errorf("%s image = %q, want %q", name, stored.Spec.Image, image)
				}
			}
		})
	}
}

func TestUpgradeSourceMountsRequestCancellation(t *testing.T) {
	for _, duringRead := range []bool{false, true} {
		t.Run(map[bool]string{false: "already cancelled", true: "cancelled during preflight"}[duringRead], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := newUpgradeSourceClient(t, upgradeSourceComponent("rbd-app-ui"))
			if duringRead {
				client.afterGet = func(ctx context.Context, obj k8sclient.Object) {
					if ctx.Err() != nil {
						t.Error("request must be live before callback cancels it")
					}
					cancel()
				}
			} else {
				cancel()
			}
			response := callUpgradeSourceGuard(ctx, client, map[string]string{"rbd-app-ui": "example.invalid/rainbond:new"})
			if response.Code != http.StatusRequestTimeout || client.patches != 0 {
				t.Fatalf("cancelled request must not patch: status=%d patches=%d body=%s", response.Code, client.patches, response.Body.String())
			}
		})
	}
}

func TestUpgradeSourceMountsConcurrentChange(t *testing.T) {
	client := newUpgradeSourceClient(t, upgradeSourceComponent("rbd-app-ui"), upgradeSourceComponent("rbd-worker"))
	client.beforePatch = func(ctx context.Context, obj k8sclient.Object, patch k8sclient.Patch) {
		if obj.GetName() != "rbd-app-ui" {
			return
		}
		var current v1alpha1.RbdComponent
		if err := client.Client.Get(ctx, k8sclient.ObjectKeyFromObject(obj), &current); err != nil {
			t.Fatal(err)
		}
		current.Spec.VolumeMounts = []corev1.VolumeMount{{Name: "concurrent", MountPath: "/app/ui/console/views/concurrent.py"}}
		if err := client.Client.Update(ctx, &current); err != nil {
			t.Fatal(err)
		}
	}
	response := callUpgradeSourceGuard(context.Background(), client, map[string]string{"rbd-app-ui": "example.invalid/rainbond:new", "rbd-worker": "example.invalid/worker:new"})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "rbd-app-ui更新异常") {
		t.Fatalf("concurrent change must keep existing per-component error semantics: status=%d body=%s", response.Code, response.Body.String())
	}
	var current v1alpha1.RbdComponent
	if err := client.Client.Get(context.Background(), types.NamespacedName{Namespace: "rbd-system", Name: "rbd-app-ui"}, &current); err != nil {
		t.Fatal(err)
	}
	if current.Spec.Image != "example.invalid/rainbond:old" || len(current.Spec.VolumeMounts) != 1 {
		t.Fatalf("stale preflight must not update image or lose concurrent mounts: %+v", current.Spec)
	}
	if err := client.Client.Get(context.Background(), types.NamespacedName{Namespace: "rbd-system", Name: "rbd-worker"}, &current); err != nil {
		t.Fatal(err)
	}
	if current.Spec.Image != "example.invalid/worker:new" {
		t.Fatal("Console patch conflict must preserve the existing per-component continuation behavior")
	}
}

type upgradeSourceClient struct {
	k8sclient.Client
	gets        []string
	patches     int
	afterGet    func(context.Context, k8sclient.Object)
	beforePatch func(context.Context, k8sclient.Object, k8sclient.Patch)
}

func (c *upgradeSourceClient) Get(ctx context.Context, key k8sclient.ObjectKey, obj k8sclient.Object, opts ...k8sclient.GetOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.gets = append(c.gets, key.Name)
	err := c.Client.Get(ctx, key, obj, opts...)
	if err == nil && c.afterGet != nil {
		c.afterGet(ctx, obj)
	}
	return err
}

func (c *upgradeSourceClient) Patch(ctx context.Context, obj k8sclient.Object, patch k8sclient.Patch, opts ...k8sclient.PatchOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.patches++
	if c.beforePatch != nil {
		c.beforePatch(ctx, obj, patch)
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func newUpgradeSourceClient(t *testing.T, objects ...k8sclient.Object) *upgradeSourceClient {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return &upgradeSourceClient{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()}
}

func upgradeSourceComponent(name string, paths ...string) *v1alpha1.RbdComponent {
	component := &v1alpha1.RbdComponent{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "rbd-system"},
		Spec:       v1alpha1.RbdComponentSpec{Image: "example.invalid/rainbond:old"},
	}
	for _, mountPath := range paths {
		component.Spec.VolumeMounts = append(component.Spec.VolumeMounts, corev1.VolumeMount{Name: "custom", MountPath: mountPath, SubPath: "file"})
	}
	return component
}

func callUpgradeSourceGuard(ctx context.Context, client k8sclient.Client, images map[string]string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v2/cluster/rbd-upgrade", nil).WithContext(ctx)
	upgradeComponentImages(recorder, req, client, "rbd-system", images)
	return recorder
}
