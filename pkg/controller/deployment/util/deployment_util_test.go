/*
Copyright 2015 The Kubernetes Authors.

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

package util

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
)

func addListRSReactor(fakeClient *fake.Clientset, obj runtime.Object) *fake.Clientset {
	fakeClient.AddReactor("list", "replicasets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})
	return fakeClient
}

func addListPodsReactor(fakeClient *fake.Clientset, obj runtime.Object) *fake.Clientset {
	fakeClient.AddReactor("list", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})
	return fakeClient
}

func addGetRSReactor(fakeClient *fake.Clientset, obj runtime.Object) *fake.Clientset {
	rsList, ok := obj.(*extensions.ReplicaSetList)
	fakeClient.AddReactor("get", "replicasets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(testclient.GetAction).GetName()
		if ok {
			for _, rs := range rsList.Items {
				if rs.Name == name {
					return true, &rs, nil
				}
			}
		}
		return false, nil, fmt.Errorf("could not find the requested replica set: %s", name)

	})
	return fakeClient
}

func addUpdateRSReactor(fakeClient *fake.Clientset) *fake.Clientset {
	fakeClient.AddReactor("update", "replicasets", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(testclient.UpdateAction).GetObject().(*extensions.ReplicaSet)
		return true, obj, nil
	})
	return fakeClient
}

func addUpdatePodsReactor(fakeClient *fake.Clientset) *fake.Clientset {
	fakeClient.AddReactor("update", "pods", func(action core.Action) (handled bool, ret runtime.Object, err error) {
		obj := action.(testclient.UpdateAction).GetObject().(*api.Pod)
		return true, obj, nil
	})
	return fakeClient
}

func newPod(now time.Time, ready bool, beforeSec int) api.Pod {
	conditionStatus := api.ConditionFalse
	if ready {
		conditionStatus = api.ConditionTrue
	}
	return api.Pod{
		Status: api.PodStatus{
			Conditions: []api.PodCondition{
				{
					Type:               api.PodReady,
					LastTransitionTime: unversioned.NewTime(now.Add(-1 * time.Duration(beforeSec) * time.Second)),
					Status:             conditionStatus,
				},
			},
		},
	}
}

func TestCountAvailablePods(t *testing.T) {
	now := time.Now()
	tests := []struct {
		pods            []api.Pod
		minReadySeconds int
		expected        int
	}{
		{
			[]api.Pod{
				newPod(now, true, 0),
				newPod(now, true, 2),
				newPod(now, false, 1),
			},
			1,
			1,
		},
		{
			[]api.Pod{
				newPod(now, true, 2),
				newPod(now, true, 11),
				newPod(now, true, 5),
			},
			10,
			1,
		},
	}

	for _, test := range tests {
		if count := countAvailablePods(test.pods, int32(test.minReadySeconds)); int(count) != test.expected {
			t.Errorf("Pods = %#v, minReadySeconds = %d, expected %d, got %d", test.pods, test.minReadySeconds, test.expected, count)
		}
	}
}

// generatePodFromRS creates a pod, with the input ReplicaSet's selector and its template
func generatePodFromRS(rs extensions.ReplicaSet) api.Pod {
	return api.Pod{
		ObjectMeta: api.ObjectMeta{
			Labels: rs.Labels,
		},
		Spec: rs.Spec.Template.Spec,
	}
}

func generatePod(labels map[string]string, image string) api.Pod {
	return api.Pod{
		ObjectMeta: api.ObjectMeta{
			Labels: labels,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:                   image,
					Image:                  image,
					ImagePullPolicy:        api.PullAlways,
					TerminationMessagePath: api.TerminationMessagePathDefault,
				},
			},
		},
	}
}

func generateRSWithLabel(labels map[string]string, image string) extensions.ReplicaSet {
	return extensions.ReplicaSet{
		ObjectMeta: api.ObjectMeta{
			Name:   api.SimpleNameGenerator.GenerateName("replicaset"),
			Labels: labels,
		},
		Spec: extensions.ReplicaSetSpec{
			Replicas: 1,
			Selector: &unversioned.LabelSelector{MatchLabels: labels},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: labels,
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:                   image,
							Image:                  image,
							ImagePullPolicy:        api.PullAlways,
							TerminationMessagePath: api.TerminationMessagePathDefault,
						},
					},
				},
			},
		},
	}
}

// generateRS creates a replica set, with the input deployment's template as its template
func generateRS(deployment extensions.Deployment) extensions.ReplicaSet {
	template := GetNewReplicaSetTemplate(&deployment)
	return extensions.ReplicaSet{
		ObjectMeta: api.ObjectMeta{
			Name:   api.SimpleNameGenerator.GenerateName("replicaset"),
			Labels: template.Labels,
		},
		Spec: extensions.ReplicaSetSpec{
			Template: template,
			Selector: &unversioned.LabelSelector{MatchLabels: template.Labels},
		},
	}
}

// generateDeployment creates a deployment, with the input image as its template
func generateDeployment(image string) extensions.Deployment {
	podLabels := map[string]string{"name": image}
	terminationSec := int64(30)
	return extensions.Deployment{
		ObjectMeta: api.ObjectMeta{
			Name: image,
		},
		Spec: extensions.DeploymentSpec{
			Replicas: 1,
			Selector: &unversioned.LabelSelector{MatchLabels: podLabels},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: podLabels,
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:                   image,
							Image:                  image,
							ImagePullPolicy:        api.PullAlways,
							TerminationMessagePath: api.TerminationMessagePathDefault,
						},
					},
					DNSPolicy:                     api.DNSClusterFirst,
					TerminationGracePeriodSeconds: &terminationSec,
					RestartPolicy:                 api.RestartPolicyAlways,
					SecurityContext:               &api.PodSecurityContext{},
				},
			},
		},
	}
}

func TestGetNewRC(t *testing.T) {
	newDeployment := generateDeployment("nginx")
	newRC := generateRS(newDeployment)

	tests := []struct {
		test     string
		objs     []runtime.Object
		expected *extensions.ReplicaSet
	}{
		{
			"No new ReplicaSet",
			[]runtime.Object{
				&api.PodList{},
				&extensions.ReplicaSetList{
					Items: []extensions.ReplicaSet{
						generateRS(generateDeployment("foo")),
						generateRS(generateDeployment("bar")),
					},
				},
			},
			nil,
		},
		{
			"Has new ReplicaSet",
			[]runtime.Object{
				&api.PodList{},
				&extensions.ReplicaSetList{
					Items: []extensions.ReplicaSet{
						generateRS(generateDeployment("foo")),
						generateRS(generateDeployment("bar")),
						generateRS(generateDeployment("abc")),
						newRC,
						generateRS(generateDeployment("xyz")),
					},
				},
			},
			&newRC,
		},
	}

	for _, test := range tests {
		fakeClient := &fake.Clientset{}
		fakeClient = addListPodsReactor(fakeClient, test.objs[0])
		fakeClient = addListRSReactor(fakeClient, test.objs[1])
		fakeClient = addUpdatePodsReactor(fakeClient)
		fakeClient = addUpdateRSReactor(fakeClient)
		rs, err := GetNewReplicaSet(&newDeployment, fakeClient)
		if err != nil {
			t.Errorf("In test case %s, got unexpected error %v", test.test, err)
		}
		if !api.Semantic.DeepEqual(rs, test.expected) {
			t.Errorf("In test case %s, expected %+v, got %+v", test.test, test.expected, rs)
		}
	}
}

func TestGetOldRCs(t *testing.T) {
	newDeployment := generateDeployment("nginx")
	newRS := generateRS(newDeployment)
	newRS.Status.FullyLabeledReplicas = newRS.Spec.Replicas
	newPod := generatePodFromRS(newRS)

	// create 2 old deployments and related replica sets/pods, with the same labels but different template
	oldDeployment := generateDeployment("nginx")
	oldDeployment.Spec.Template.Spec.Containers[0].Name = "nginx-old-1"
	oldRS := generateRS(oldDeployment)
	oldRS.Status.FullyLabeledReplicas = oldRS.Spec.Replicas
	oldPod := generatePodFromRS(oldRS)
	oldDeployment2 := generateDeployment("nginx")
	oldDeployment2.Spec.Template.Spec.Containers[0].Name = "nginx-old-2"
	oldRS2 := generateRS(oldDeployment2)
	oldRS2.Status.FullyLabeledReplicas = oldRS2.Spec.Replicas
	oldPod2 := generatePodFromRS(oldRS2)

	// create 1 ReplicaSet that existed before the deployment, with the same labels as the deployment
	existedPod := generatePod(newDeployment.Spec.Template.Labels, "foo")
	existedRS := generateRSWithLabel(newDeployment.Spec.Template.Labels, "foo")
	existedRS.Status.FullyLabeledReplicas = existedRS.Spec.Replicas

	tests := []struct {
		test     string
		objs     []runtime.Object
		expected []*extensions.ReplicaSet
	}{
		{
			"No old ReplicaSets",
			[]runtime.Object{
				&api.PodList{
					Items: []api.Pod{
						generatePod(newDeployment.Spec.Template.Labels, "foo"),
						generatePod(newDeployment.Spec.Template.Labels, "bar"),
						newPod,
					},
				},
				&extensions.ReplicaSetList{
					Items: []extensions.ReplicaSet{
						generateRS(generateDeployment("foo")),
						newRS,
						generateRS(generateDeployment("bar")),
					},
				},
			},
			[]*extensions.ReplicaSet{},
		},
		{
			"Has old ReplicaSet",
			[]runtime.Object{
				&api.PodList{
					Items: []api.Pod{
						oldPod,
						oldPod2,
						generatePod(map[string]string{"name": "bar"}, "bar"),
						generatePod(map[string]string{"name": "xyz"}, "xyz"),
						existedPod,
						generatePod(newDeployment.Spec.Template.Labels, "abc"),
					},
				},
				&extensions.ReplicaSetList{
					Items: []extensions.ReplicaSet{
						oldRS2,
						oldRS,
						existedRS,
						newRS,
						generateRSWithLabel(map[string]string{"name": "xyz"}, "xyz"),
						generateRSWithLabel(map[string]string{"name": "bar"}, "bar"),
					},
				},
			},
			[]*extensions.ReplicaSet{&oldRS, &oldRS2, &existedRS},
		},
	}

	for _, test := range tests {
		fakeClient := &fake.Clientset{}
		fakeClient = addListPodsReactor(fakeClient, test.objs[0])
		fakeClient = addListRSReactor(fakeClient, test.objs[1])
		fakeClient = addGetRSReactor(fakeClient, test.objs[1])
		fakeClient = addUpdatePodsReactor(fakeClient)
		fakeClient = addUpdateRSReactor(fakeClient)
		rss, _, err := GetOldReplicaSets(&newDeployment, fakeClient)
		if err != nil {
			t.Errorf("In test case %s, got unexpected error %v", test.test, err)
		}
		if !equal(rss, test.expected) {
			t.Errorf("In test case %q, expected:", test.test)
			for _, rs := range test.expected {
				t.Errorf("rs = %+v", rs)
			}
			t.Errorf("In test case %q, got:", test.test)
			for _, rs := range rss {
				t.Errorf("rs = %+v", rs)
			}
		}
	}
}

func generatePodTemplateSpec(name, nodeName string, annotations, labels map[string]string) api.PodTemplateSpec {
	return api.PodTemplateSpec{
		ObjectMeta: api.ObjectMeta{
			Name:        name,
			Annotations: annotations,
			Labels:      labels,
		},
		Spec: api.PodSpec{
			NodeName: nodeName,
		},
	}
}

func TestEqualIgnoreHash(t *testing.T) {
	tests := []struct {
		test           string
		former, latter api.PodTemplateSpec
		expected       bool
	}{
		{
			"Same spec, same labels",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			true,
		},
		{
			"Same spec, only pod-template-hash label value is different",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			true,
		},
		{
			"Same spec, the former doesn't have pod-template-hash label",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			true,
		},
		{
			"Same spec, the label is different, and the pod-template-hash label value is the same",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			false,
		},
		{
			"Different spec, same labels",
			generatePodTemplateSpec("foo", "foo-node", map[string]string{"former": "value"}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo", "foo-node", map[string]string{"latter": "value"}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			false,
		},
		{
			"Different spec, different pod-template-hash label value",
			generatePodTemplateSpec("foo-1", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-1", "something": "else"}),
			generatePodTemplateSpec("foo-2", "foo-node", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			false,
		},
		{
			"Different spec, the former doesn't have pod-template-hash label",
			generatePodTemplateSpec("foo-1", "foo-node-1", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo-2", "foo-node-2", map[string]string{}, map[string]string{extensions.DefaultDeploymentUniqueLabelKey: "value-2", "something": "else"}),
			false,
		},
		{
			"Different spec, different labels",
			generatePodTemplateSpec("foo", "foo-node-1", map[string]string{}, map[string]string{"something": "else"}),
			generatePodTemplateSpec("foo", "foo-node-2", map[string]string{}, map[string]string{"nothing": "else"}),
			false,
		},
	}

	for _, test := range tests {
		equal, err := equalIgnoreHash(test.former, test.latter)
		if err != nil {
			t.Errorf("In test case %q, returned unexpected error %v", test.test, err)
		}
		if equal != test.expected {
			t.Errorf("In test case %q, expected %v", test.test, test.expected)
		}
		equal, err = equalIgnoreHash(test.latter, test.former)
		if err != nil {
			t.Errorf("In test case %q (reverse order), returned unexpected error %v", test.test, err)
		}
		if equal != test.expected {
			t.Errorf("In test case %q (reverse order), expected %v", test.test, test.expected)
		}
	}
}

func TestFindNewReplicaSet(t *testing.T) {
	deployment := generateDeployment("nginx")
	newRS := generateRS(deployment)
	newRS.Labels[extensions.DefaultDeploymentUniqueLabelKey] = "different-hash"
	oldDeployment := generateDeployment("nginx")
	oldDeployment.Spec.Template.Spec.Containers[0].Name = "nginx-old-1"
	oldRS := generateRS(oldDeployment)
	oldRS.Status.FullyLabeledReplicas = oldRS.Spec.Replicas

	tests := []struct {
		test       string
		deployment extensions.Deployment
		rsList     []extensions.ReplicaSet
		expected   *extensions.ReplicaSet
	}{
		{
			test:       "Get new ReplicaSet with the same spec but different pod-template-hash value",
			deployment: deployment,
			rsList:     []extensions.ReplicaSet{newRS, oldRS},
			expected:   &newRS,
		},
		{
			test:       "Get nil new ReplicaSet",
			deployment: deployment,
			rsList:     []extensions.ReplicaSet{oldRS},
			expected:   nil,
		},
	}

	for _, test := range tests {
		if rs, err := FindNewReplicaSet(&test.deployment, test.rsList); !reflect.DeepEqual(rs, test.expected) || err != nil {
			t.Errorf("In test case %q, expected %#v, got %#v: %v", test.test, test.expected, rs, err)
		}
	}
}

func TestFindOldReplicaSets(t *testing.T) {
	deployment := generateDeployment("nginx")
	newRS := generateRS(deployment)
	newRS.Labels[extensions.DefaultDeploymentUniqueLabelKey] = "different-hash"
	oldDeployment := generateDeployment("nginx")
	oldDeployment.Spec.Template.Spec.Containers[0].Name = "nginx-old-1"
	oldRS := generateRS(oldDeployment)
	oldRS.Status.FullyLabeledReplicas = oldRS.Spec.Replicas
	newPod := generatePodFromRS(newRS)
	oldPod := generatePodFromRS(oldRS)

	tests := []struct {
		test       string
		deployment extensions.Deployment
		rsList     []extensions.ReplicaSet
		podList    *api.PodList
		expected   []*extensions.ReplicaSet
	}{
		{
			test:       "Get old ReplicaSets",
			deployment: deployment,
			rsList:     []extensions.ReplicaSet{newRS, oldRS},
			podList: &api.PodList{
				Items: []api.Pod{
					newPod,
					oldPod,
				},
			},
			expected: []*extensions.ReplicaSet{&oldRS},
		},
		{
			test:       "Get old ReplicaSets with no new ReplicaSet",
			deployment: deployment,
			rsList:     []extensions.ReplicaSet{oldRS},
			podList: &api.PodList{
				Items: []api.Pod{
					oldPod,
				},
			},
			expected: []*extensions.ReplicaSet{&oldRS},
		},
		{
			test:       "Get empty old ReplicaSets",
			deployment: deployment,
			rsList:     []extensions.ReplicaSet{newRS},
			podList: &api.PodList{
				Items: []api.Pod{
					newPod,
				},
			},
			expected: []*extensions.ReplicaSet{},
		},
	}

	for _, test := range tests {
		if old, _, err := FindOldReplicaSets(&test.deployment, test.rsList, test.podList); !reflect.DeepEqual(old, test.expected) || err != nil {
			t.Errorf("In test case %q, expected %#v, got %#v: %v", test.test, test.expected, old, err)
		}
	}
}

// equal compares the equality of two ReplicaSet slices regardless of their ordering
func equal(rss1, rss2 []*extensions.ReplicaSet) bool {
	if reflect.DeepEqual(rss1, rss2) {
		return true
	}
	if rss1 == nil || rss2 == nil || len(rss1) != len(rss2) {
		return false
	}
	count := 0
	for _, rs1 := range rss1 {
		for _, rs2 := range rss2 {
			if reflect.DeepEqual(rs1, rs2) {
				count++
				break
			}
		}
	}
	return count == len(rss1)
}
