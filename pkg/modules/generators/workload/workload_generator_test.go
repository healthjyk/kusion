package workload

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"

	"kusionstack.io/kusion/pkg/apis/core/v1/workload/container"
	"kusionstack.io/kusion/pkg/apis/core/v1/workload/network"

	apiv1 "kusionstack.io/kusion/pkg/apis/core/v1"
	"kusionstack.io/kusion/pkg/apis/core/v1/workload"
	"kusionstack.io/kusion/pkg/modules"
)

func TestNewWorkloadGeneratorFunc(t *testing.T) {
	t.Run("NewWorkloadGeneratorFunc should return a valid generator function", func(t *testing.T) {
		expectedWorkload := &workload.Workload{}
		expectedAppName := "test"
		expectedProject := "test"
		expectedStack := "test"
		expectedModuleConfigs := map[string]apiv1.GenericConfig{
			"service": {
				"type": "Deployment",
			},
			"job": {
				"replicas": 2,
			},
		}

		generatorFunc := NewWorkloadGeneratorFunc(&Generator{
			Project:         expectedProject,
			Stack:           expectedStack,
			App:             expectedAppName,
			Namespace:       expectedAppName,
			Workload:        expectedWorkload,
			PlatformConfigs: expectedModuleConfigs,
		})
		actualGenerator, err := generatorFunc()

		assert.NoError(t, err, "Error should be nil")
		assert.NotNil(t, actualGenerator, "Generator should not be nil")
		assert.Equal(t, expectedProject, actualGenerator.(*Generator).Project, "Project mismatch")
		assert.Equal(t, expectedStack, actualGenerator.(*Generator).Stack, "Stack mismatch")
		assert.Equal(t, expectedAppName, actualGenerator.(*Generator).App, "AppName mismatch")
		assert.Equal(t, expectedWorkload, actualGenerator.(*Generator).Workload, "Workload mismatch")
		assert.Equal(t, expectedModuleConfigs, actualGenerator.(*Generator).PlatformConfigs, "ModuleConfigs mismatch")
	})
}

