package controller

import (
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"

	"github.com/goodrain/rainbond-operator/api/v1alpha1"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func upgradeComponentImages(w http.ResponseWriter, r *http.Request, kubeClient k8sclient.Client, namespace string, images map[string]string) {
	ctx := r.Context()
	failures := make([]string, 0)
	names := make([]string, 0, len(images))
	for name := range images {
		names = append(names, name)
	}
	sort.Strings(names)
	components := make([]*v1alpha1.RbdComponent, 0, len(names))
	// Finish preflight before changing any image in this request. A source mount
	// keeps hiding the new image's files, including when a mutable tag is reused.
	for _, name := range names {
		if ctx.Err() != nil {
			returnUpgradeRequestCancelled(w, r)
			return
		}
		var component v1alpha1.RbdComponent
		if err := kubeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &component); err != nil {
			failures = append(failures, fmt.Sprintf("%s获取异常%s", name, err.Error()))
			continue
		}
		if name == "rbd-app-ui" {
			if paths := consoleSourceMountPaths(component.Spec.VolumeMounts); len(paths) > 0 {
				httputil.ReturnError(r, w, http.StatusConflict, fmt.Sprintf(
					"升级前检查发现 RbdComponent %s/%s 的 spec.volumeMounts 覆盖 Console 核心源码：%s。"+
						"本次请求未更新任何组件。请先复核自定义源码与目标镜像的兼容性，并在权威 RbdComponent 或 Helm/GitOps 来源中迁移对应挂载；"+
						"仅修改 Deployment 无法保证升级后生效，请勿自动移除合法定制。", namespace, name, strings.Join(paths, ", ")))
				return
			}
		}
		components = append(components, &component)
	}
	for _, component := range components {
		if ctx.Err() != nil {
			returnUpgradeRequestCancelled(w, r)
			return
		}
		original := component.DeepCopy()
		component.Spec.Image = images[component.Name]
		patch := k8sclient.MergeFrom(original)
		if original.ResourceVersion != "" {
			// Do not retry a conflict: a concurrent mount change invalidates the
			// preflight. Merge patch also preserves fields absent from our CR type.
			patch = k8sclient.MergeFromWithOptions(original, k8sclient.MergeFromWithOptimisticLock{})
		}
		logrus.Infof("upgrade [%s] image to [%s]", component.Name, component.Spec.Image)
		if err := kubeClient.Patch(ctx, component, patch); err != nil {
			failures = append(failures, fmt.Sprintf("%s更新异常%s", component.Name, err.Error()))
		}
	}
	if ctx.Err() != nil {
		returnUpgradeRequestCancelled(w, r)
		return
	}
	httputil.ReturnSuccess(r, w, failures)
}

func returnUpgradeRequestCancelled(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnError(r, w, http.StatusRequestTimeout, "升级请求已取消或超时，未继续更新组件，请重新检查后重试")
}

func consoleSourceMountPaths(mounts []corev1.VolumeMount) []string {
	paths := make(map[string]struct{})
	for _, mount := range mounts {
		mountPath := path.Clean(mount.MountPath)
		if path.IsAbs(mountPath) && coversConsoleSource(mountPath) {
			paths[mountPath] = struct{}{}
		}
	}
	result := make([]string, 0, len(paths))
	for mountPath := range paths {
		result = append(result, mountPath)
	}
	sort.Strings(result)
	return result
}

func coversConsoleSource(mountPath string) bool {
	for _, root := range []string{"/app/ui/console", "/app/ui/www", "/app/ui/goodrain_web"} {
		if pathContains(mountPath, root) {
			return true
		}
		if !pathContains(root, mountPath) {
			continue
		}
		// Console and www migrations are persistent upgrade assets. Exclude
		// their complete subtrees, including individual Python migration files.
		if (root == "/app/ui/console" || root == "/app/ui/www") && pathContains(root+"/migrations", mountPath) {
			return false
		}
		extension := path.Ext(mountPath)
		if extension == ".py" || extension == ".pyc" {
			return true
		}
		// Files with other extensions do not replace Python modules. For
		// directory mounts, allow the asset/configuration trees.
		if extension != "" {
			return false
		}
		relative := strings.TrimPrefix(mountPath, root+"/")
		first := strings.SplitN(relative, "/", 2)[0]
		if first == "static" || first == "templates" || first == "config" {
			return false
		}
		// Unknown directories may hide Python packages. Require compatibility
		// review instead of reading volume data or deleting custom mounts.
		return true
	}
	return false
}

func pathContains(directory, target string) bool {
	return target == directory || strings.HasPrefix(target, strings.TrimSuffix(directory, "/")+"/")
}
