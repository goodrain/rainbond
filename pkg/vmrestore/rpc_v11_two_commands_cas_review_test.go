package vmrestore
import("context";"errors";"testing")
func TestIndependentV11TwoCommandsSameCASMustNotBothApply(t *testing.T){
 f,e,r:=engineFixture(t);r.observation.Halted=true;r.observation.WritersClear=true;initial,_:=e.Status(f.scope,f.expect.OperationID)
 request:=RestoreCommand{ExpectedOperationEpoch:initial.OperationEpoch,ExpectedControlRevision:initial.ControlRevision,ExpectedBindingGeneration:initial.BindingGeneration}
 gate:=&reviewCASPause{Context:context.Background(),reached:make(chan struct{}),release:make(chan struct{})};result:=make(chan RestoreOperationDTO,1);failure:=make(chan error,1)
 go func(){<-gate.reached;d,err:=e.Command(context.Background(),f.scope,f.expect.OperationID,"cancel",request);result<-d;failure<-err;close(gate.release)}()
 halt,err:=e.Command(gate,f.scope,f.expect.OperationID,"halt",request);cancel:=<-result;if ce:=<-failure;ce!=nil||!cancel.Applied{t.Fatal("valid concurrent cancellation control failed",ce)}
 if !errors.Is(err,ErrConflict){t.Fatalf("two commands with same client CAS both applied: cancel=%v halt=%v old_revision=%d final_revision=%d err=%v",cancel.Applied,halt.Applied,initial.ControlRevision,halt.ControlRevision,err)}
}
