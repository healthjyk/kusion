package preview

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/mockey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "kusionstack.io/kusion/pkg/apis/core/v1"
	v1 "kusionstack.io/kusion/pkg/apis/status/v1"
	"kusionstack.io/kusion/pkg/cmd/build"
	"kusionstack.io/kusion/pkg/cmd/build/builders"
	"kusionstack.io/kusion/pkg/engine"
	"kusionstack.io/kusion/pkg/engine/operation"
	opsmodels "kusionstack.io/kusion/pkg/engine/operation/models"
	"kusionstack.io/kusion/pkg/engine/runtime"
	"kusionstack.io/kusion/pkg/engine/runtime/kubernetes"
	"kusionstack.io/kusion/pkg/engine/states/local"
	"kusionstack.io/kusion/pkg/project"
)

var (
	apiVersion = "v1"
	kind       = "ServiceAccount"
	namespace  = "test-ns"

	p = &apiv1.Project{
		Name: "testdata",
	}
	s = &apiv1.Stack{
		Name: "dev",
	}

	sa1 = newSA("sa1")
	sa2 = newSA("sa2")
	sa3 = newSA("sa3")
)

func Test_preview(t *testing.T) {
	stateStorage := &local.FileSystemState{Path: filepath.Join("", local.KusionStateFileFile)}
	t.Run("preview success", func(t *testing.T) {
		m := mockOperationPreview()
		defer m.UnPatch()

		o := NewPreviewOptions()
		_, err := Preview(o, stateStorage, &apiv1.Intent{Resources: []apiv1.Resource{sa1, sa2, sa3}}, p, s)
		assert.Nil(t, err)
	})
}

func TestPreviewOptions_Run(t *testing.T) {
	defer func() {
		os.Remove("kusion_state.json")
	}()

	t.Run("no project or stack", func(t *testing.T) {
		o := NewPreviewOptions()
		o.Detail = true
		err := o.Run()
		assert.NotNil(t, err)
	})

	t.Run("compile failed", func(t *testing.T) {
		m := mockDetectProjectAndStack()
		defer m.UnPatch()

		o := NewPreviewOptions()
		o.Detail = true
		err := o.Run()
		assert.NotNil(t, err)
	})

	t.Run("no changes", func(t *testing.T) {
		m1 := mockDetectProjectAndStack()
		m2 := mockPatchBuildIntentWithSpinner()
		m3 := mockNewKubernetesRuntime()
		defer m1.UnPatch()
		defer m2.UnPatch()
		defer m3.UnPatch()

		o := NewPreviewOptions()
		o.Detail = true
		err := o.Run()
		assert.Nil(t, err)
	})

	t.Run("detail is true", func(t *testing.T) {
		m1 := mockDetectProjectAndStack()
		m2 := mockPatchBuildIntentWithSpinner()
		m3 := mockNewKubernetesRuntime()
		m4 := mockOperationPreview()
		m5 := mockPromptDetail("")
		defer m1.UnPatch()
		defer m2.UnPatch()
		defer m3.UnPatch()
		defer m4.UnPatch()
		defer m5.UnPatch()

		o := NewPreviewOptions()
		o.Detail = true
		err := o.Run()
		assert.Nil(t, err)
	})

	t.Run("json output is true", func(t *testing.T) {
		m1 := mockDetectProjectAndStack()
		m2 := mockBuildIntent()
		m3 := mockNewKubernetesRuntime()
		m4 := mockOperationPreview()
		m5 := mockPromptDetail("")
		defer m1.UnPatch()
		defer m2.UnPatch()
		defer m3.UnPatch()
		defer m4.UnPatch()
		defer m5.UnPatch()

		o := NewPreviewOptions()
		o.Output = jsonOutput
		err := o.Run()
		assert.Nil(t, err)
	})

	t.Run("no style is true", func(t *testing.T) {
		m1 := mockDetectProjectAndStack()
		m2 := mockPatchBuildIntentWithSpinner()
		m3 := mockNewKubernetesRuntime()
		m4 := mockOperationPreview()
		m5 := mockPromptDetail("")
		defer m1.UnPatch()
		defer m2.UnPatch()
		defer m3.UnPatch()
		defer m4.UnPatch()
		defer m5.UnPatch()

		o := NewPreviewOptions()
		o.NoStyle = true
		err := o.Run()
		assert.Nil(t, err)
	})
}

type fooRuntime struct{}

func (f *fooRuntime) Import(ctx context.Context, request *runtime.ImportRequest) *runtime.ImportResponse {
	return &runtime.ImportResponse{Resource: request.PlanResource}
}

func (f *fooRuntime) Apply(ctx context.Context, request *runtime.ApplyRequest) *runtime.ApplyResponse {
	return &runtime.ApplyResponse{
		Resource: request.PlanResource,
		Status:   nil,
	}
}

func (f *fooRuntime) Read(ctx context.Context, request *runtime.ReadRequest) *runtime.ReadResponse {
	if request.PlanResource.ResourceKey() == "fake-id" {
		return &runtime.ReadResponse{
			Resource: nil,
			Status:   nil,
		}
	}
	return &runtime.ReadResponse{
		Resource: request.PlanResource,
		Status:   nil,
	}
}

func (f *fooRuntime) Delete(ctx context.Context, request *runtime.DeleteRequest) *runtime.DeleteResponse {
	return nil
}