func TestWorkloadGenerator_Generate(t *testing.T) {
	testCases := []struct {
		name             string
		expectedWorkload *workload.Workload
	}{
		{
			name: "Generate should generate the expected service",
			expectedWorkload: &workload.Workload{
				Header: workload.Header{
					Type: "Service",
				},
				Service: &workload.Service{
					Base: workload.Base{},
					Ports: []network.Port{
						{
							Port:     80,
							Protocol: "TCP",
						},
					},
				},
			},
		},
		{
			name: "Generate should generate the expected job",
			expectedWorkload: &workload.Workload{
				Header: workload.Header{
					Type: "Job",
				},
				Job: &workload.Job{
					Base:     workload.Base{},
					Schedule: "* * * * *",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expectedProject := "test"
			expectedStack := "test"
			expectedAppName := "test"
			expectedModuleConfigs := map[string]apiv1.GenericConfig{
				"service": {
					"type": "Deployment",
				},
				"job": {
					"replicas": 2,
				},
			}

			generatorFunc := NewWorkloadGeneratorFunc(&Generator{
				Project:         expectedProject,
				Stack:           expectedStack,
				App:             expectedAppName,
				Namespace:       expectedAppName,
				PlatformConfigs: expectedModuleConfigs,
				Workload:        tc.expectedWorkload,
			})
			actualGenerator, err := generatorFunc()
			assert.NoError(t, err, "generator func Error should be nil")
			spec := &apiv1.Intent{}
			err = actualGenerator.Generate(spec)
			assert.NoError(t, err, "Error should be nil")
			assert.NotNil(t, spec.Resources, "Resources should not be nil")

			// Check the generated resource
			resource := spec.Resources[0]
			actual := mapToUnstructured(resource.Attributes)

			assert.Equal(t, expectedProject, actual.GetNamespace(), "Namespace mismatch")
			assert.Equal(t, modules.UniqueAppName(expectedProject, expectedStack, expectedAppName), actual.GetName(), "Name mismatch")

			if tc.expectedWorkload.Header.Type == "Service" {
				assert.Equal(t, "Deployment", actual.GetKind(), "Resource kind mismatch")
				assert.Equal(t, modules.MergeMaps(modules.UniqueAppLabels(expectedProject, expectedAppName), tc.expectedWorkload.Service.Labels), actual.GetLabels(), "Labels mismatch")
			} else if tc.expectedWorkload.Header.Type == "Job" {
				assert.Equal(t, "CronJob", actual.GetKind(), "Resource kind mismatch")
				assert.Equal(t, modules.MergeMaps(modules.UniqueAppLabels(expectedProject, expectedAppName), tc.expectedWorkload.Job.Labels), actual.GetLabels(), "Labels mismatch")
				assert.Equal(t, modules.MergeMaps(tc.expectedWorkload.Job.Annotations), actual.GetAnnotations(), "Annotations mismatch")
			}
		})
	}
}

func TestToOrderedContainers(t *testing.T) {
	t.Run("toOrderedContainers should convert app containers to ordered containers", func(t *testing.T) {
		appContainers := make(map[string]container.Container)
		appContainers["container1"] = container.Container{
			Image: "image1",
			Env:   make(yaml.MapSlice, 0),
		}
		appContainers["container2"] = container.Container{
			Image: "image2",
			Env: yaml.MapSlice{
				{
					Key:   "key",
					Value: "value",
				},
			},
		}
		appContainers["container3"] = container.Container{
			Image: "image3",
			Files: map[string]container.FileSpec{
				"/tmp/example1/file.txt": {
					Content: "some file contents",
					Mode:    "0777",
				},
				"/tmp/example2/file.txt": {
					Content: "some file contents",
					Mode:    "0644",
				},
			},
		}

		actualContainers, actualVolumes, actualConfigMaps, err := toOrderedContainers(appContainers, "mock-app-name")
		wantedConfigMapData := map[string]string{"file.txt": "some file contents"}

		assert.NoError(t, err, "Error should be nil")
		assert.Len(t, actualContainers, 3, "Number of containers mismatch")
		assert.Equal(t, "container1", actualContainers[0].Name, "Container name mismatch")
		assert.Equal(t, "image1", actualContainers[0].Image, "Container image mismatch")
		assert.Equal(t, "container2", actualContainers[1].Name, "Container name mismatch")
		assert.Equal(t, "image2", actualContainers[1].Image, "Container image mismatch")
		assert.Len(t, actualContainers[1].Env, 1, "Number of env vars mismatch")
		assert.Equal(t, "key", actualContainers[1].Env[0].Name, "Env var name mismatch")
		assert.Equal(t, "value", actualContainers[1].Env[0].Value, "Env var value mismatch")
		assert.Equal(t, "container3", actualContainers[2].Name, "Container name mismatch")
		assert.Equal(t, "image3", actualContainers[2].Image, "Container image mismatch")
		assert.Equal(t, "mock-app-name-container3-0", actualContainers[2].VolumeMounts[0].Name, "Container volumeMount name mismatch")
		assert.Equal(t, "/tmp/example1", actualContainers[2].VolumeMounts[0].MountPath, "Container volumeMount path mismatch")
		assert.Equal(t, "/tmp/example2", actualContainers[2].VolumeMounts[1].MountPath, "Container volumeMount path mismatch")
		assert.Equal(t, int32(511), *actualVolumes[0].ConfigMap.DefaultMode, "Volume mode mismatch")
		assert.Equal(t, int32(420), *actualVolumes[1].ConfigMap.DefaultMode, "Volume mode mismatch")
		assert.Equal(t, wantedConfigMapData, actualConfigMaps[0].Data, "ConfigMap data mismatch")
		assert.Equal(t, wantedConfigMapData, actualConfigMaps[1].Data, "ConfigMap data mismatch")
	})
	t.Run("toOrderedContainers should convert app containers with probe to ordered containers", func(t *testing.T) {
		appContainers := map[string]container.Container{
			"nginx": {
				Image: "nginx:v1",
				Resources: map[string]string{
					"cpu":    "2-4",
					"memory": "4Gi-8Gi",
				},
				LivenessProbe: &container.Probe{
					ProbeHandler: &container.ProbeHandler{
						TypeWrapper: container.TypeWrapper{
							Type: "Exec",
						},
						ExecAction: &container.ExecAction{
							Command: []string{"/bin/sh", "-c", "echo live"},
						},
					},
				},
				ReadinessProbe: &container.Probe{
					ProbeHandler: &container.ProbeHandler{
						TypeWrapper: container.TypeWrapper{
							Type: "Http",
						},
						HTTPGetAction: &container.HTTPGetAction{
							URL: "http://localhost:8080/readiness",
							Headers: map[string]string{
								"header": "value",
							},
						},
					},
					InitialDelaySeconds: 10,
				},
				StartupProbe: &container.Probe{
					ProbeHandler: &container.ProbeHandler{
						TypeWrapper: container.TypeWrapper{
							Type: "Tcp",
						},
						TCPSocketAction: &container.TCPSocketAction{
							URL: "10.0.0.1:8888",
						},
					},
				},
			},
		}

		actualContainers, _, _, err := toOrderedContainers(appContainers, "mock-app-name")

		assert.NoError(t, err, "Error should be nil")
		assert.Len(t, actualContainers, 1, "Number of containers mismatch")
		assert.Equal(t, "nginx", actualContainers[0].Name, "Container name mismatch")
		assert.Equal(t, "nginx:v1", actualContainers[0].Image, "Container image mismatch")
		assert.Len(t, actualContainers[0].Resources.Requests, 2, "Number of resource requests mismatch")

		// Assert request resources
		rQuantity := actualContainers[0].Resources.Requests["cpu"]
		assert.Equal(t, "2", (&rQuantity).String(), "CPU request mismatch")
		rQuantity = actualContainers[0].Resources.Requests["memory"]
		assert.Equal(t, "4Gi", (&rQuantity).String(), "CPU request mismatch")

		// Assert limit resources
		rQuantity = actualContainers[0].Resources.Limits["cpu"]
		assert.Equal(t, "4", (&rQuantity).String(), "CPU request mismatch")
		rQuantity = actualContainers[0].Resources.Limits["memory"]
		assert.Equal(t, "8Gi", (&rQuantity).String(), "CPU request mismatch")

		assert.NotNil(t, actualContainers[0].ReadinessProbe, "ReadinessProbe should not be nil")
		assert.NotNil(t, actualContainers[0].ReadinessProbe.HTTPGet, "ReadinessProbe.HTTPGet action should not be nil")
		assert.Equal(t, "HTTP", string(actualContainers[0].ReadinessProbe.HTTPGet.Scheme), "HTTPGet.Scheme mismatch")
		assert.Equal(t, "/readiness", actualContainers[0].ReadinessProbe.HTTPGet.Path, "HTTPGet.Path mismatch")
		assert.Equal(t, "8080", actualContainers[0].ReadinessProbe.HTTPGet.Port.String(), "HTTPGet.Port mismatch")
		assert.Equal(t, "", actualContainers[0].ReadinessProbe.HTTPGet.Host, "HTTPGet.Host mismatch")
		assert.Equal(t, 1, len(actualContainers[0].ReadinessProbe.HTTPGet.HTTPHeaders), "HTTPGet.HTTPHeaders length mismatch")

		assert.NotNil(t, actualContainers[0].LivenessProbe, "LivenessProbe should not be nil")
		assert.NotNil(t, actualContainers[0].LivenessProbe.Exec, "LivenessProbe.Exec action should not be nil")
		assert.Len(t, actualContainers[0].LivenessProbe.Exec.Command, 3, "LivenessProbe.Exec commands length mismatch")
		assert.Equal(t, []string{"/bin/sh", "-c", "echo live"}, actualContainers[0].LivenessProbe.Exec.Command, "LivenessProbe.Exec commands mismatch")

		assert.NotNil(t, actualContainers[0].StartupProbe, "StartupProbe should not be nil")
		assert.NotNil(t, actualContainers[0].StartupProbe.TCPSocket, "StartupProbe.TCPSocket action should not be nil")
		assert.Equal(t, "10.0.0.1", actualContainers[0].StartupProbe.TCPSocket.Host, "TCPSocket.Host mismatch")
		assert.Equal(t, "8888", actualContainers[0].StartupProbe.TCPSocket.Port.String(), "TCPSocket.Port mismatch")
	})
	t.Run("toOrderedContainers should convert app containers with lifecycle to ordered containers", func(t *testing.T) {
		appContainers := map[string]container.Container{
			"nginx": {
				Image: "nginx:v1",
				Lifecycle: &container.Lifecycle{
					PreStop: &container.LifecycleHandler{
						TypeWrapper: container.TypeWrapper{
							Type: "Exec",
						},
						ExecAction: &container.ExecAction{
							Command: []string{"/bin/sh", "-c", "echo live"},
						},
					},
					PostStart: &container.LifecycleHandler{
						TypeWrapper: container.TypeWrapper{
							Type: "Http",
						},
						HTTPGetAction: &container.HTTPGetAction{
							URL: "http://localhost:8080/readiness",
							Headers: map[string]string{
								"header": "value",
							},
						},
					},
				},
			},
		}

		actualContainers, _, _, err := toOrderedContainers(appContainers, "mock-app-name")

		assert.NoError(t, err, "Error should be nil")
		assert.Len(t, actualContainers, 1, "Number of containers mismatch")
		assert.Equal(t, "nginx", actualContainers[0].Name, "Container name mismatch")
		assert.Equal(t, "nginx:v1", actualContainers[0].Image, "Container image mismatch")

		assert.NotNil(t, actualContainers[0].Lifecycle, "Lifecycle should not be nil")
		assert.NotNil(t, actualContainers[0].Lifecycle.PreStop, "Lifecycle.PreStop should not be nil")
		assert.NotNil(t, actualContainers[0].Lifecycle.PreStop.Exec, "PreStop.Exec action should not be nil")
		assert.Len(t, actualContainers[0].Lifecycle.PreStop.Exec.Command, 3, "PreStop.Exec commands length mismatch")
		assert.Equal(t, []string{"/bin/sh", "-c", "echo live"}, actualContainers[0].Lifecycle.PreStop.Exec.Command, "PreStop.Exec commands mismatch")
		assert.NotNil(t, actualContainers[0].Lifecycle.PostStart, "Lifecycle.PostStart should not be nil")
		assert.Equal(t, "HTTP", string(actualContainers[0].Lifecycle.PostStart.HTTPGet.Scheme), "PostStart.HTTPGet.Scheme mismatch")
		assert.Equal(t, "/readiness", actualContainers[0].Lifecycle.PostStart.HTTPGet.Path, "PostStart.HTTPGet.Path mismatch")
		assert.Equal(t, "8080", actualContainers[0].Lifecycle.PostStart.HTTPGet.Port.String(), "PostStart.HTTPGet.Port mismatch")
		assert.Equal(t, "", actualContainers[0].Lifecycle.PostStart.HTTPGet.Host, "PostStart.HTTPGet.Host mismatch")
		assert.Equal(t, 1, len(actualContainers[0].Lifecycle.PostStart.HTTPGet.HTTPHeaders), "PostStart.HTTPGet.HTTPHeaders length mismatch")
	})
}

func TestCompleteBaseWorkload(t *testing.T) {
	r4 := int32(4)
	r3 := int32(3)

	testcases := []struct {
		name          string
		base          *workload.Base
		config        apiv1.GenericConfig
		success       bool
		completedBase *workload.Base
	}{
		{
			name: "successfully complete base",
			base: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
				Labels: map[string]string{
					"k1": "v1",
					"k2": "v2",
				},
			},
			config: apiv1.GenericConfig{
				"labels": apiv1.GenericConfig{
					"k1": "v1-ws",
					"k3": "v3-ws",
				},
				"annotations": apiv1.GenericConfig{
					"k1": "v1-ws",
				},
				"replicas": 4,
			},
			success: true,
			completedBase: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
				Replicas: &r4,
				Labels: map[string]string{
					"k1": "v1",
					"k2": "v2",
					"k3": "v3-ws",
				},
				Annotations: map[string]string{
					"k1": "v1-ws",
				},
			},
		},
		{
			name: "use base replicas",
			base: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
				Replicas: &r3,
				Labels: map[string]string{
					"k1": "v1",
				},
				Annotations: map[string]string{
					"k1": "v1",
				},
			},
			config: apiv1.GenericConfig{
				"replicas": 4,
			},
			success: true,
			completedBase: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
				Replicas: &r3,
				Labels: map[string]string{
					"k1": "v1",
				},
				Annotations: map[string]string{
					"k1": "v1",
				},
			},
		},
		{
			name: "use platform replicas",
			base: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
				Labels: map[string]string{
					"k1": "v1",
				},
				Annotations: map[string]string{
					"k1": "v1",
				},
			},
			config: apiv1.GenericConfig{
				"replicas": 4,
			},
			success: true,
			completedBase: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
				Replicas: &r4,
				Labels: map[string]string{
					"k1": "v1",
				},
				Annotations: map[string]string{
					"k1": "v1",
				},
			},
		},
		{
			name: "invalid replicas config",
			base: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
			},
			config: apiv1.GenericConfig{
				"replicas": "2",
			},
			success:       false,
			completedBase: nil,
		},
		{
			name: "invalid labels config",
			base: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
			},
			config: apiv1.GenericConfig{
				"labels": "k1=v1",
			},
			success:       false,
			completedBase: nil,
		},
		{
			name: "invalid annotations config",
			base: &workload.Base{
				Containers: map[string]container.Container{
					"nginx": {
						Image: "nginx:v1",
					},
				},
			},
			config: apiv1.GenericConfig{
				"annotations": "k1=v1",
			},
			success:       false,
			completedBase: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := completeBaseWorkload(tc.base, tc.config)
			assert.Equal(t, tc.success, err == nil)
			if tc.success {
				assert.True(t, reflect.DeepEqual(tc.base, tc.completedBase))
			}
		})
	}
}
