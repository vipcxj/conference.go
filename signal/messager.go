package signal

import (
	"sync"

	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/proto"
)

type OnStateFunc func(*proto.StateMessage)
type OnWantFunc func(*proto.WantMessage)
type OnSelectFunc func(*proto.SelectMessage)

type TargetMessage interface {
	GetTarget() *proto.Target
}

type Messager struct {
	mu sync.Mutex
	onStateCallbacks map[string]OnStateFunc
	onWantCallbacks map[string]OnWantFunc
	onSelectCallbacks map[string]OnSelectFunc
}

func (m *Messager) OnState(id string, fun OnStateFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateCallbacks[id] = fun
}

func (m *Messager) OffState(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.onStateCallbacks, id)
}

func (m *Messager) OnWant(id string, fun OnWantFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onWantCallbacks[id] = fun
}

func (m *Messager) OffWant(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.onWantCallbacks, id)
}

func (m *Messager) OnSelect(id string, fun OnSelectFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSelectCallbacks[id] = fun
}

func (m *Messager) OffSelect(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.onSelectCallbacks, id)
}

func (m *Messager) Emit(msg TargetMessage) {
	target := msg.GetTarget()
	if target == nil || target.Room == "" {
		panic(errors.InvalidMessage("invalid target message, target is nil or target without room"))
	}
	
}