func (f *fooRuntime) Watch(ctx context.Context, request *runtime.WatchRequest) *runtime.WatchResponse {
	return nil
}

func mockOperationPreview() *mockey.Mocker {
	return mockey.Mock((*operation.PreviewOperation).Preview).To(func(
		*operation.PreviewOperation,
		*operation.PreviewRequest,
	) (rsp *operation.PreviewResponse, s v1.Status) {
		return &operation.PreviewResponse{
			Order: &opsmodels.ChangeOrder{
				StepKeys: []string{sa1.ID, sa2.ID, sa3.ID},
				ChangeSteps: map[string]*opsmodels.ChangeStep{
					sa1.ID: {
						ID:     sa1.ID,
						Action: opsmodels.Create,
						From:   &sa1,
					},
					sa2.ID: {
						ID:     sa2.ID,
						Action: opsmodels.UnChanged,
						From:   &sa2,
					},
					sa3.ID: {
						ID:     sa3.ID,
						Action: opsmodels.Undefined,
						From:   &sa1,
					},
				},
			},
		}, nil
	}).Build()
}

func newSA(name string) apiv1.Resource {
	return apiv1.Resource{
		ID:   engine.BuildID(apiVersion, kind, namespace, name),
		Type: "Kubernetes",
		Attributes: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func mockDetectProjectAndStack() *mockey.Mocker {
	return mockey.Mock(project.DetectProjectAndStack).To(func(stackDir string) (*apiv1.Project, *apiv1.Stack, error) {
		p.Path = stackDir
		s.Path = stackDir
		return p, s, nil
	}).Build()
}

func mockBuildIntent() *mockey.Mocker {
	return mockey.Mock(build.Intent).To(func(
		o *builders.Options,
		project *apiv1.Project,
		stack *apiv1.Stack,
	) (*apiv1.Intent, error) {
		return &apiv1.Intent{Resources: []apiv1.Resource{sa1, sa2, sa3}}, nil
	}).Build()
}

func mockPatchBuildIntentWithSpinner() *mockey.Mocker {
	return mockey.Mock(build.IntentWithSpinner).To(func(
		o *builders.Options,
		project *apiv1.Project,
		stack *apiv1.Stack,
	) (*apiv1.Intent, error) {
		return &apiv1.Intent{Resources: []apiv1.Resource{sa1, sa2, sa3}}, nil
	}).Build()
}

func mockNewKubernetesRuntime() *mockey.Mocker {
	return mockey.Mock(kubernetes.NewKubernetesRuntime).To(func() (runtime.Runtime, error) {
		return &fooRuntime{}, nil
	}).Build()
}

func mockPromptDetail(input string) *mockey.Mocker {
	return mockey.Mock((*opsmodels.ChangeOrder).PromptDetails).To(func(co *opsmodels.ChangeOrder) (string, error) {
		return input, nil
	}).Build()
}

func TestPreviewOptions_ValidateIntentFile(t *testing.T) {
	currDir, _ := os.Getwd()
	tests := []struct {
		name             string
		intentFile       string
		workDir          string
		createIntentFile bool
		wantErr          bool
	}{
		{
			name:             "test1",
			intentFile:       "kusion_intent.yaml",
			workDir:          "",
			createIntentFile: true,
		},
		{
			name:             "test2",
			intentFile:       filepath.Join(currDir, "kusion_intent.yaml"),
			workDir:          "",
			createIntentFile: true,
		},
		{
			name:             "test3",
			intentFile:       "kusion_intent.yaml",
			workDir:          "",
			createIntentFile: false,
			wantErr:          true,
		},
		{
			name:             "test4",
			intentFile:       "ci-test/stdout.golden.yaml",
			workDir:          "",
			createIntentFile: true,
		},
		{
			name:             "test5",
			intentFile:       "../kusion_intent.yaml",
			workDir:          "",
			createIntentFile: true,
			wantErr:          true,
		},
		{
			name:             "test6",
			intentFile:       filepath.Join(currDir, "../kusion_intent.yaml"),
			workDir:          "",
			createIntentFile: true,
			wantErr:          true,
		},
		{
			name:       "test7",
			intentFile: "",
			workDir:    "",
			wantErr:    false,
		},
		{
			name:       "test8",
			intentFile: currDir,
			workDir:    "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := Options{}
			o.IntentFile = tt.intentFile
			o.WorkDir = tt.workDir
			if tt.createIntentFile {
				dir := filepath.Dir(tt.intentFile)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					os.MkdirAll(dir, 0o755)
					defer os.RemoveAll(dir)
				}
				os.Create(tt.intentFile)
				defer os.Remove(tt.intentFile)
			}
			err := o.ValidateIntentFile()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPreviewOptions_Validate(t *testing.T) {
	m := mockey.Mock((*build.Options).Validate).Return(nil).Build()
	defer m.UnPatch()
	tests := []struct {
		name    string
		output  string
		wantErr bool
	}{
		{
			name:    "test1",
			output:  "json",
			wantErr: false,
		},
		{
			name:    "test2",
			output:  "yaml",
			wantErr: true,
		},
		{
			name:    "test3",
			output:  "",
			wantErr: false,
		},
		{
			name:    "test4",
			output:  "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Options{}
			o.Output = tt.output
			err := o.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
