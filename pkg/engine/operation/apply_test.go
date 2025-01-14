package operation

import (
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/bytedance/mockey"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"

	apiv1 "kusionstack.io/kusion/pkg/apis/core/v1"
	v1 "kusionstack.io/kusion/pkg/apis/status/v1"
	"kusionstack.io/kusion/pkg/engine/operation/graph"
	opsmodels "kusionstack.io/kusion/pkg/engine/operation/models"
	"kusionstack.io/kusion/pkg/engine/runtime"
	runtimeinit "kusionstack.io/kusion/pkg/engine/runtime/init"
	"kusionstack.io/kusion/pkg/engine/runtime/kubernetes"
	"kusionstack.io/kusion/pkg/engine/states"
	"kusionstack.io/kusion/pkg/engine/states/local"
)

func Test_validateRequest(t *testing.T) {
	type args struct {
		request *opsmodels.Request
	}
	tests := []struct {
		name string
		args args
		want v1.Status
	}{
		{
			name: "t1",
			args: args{
				request: &opsmodels.Request{},
			},
			want: v1.NewErrorStatusWithMsg(v1.InvalidArgument,
				"request.Intent is empty. If you want to delete all resources, please use command 'destroy'"),
		},
		{
			name: "t2",
			args: args{
				request: &opsmodels.Request{
					Intent: &apiv1.Intent{Resources: []apiv1.Resource{}},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateRequest(tt.args.request); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperation_Apply(t *testing.T) {
	type fields struct {
		OperationType           opsmodels.OperationType
		StateStorage            states.StateStorage
		CtxResourceIndex        map[string]*apiv1.Resource
		PriorStateResourceIndex map[string]*apiv1.Resource
		StateResourceIndex      map[string]*apiv1.Resource
		Order                   *opsmodels.ChangeOrder
		RuntimeMap              map[apiv1.Type]runtime.Runtime
		Stack                   *apiv1.Stack
		MsgCh                   chan opsmodels.Message
		resultState             *states.State
		lock                    *sync.Mutex
	}
	type args struct {
		applyRequest *ApplyRequest
	}

	const Jack = "jack"
	mf := &apiv1.Intent{Resources: []apiv1.Resource{
		{
			ID:   Jack,
			Type: runtime.Kubernetes,
			Attributes: map[string]interface{}{
				"a": "b",
			},
			DependsOn: nil,
		},
	}}

	rs := &states.State{
		ID:            0,
		Tenant:        "fakeTenant",
		Stack:         "fakeStack",
		Project:       "fakeProject",
		Version:       0,
		KusionVersion: "",
		Serial:        1,
		Operator:      "faker",
		Resources: []apiv1.Resource{
			{
				ID:   Jack,
				Type: runtime.Kubernetes,
				Attributes: map[string]interface{}{
					"a": "b",
				},
				DependsOn: nil,
			},
		},
	}

	s := &apiv1.Stack{
		Name: "fakeStack",
		Path: "fakePath",
	}
	p := &apiv1.Project{
		Name:   "fakeProject",
		Path:   "fakePath",
		Stacks: []*apiv1.Stack{s},
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantRsp *ApplyResponse
		wantSt  v1.Status
	}{
		{
			name: "apply test",
			fields: fields{
				OperationType: opsmodels.Apply,
				StateStorage:  &local.FileSystemState{Path: filepath.Join("test_data", local.KusionStateFileFile)},
				RuntimeMap:    map[apiv1.Type]runtime.Runtime{runtime.Kubernetes: &kubernetes.KubernetesRuntime{}},
				MsgCh:         make(chan opsmodels.Message, 5),
			},
			args: args{applyRequest: &ApplyRequest{opsmodels.Request{
				Tenant:   "fakeTenant",
				Stack:    s,
				Project:  p,
				Operator: "faker",
				Intent:   mf,
			}}},
			wantRsp: &ApplyResponse{rs},
			wantSt:  nil,
		},
	}

	for _, tt := range tests {
		mockey.PatchConvey(tt.name, t, func() {
			o := &opsmodels.Operation{
				OperationType:           tt.fields.OperationType,
				StateStorage:            tt.fields.StateStorage,
				CtxResourceIndex:        tt.fields.CtxResourceIndex,
				PriorStateResourceIndex: tt.fields.PriorStateResourceIndex,
				StateResourceIndex:      tt.fields.StateResourceIndex,
				ChangeOrder:             tt.fields.Order,
				RuntimeMap:              tt.fields.RuntimeMap,
				Stack:                   tt.fields.Stack,
				MsgCh:                   tt.fields.MsgCh,
				ResultState:             tt.fields.resultState,
				Lock:                    tt.fields.lock,
			}
			ao := &ApplyOperation{
				Operation: *o,
			}

			mockey.Mock((*graph.ResourceNode).Execute).To(func(rn *graph.ResourceNode, operation *opsmodels.Operation) v1.Status {
				o.ResultState = rs
				return nil
			}).Build()
			mockey.Mock(runtimeinit.Runtimes).To(func(
				resources apiv1.Resources,
			) (map[apiv1.Type]runtime.Runtime, v1.Status) {
				return map[apiv1.Type]runtime.Runtime{runtime.Kubernetes: &kubernetes.KubernetesRuntime{}}, nil
			}).Build()

			gotRsp, gotSt := ao.Apply(tt.args.applyRequest)
			assert.Equalf(t, tt.wantRsp.State.Stack, gotRsp.State.Stack, "Apply(%v)", tt.args.applyRequest)
			assert.Equalf(t, tt.wantSt, gotSt, "Apply(%v)", tt.args.applyRequest)
		})
	}
}
