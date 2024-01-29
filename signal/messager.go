package signal

import (
	"context"
	"reflect"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/log"
	"github.com/vipcxj/conference.go/proto"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
	gproto "google.golang.org/protobuf/proto"
)

type OnStateFunc = func(*proto.StateMessage)
type OnWantFunc = func(*proto.WantMessage)
type OnSelectFunc = func(*proto.SelectMessage)

type RoomMessage interface {
	gproto.Message
	GetRouter() *proto.Router
}

type messagerCallbackBox[M RoomMessage] struct {
	id  string
	fun func(M)
}

type OnStateFuncBox = messagerCallbackBox[*proto.StateMessage]
type OnStateFuncBoxes = map[string]*OnStateFuncBox
type OnWantFuncBox = messagerCallbackBox[*proto.WantMessage]
type OnWantFuncBoxes = map[string]*OnWantFuncBox
type OnSelectFuncBox = messagerCallbackBox[*proto.SelectMessage]
type OnSelectFuncBoxes = map[string]*OnSelectFuncBox

type Messager struct {
	nodeName          string
	kafka             *KafkaClient
	onStateMutex      sync.RWMutex
	onStateCallbacks  *PatternMap[OnStateFuncBoxes]
	onWantMutex      sync.RWMutex
	onWantCallbacks   *PatternMap[OnWantFuncBoxes]
	onSelectMutex      sync.RWMutex
	onSelectCallbacks *PatternMap[OnSelectFuncBoxes]
	logger            *zap.Logger
	sugar             *zap.SugaredLogger
}

type GinLike interface {
	Use(middleware ...gin.HandlerFunc) gin.IRoutes
}

func NewMessager() (*Messager, error) {
	clusterConfig := &config.Conf().Cluster
	logger := log.Logger().With(zap.String("tag", "messager"))
	messager := &Messager{
		onStateCallbacks: NewPatternMap[OnStateFuncBoxes](),
		onWantCallbacks: NewPatternMap[OnWantFuncBoxes](),
		onSelectCallbacks: NewPatternMap[OnSelectFuncBoxes](),
		logger: logger,
		sugar: logger.Sugar(),
	}
	if clusterConfig.Enable {
		messager.nodeName = clusterConfig.NodeName
		topicState := MakeKafkaTopic("cluster:state")
		topicWant := MakeKafkaTopic("cluster:want")
		topicSelect := MakeKafkaTopic("cluster:select")
		workers := map[string]func(*kgo.Record) {
			topicState: messager.onTopicState,
			topicWant: messager.onTopicWant,
			topicSelect: messager.onTopicSelect,
		}
		kafka, err := NewKafkaClient(
			KafkaOptGroup(clusterConfig.NodeName),
			KafkaOptTopics(topicState, topicWant, topicSelect),
			KafkaOptWorkers(workers),
		)
		if err != nil {
			return nil, err
		}
		messager.kafka = kafka
	}
	return messager, nil
}

func (m *Messager) Logger() *zap.Logger {
	return m.logger
}

func (m *Messager) Sugar() *zap.SugaredLogger {
	return m.sugar
}

func (m *Messager) Run(ctx context.Context) {
	defer m.kafka.Close()
	m.kafka.Poll(ctx)
}

func (m *Messager) onTopicState(record *kgo.Record) {
	var msg proto.StateMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the state record: %v", record)
		return
	}
	if msg.Router == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.Router.NodeFrom != m.nodeName {
		m.consumeState(&msg)
	}
}

func (m *Messager) onTopicWant(record *kgo.Record) {
	var msg proto.WantMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the want record: %v", record)
		return
	}
	if msg.Router == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.Router.NodeFrom != m.nodeName {
		m.consumeWant(&msg)
	}
}

func (m *Messager) onTopicSelect(record *kgo.Record) {
	var msg proto.SelectMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the select record: %v", record)
		return
	}
	if msg.Router == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.Router.NodeFrom != m.nodeName {
		m.consumeSelect(&msg)
	}
}

func onMessage[M RoomMessage](
	pm *PatternMap[map[string]*messagerCallbackBox[M]],
	funId string, fun func(M),
	roomPattern ...string,
) {
	box := &messagerCallbackBox[M]{
		id:  funId,
		fun: fun,
	}
	for _, rp := range roomPattern {
		pm.UpdatePatternData(rp, func(old map[string]*messagerCallbackBox[M], found bool) (new map[string]*messagerCallbackBox[M], remove bool) {
			if !found {
				old = make(map[string]*messagerCallbackBox[M])
			}
			old[funId] = box
			return old, false
		})
	}
}

func offMessage[M RoomMessage](
	pm *PatternMap[map[string]*messagerCallbackBox[M]],
	funId string,
	roomPattern ...string,
) {
	for _, rp := range roomPattern {
		pm.UpdatePatternData(rp, func(old map[string]*messagerCallbackBox[M], found bool) (new map[string]*messagerCallbackBox[M], remove bool) {
			if found {
				delete(old, funId)
			}
			return old, false
		})
	}
}

func consumeMessage[M RoomMessage](
	pm *PatternMap[map[string]*messagerCallbackBox[M]],
	msg M,
	sugar *zap.SugaredLogger,
	nodeName string,
) []func(M) {
	target := msg.GetRouter()
	if target == nil || target.Room == "" {
		sugar.Errorf("accept a message without router or room: %v", msg)
		return nil
	}
	if target.NodeTo != "" && target.NodeTo != nodeName {
		return nil
	}
	bm := make(map[string]func(M))
	for _, boxes := range pm.Collect(target.Room) {
		for key, box := range boxes {
			bm[key] = box.fun
		}
	}
	return utils.MapValues(bm)
}

