/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package action

import (
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"helm.sh/helm/v4/pkg/action"
	helmaction "helm.sh/helm/v4/pkg/action"
	helmchart "helm.sh/helm/v4/pkg/chart/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/helm-controller/internal/testutil"
)

func Test_newUpgrade(t *testing.T) {
	t.Run("new upgrade", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				Timeout: &metav1.Duration{Duration: time.Minute},
				Upgrade: &v2.Upgrade{
					Timeout: &metav1.Duration{Duration: 10 * time.Second},
					Force:   true,
				},
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.Namespace).To(Equal(obj.Namespace))
		g.Expect(got.Timeout).To(Equal(obj.Spec.Upgrade.Timeout.Duration))
		// ForceReplace is not set in the constructor; it is set after SSA resolution
		// in Upgrade() to avoid the Helm SDK mutual exclusivity error.
		g.Expect(got.ForceReplace).To(BeFalse())
	})

	t.Run("timeout fallback", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				Timeout: &metav1.Duration{Duration: time.Minute},
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.Namespace).To(Equal(obj.Namespace))
		g.Expect(got.Timeout).To(Equal(obj.Spec.Timeout.Duration))
	})

	t.Run("applies options", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, []UpgradeOption{
			func(upgrade *helmaction.Upgrade) {
				upgrade.Install = true
			},
			func(upgrade *helmaction.Upgrade) {
				upgrade.DryRunStrategy = helmaction.DryRunClient
			},
		})
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.Install).To(BeTrue())
		g.Expect(got.DryRunStrategy).To(Equal(helmaction.DryRunClient))
	})

	t.Run("disable take ownership", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				Upgrade: &v2.Upgrade{
					DisableTakeOwnership: true,
				},
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.TakeOwnership).To(BeFalse())
	})

	t.Run("server side apply is auto regardless of UseHelm3Defaults", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{},
		}

		// Save and restore UseHelm3Defaults
		oldUseHelm3Defaults := UseHelm3Defaults
		t.Cleanup(func() { UseHelm3Defaults = oldUseHelm3Defaults })

		// Test with UseHelm3Defaults = false
		UseHelm3Defaults = false
		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.ServerSideApply).To(Equal("auto"))

		// Test with UseHelm3Defaults = true
		UseHelm3Defaults = true
		got = newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.ServerSideApply).To(Equal("auto"))
	})

	t.Run("server side apply user specified", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				Upgrade: &v2.Upgrade{
					ServerSideApply: v2.ServerSideApplyEnabled,
				},
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.ServerSideApply).To(Equal("true"))

		obj.Spec.Upgrade.ServerSideApply = v2.ServerSideApplyDisabled
		got = newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.ServerSideApply).To(Equal("false"))
	})

	t.Run("post render strategy defaults to combined with Helm4 defaults", func(t *testing.T) {
		g := NewWithT(t)

		// Save and restore UseHelm3Defaults
		oldUseHelm3Defaults := UseHelm3Defaults
		t.Cleanup(func() { UseHelm3Defaults = oldUseHelm3Defaults })
		UseHelm3Defaults = false

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.PostRenderStrategy).To(Equal(action.PostRenderStrategyCombined))
	})

	t.Run("post render strategy defaults to nohooks with UseHelm3Defaults", func(t *testing.T) {
		g := NewWithT(t)

		// Save and restore UseHelm3Defaults
		oldUseHelm3Defaults := UseHelm3Defaults
		t.Cleanup(func() { UseHelm3Defaults = oldUseHelm3Defaults })
		UseHelm3Defaults = true

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.PostRenderStrategy).To(Equal(action.PostRenderStrategyNoHooks))
	})

	t.Run("post render strategy combined", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				PostRenderStrategy: v2.PostRenderStrategyCombined,
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.PostRenderStrategy).To(Equal(action.PostRenderStrategyCombined))
	})

	t.Run("post render strategy separate", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				PostRenderStrategy: v2.PostRenderStrategySeparate,
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.PostRenderStrategy).To(Equal(action.PostRenderStrategySeparate))
	})

	t.Run("post render strategy nohooks", func(t *testing.T) {
		g := NewWithT(t)

		obj := &v2.HelmRelease{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "upgrade",
				Namespace: "upgrade-ns",
			},
			Spec: v2.HelmReleaseSpec{
				PostRenderStrategy: v2.PostRenderStrategyNoHooks,
			},
		}

		got := newUpgrade(&helmaction.Configuration{}, obj, nil)
		g.Expect(got).ToNot(BeNil())
		g.Expect(got.PostRenderStrategy).To(Equal(action.PostRenderStrategyNoHooks))
	})
}

func Test_copyChartForRender(t *testing.T) {
	g := NewWithT(t)
	child := testutil.BuildChart(testutil.ChartWithName("child"), testutil.ChartWithValues(map[string]any{"key": "original"}))
	grandchild := testutil.BuildChart(testutil.ChartWithName("grandchild"))
	child.AddDependency(grandchild)
	chart := testutil.BuildChart(testutil.ChartWithDependency(&helmchart.Dependency{Name: "child", Version: "0.1.0"}, child))
	copied, err := copyChartForRender(chart)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(copied.Dependencies()).To(HaveLen(1))
	g.Expect(copied.Dependencies()[0].Parent()).To(BeIdenticalTo(copied))
	g.Expect(copied.Dependencies()[0].Dependencies()).To(HaveLen(1))
	copied.Dependencies()[0].Values["key"] = "changed"
	copied.Dependencies()[0].Dependencies()[0].Metadata.Name = "changed"
	g.Expect(child.Values["key"]).To(Equal("original"))
	g.Expect(grandchild.Name()).To(Equal("grandchild"))
}

func Test_renderUpgradeError(t *testing.T) {
	g := NewWithT(t)
	cause := errors.New("secret-value")
	err := &renderUpgradeError{cause: cause}
	g.Expect(err.Error()).NotTo(ContainSubstring("secret-value"))
	g.Expect(errors.Is(err, cause)).To(BeTrue())
}
