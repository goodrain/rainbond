package clean

import (
	"fmt"
	"github.com/goodrain/rainbond/builder/sources/registry"
	"sort"
	"testing"
)

// TestAutoClean 执行此方法你应该通过第三方组件将rbd-hub暴露出来，并且通过kubectl命令查找仓库账号密码
func TestAutoClean(t *testing.T) {
	reg, _ := registry.New("", "", "")
	rep := ""
	tags, err := reg.Tags(rep)
	if err != nil {
		return
	}

	sort.Strings(tags)
	fmt.Println(tags)
	for _, v := range tags {
		v2, err := reg.ManifestDigestV2(rep, v)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(v2)
	}
}
