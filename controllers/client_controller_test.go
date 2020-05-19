/*
Copyright 2020 Betsson Group.

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
package controllers

import (
	"context"

	oauth2v1 "github.com/BetssonGroup/dex-operator/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Context("Inside of a new namespace", func() {
	ctx := context.TODO()
	ns := SetupTest(ctx)
	redirectURLs := make([]string, 1)
	redirectURLs[0] = "https://www.betssongroup.com"

	Describe("when no existing resources exist", func() {
		It("should create a new Client resource with the specified name", func() {
			client := &oauth2v1.Client{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testing-client",
					Namespace: ns.Name,
				},
				Spec: oauth2v1.ClientSpec{
					Secret:       "xxx-xxx-xxx-xxx",
					Name:         "Testing Client",
					RedirectURIs: redirectURLs,
				},
			}
			err := k8sClient.Create(ctx, client)
			Expect(err).NotTo(HaveOccurred(), "failed to create test Client resource")
		})
	})
})

func getResourceFunc(ctx context.Context, key client.ObjectKey, obj runtime.Object) func() error {
	return func() error {
		return k8sClient.Get(ctx, key, obj)
	}
}