func (m *Messager) OnState(funId string, fun OnStateFunc, roomPattern ...string) {
	m.onStateMutex.Lock()
	defer m.onStateMutex.Unlock()
	onMessage(m.onStateCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffState(funId string, roomPattern ...string) {
	m.onStateMutex.Lock()
	defer m.onStateMutex.Unlock()
	offMessage(m.onStateCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeState(msg *proto.StateMessage) {
	funs := func () []OnStateFunc {
		m.onStateMutex.RLock()
		defer m.onStateMutex.RUnlock()
		return consumeMessage(m.onStateCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) OnWant(funId string, fun OnWantFunc, roomPattern ...string) {
	m.onWantMutex.Lock()
	defer m.onWantMutex.Unlock()
	onMessage(m.onWantCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffWant(funId string, roomPattern ...string) {
	m.onWantMutex.Lock()
	defer m.onWantMutex.Unlock()
	offMessage(m.onWantCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeWant(msg *proto.WantMessage) {
	funs := func () []OnWantFunc {
		m.onWantMutex.RLock()
		defer m.onWantMutex.RUnlock()
		return consumeMessage(m.onWantCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) OnSelect(funId string, fun OnSelectFunc, roomPattern ...string) {
	m.onSelectMutex.Lock()
	defer m.onSelectMutex.Unlock()
	onMessage(m.onSelectCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffSelect(funId string, roomPattern ...string) {
	m.onSelectMutex.Lock()
	defer m.onSelectMutex.Unlock()
	offMessage(m.onSelectCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeSelect(msg *proto.SelectMessage) {
	funs := func () []OnSelectFunc {
		m.onSelectMutex.RLock()
		defer m.onSelectMutex.RUnlock()
		return consumeMessage(m.onSelectCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) logEmitMsg(msg RoomMessage, msgType string) {
	router := msg.GetRouter()
	if router.GetNodeTo() != "" {
		if router.GetUserFrom() != "" {
			if router.GetUserTo() != "" {
				m.Sugar().Debugf(
					"send %s msg from user %s in node %s to use %s in room %s of node %s, msg: %v",
					msgType,
					router.GetUserFrom(), router.GetNodeFrom(),
					router.GetUserTo(), router.GetRoom(), router.GetNodeTo(),
					msg,
				)
			} else {
				m.Sugar().Debugf(
					"send %s msg from user %s in node %s to room %s of node %s, msg: %v",
					msgType,
					router.GetUserFrom(), router.GetNodeFrom(),
					router.GetRoom(), router.GetNodeTo(),
					msg,
				)
			}
		} else {
			if router.GetUserTo() != "" {
				m.Sugar().Debugf(
					"send %s msg from node %s to use %s in room %s of node %s, msg: %v",
					msgType,
					router.GetNodeFrom(),
					router.GetUserTo(), router.GetRoom(), router.GetNodeTo(),
					msg,
				)
			} else {
				m.Sugar().Debugf(
					"send %s msg from node %s to room %s of node %s, msg: %v",
					msgType,
					router.GetNodeFrom(),
					router.GetRoom(), router.GetNodeTo(),
					msg,
				)
			}
		}
	} else {
		if router.GetUserFrom() != "" {
			if router.GetUserTo() != "" {
				m.Sugar().Debugf(
					"send %s msg from user %s in node %s to use %s in room %s, msg: %v",
					msgType,
					router.GetUserFrom(), router.GetNodeFrom(),
					router.GetUserTo(), router.GetRoom(),
					msg,
				)
			} else {
				m.Sugar().Debugf(
					"send %s msg from user %s in node %s to room %s, msg: %v",
					msgType,
					router.GetUserFrom(), router.GetNodeFrom(),
					router.GetRoom(),
					msg,
				)
			}
		} else {
			if router.GetUserTo() != "" {
				m.Sugar().Debugf(
					"send %s msg from node %s to use %s in room %s, msg: %v",
					msgType,
					router.GetNodeFrom(),
					router.GetUserTo(), router.GetRoom(),
					msg,
				)
			} else {
				m.Sugar().Debugf(
					"send %s msg from node %s to room %s, msg: %v",
					msgType,
					router.GetNodeFrom(),
					router.GetRoom(),
					msg,
				)
			}
		}
	}
}

func (m *Messager) Emit(ctx context.Context, msg RoomMessage) error {
	target := msg.GetRouter()
	if target == nil || target.Room == "" {
		return errors.InvalidMessage("invalid room message, router is nil or router without room")
	}
	if target.NodeFrom != "" && target.NodeFrom != m.nodeName {
		return errors.InvalidMessage("invalid room message, nodeFrom %s different from current node %s", target.NodeFrom, m.nodeName)
	} else if target.NodeFrom == "" {
		target.NodeFrom = m.nodeName
	}
	switch typedMsg := msg.(type) {
	case *StateMessage:
		m.logEmitMsg(msg, "state")
		m.consumeState(typedMsg)
	case *WantMessage:
		m.logEmitMsg(msg, "want")
		m.consumeWant(typedMsg)
	case *SelectMessage:
		m.logEmitMsg(msg, "select")
		m.consumeSelect(typedMsg)
	default:
		return errors.InvalidMessage("invalid room message, unknown message type %v", reflect.TypeOf(msg))
	}
	data, err := gproto.Marshal(msg)
	if err != nil {
		return errors.InvalidMessage("unable to marshal room message, %v", err)
	}
	return m.kafka.Produce(ctx, &kgo.Record{
		Value: data,
	})
}
