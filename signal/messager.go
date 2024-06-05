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
	"github.com/vipcxj/conference.go/model"
	"github.com/vipcxj/conference.go/utils"
	"go.uber.org/zap"
	gproto "google.golang.org/protobuf/proto"
)

type OnStateFunc = func(*model.StateMessage)
type OnWantFunc = func(*model.WantMessage)
type OnSelectFunc = func(*model.SelectMessage)
type OnWantParticipantFunc = func(*model.WantParticipantMessage)
type OnStateParticipantFunc = func(*model.StateParticipantMessage)
type OnCustomFunc = func(*model.CustomClusterMessage)
type OnCustomAckFunc = func(*model.CustomAckMessage)

type messagerCallbackBox[M model.RoomMessage] struct {
	id  string
	fun func(M)
}

type OnStateFuncBox = messagerCallbackBox[*model.StateMessage]
type OnStateFuncBoxes = map[string]*OnStateFuncBox
type OnWantFuncBox = messagerCallbackBox[*model.WantMessage]
type OnWantFuncBoxes = map[string]*OnWantFuncBox
type OnSelectFuncBox = messagerCallbackBox[*model.SelectMessage]
type OnSelectFuncBoxes = map[string]*OnSelectFuncBox
type OnWantParticipantFuncBox = messagerCallbackBox[*model.WantParticipantMessage]
type OnWantParticipantFuncBoxes = map[string]*OnWantParticipantFuncBox
type OnStateParticipantFuncBox = messagerCallbackBox[*model.StateParticipantMessage]
type OnStateParticipantFuncBoxes = map[string]*OnStateParticipantFuncBox
type OnCustomFuncBox = messagerCallbackBox[*model.CustomClusterMessage]
type OnCustomFuncBoxes = map[string]*OnCustomFuncBox
type OnCustomAckFuncBox = messagerCallbackBox[*model.CustomAckMessage]
type OnCustomAckFuncBoxes = map[string]*OnCustomAckFuncBox

type Messager struct {
	global                      *Global
	nodeName                    string
	kafka                       *KafkaClient
	onStateMutex                sync.RWMutex
	onStateCallbacks            *PatternMap[OnStateFuncBoxes]
	onWantMutex                 sync.RWMutex
	onWantCallbacks             *PatternMap[OnWantFuncBoxes]
	onSelectMutex               sync.RWMutex
	onSelectCallbacks           *PatternMap[OnSelectFuncBoxes]
	onWantParticipantMutex      sync.RWMutex
	onWantParticipantCallbacks  *PatternMap[OnWantParticipantFuncBoxes]
	onStateParticipantMutex     sync.RWMutex
	onStateParticipantCallbacks *PatternMap[OnStateParticipantFuncBoxes]
	onCustomMutex               sync.RWMutex
	onCustomCallbacks           *PatternMap[OnCustomFuncBoxes]
	onCustomAckMutex            sync.RWMutex
	onCustomAckCallbacks        *PatternMap[OnCustomAckFuncBoxes]
	logger                      *zap.Logger
	sugar                       *zap.SugaredLogger
}

type GinLike interface {
	Use(middleware ...gin.HandlerFunc) gin.IRoutes
}

const (
	TOPIC_STATE             = "cluster_state"
	TOPIC_WANT              = "cluster_want"
	TOPIC_SELECT            = "cluster_select"
	TOPIC_WANT_PARTICIPANT  = "cluster_want_participant"
	TOPIC_STATE_PARTICIPANT = "cluster_state_participant"
	TOPIC_CUSTOM            = "cluster_custom"
	TOPIC_CUSTOM_ACK        = "cluster_custom_ack"
)

func NewMessager(global *Global) (*Messager, error) {
	clusterConfig := &global.Conf().Cluster
	logger := log.Logger().With(zap.String("tag", "messager"))
	messager := &Messager{
		global:                      global,
		onStateCallbacks:            NewPatternMap[OnStateFuncBoxes](),
		onWantCallbacks:             NewPatternMap[OnWantFuncBoxes](),
		onSelectCallbacks:           NewPatternMap[OnSelectFuncBoxes](),
		onWantParticipantCallbacks:  NewPatternMap[OnWantParticipantFuncBoxes](),
		onStateParticipantCallbacks: NewPatternMap[OnStateParticipantFuncBoxes](),
		onCustomCallbacks:           NewPatternMap[OnCustomFuncBoxes](),
		onCustomAckCallbacks:        NewPatternMap[OnCustomAckFuncBoxes](),
		logger:                      logger,
		sugar:                       logger.Sugar(),
	}
	if clusterConfig.Enable {
		messager.nodeName = clusterConfig.GetNodeName()
		topicState := MakeKafkaTopic(global.Conf(), TOPIC_STATE)
		topicWant := MakeKafkaTopic(global.Conf(), TOPIC_WANT)
		topicSelect := MakeKafkaTopic(global.Conf(), TOPIC_SELECT)
		topicWantParticipant := MakeKafkaTopic(global.Conf(), TOPIC_WANT_PARTICIPANT)
		topicStateParticipant := MakeKafkaTopic(global.Conf(), TOPIC_STATE_PARTICIPANT)
		topicCustom := MakeKafkaTopic(global.Conf(), TOPIC_CUSTOM)
		topicCustomAck := MakeKafkaTopic(global.Conf(), TOPIC_CUSTOM_ACK)
		workers := map[string]func(*kgo.Record){
			topicState:            messager.onTopicState,
			topicWant:             messager.onTopicWant,
			topicSelect:           messager.onTopicSelect,
			topicWantParticipant:  messager.onTopicWantParticipant,
			topicStateParticipant: messager.onTopicStateParticipant,
			topicCustom:           messager.onTopicCustom,
			topicCustomAck:        messager.onTopicCustomAck,
		}
		kafka, err := NewKafkaClient(
			global,
			KafkaOptGroup(clusterConfig.GetNodeName()),
			KafkaOptTopics(topicState, topicWant, topicSelect, topicWantParticipant, topicStateParticipant, topicCustom, topicCustomAck),
			KafkaOptWorkers(workers),
		)
		if err != nil {
			return nil, errors.FatalError("unable to create messager, %v", err)
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

func (m *Messager) Conf() *config.ConferenceConfigure {
	return m.global.Conf()
}

func (m *Messager) NodeName() string {
	return m.nodeName
}

func (m *Messager) Run(ctx context.Context) {
	defer m.kafka.Close()
	m.kafka.Poll(ctx)
}

func (m *Messager) onTopicState(record *kgo.Record) {
	var msg model.StateMessage
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
	var msg model.WantMessage
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
	var msg model.SelectMessage
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

func (m *Messager) onTopicWantParticipant(record *kgo.Record) {
	var msg model.WantParticipantMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the select record: %v", record)
		return
	}
	if msg.Router == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.Router.NodeFrom != m.nodeName {
		m.consumeWantParticipant(&msg)
	}
}

func (m *Messager) onTopicStateParticipant(record *kgo.Record) {
	var msg model.StateParticipantMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the select record: %v", record)
		return
	}
	if msg.Router == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.Router.NodeFrom != m.nodeName {
		m.consumeStateParticipant(&msg)
	}
}

func (m *Messager) onTopicCustom(record *kgo.Record) {
	var msg model.CustomClusterMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the custom record: %v", record)
		return
	}
	if msg.GetRouter() == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.GetRouter().NodeFrom != m.nodeName {
		m.consumeCustom(&msg)
	}
}

func (m *Messager) onTopicCustomAck(record *kgo.Record) {
	var msg model.CustomAckMessage
	err := gproto.Unmarshal(record.Value, &msg)
	if err != nil {
		m.Sugar().Errorf("unable to unmarshal the user ack record: %v", record)
		return
	}
	if msg.Router == nil {
		m.Sugar().Errorf("accept a message without router")
	}
	if msg.Router.NodeFrom != m.nodeName {
		m.consumeCustomAck(&msg)
	}
}

func onMessage[M model.RoomMessage](
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

func offMessage[M model.RoomMessage](
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

func consumeMessage[M model.RoomMessage](
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

func (m *Messager) consumeState(msg *model.StateMessage) {
	funs := func() []OnStateFunc {
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

func (m *Messager) consumeWant(msg *model.WantMessage) {
	funs := func() []OnWantFunc {
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

func (m *Messager) consumeSelect(msg *model.SelectMessage) {
	funs := func() []OnSelectFunc {
		m.onSelectMutex.RLock()
		defer m.onSelectMutex.RUnlock()
		return consumeMessage(m.onSelectCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) OnWantParticipant(funId string, fun OnWantParticipantFunc, roomPattern ...string) {
	m.onWantParticipantMutex.Lock()
	defer m.onWantParticipantMutex.Unlock()
	onMessage(m.onWantParticipantCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffWantParticipant(funId string, roomPattern ...string) {
	m.onWantParticipantMutex.Lock()
	defer m.onWantParticipantMutex.Unlock()
	offMessage(m.onWantParticipantCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeWantParticipant(msg *model.WantParticipantMessage) {
	funs := func() []OnWantParticipantFunc {
		m.onWantParticipantMutex.RLock()
		defer m.onWantParticipantMutex.RUnlock()
		return consumeMessage(m.onWantParticipantCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) OnStateParticipant(funId string, fun OnStateParticipantFunc, roomPattern ...string) {
	m.onStateParticipantMutex.Lock()
	defer m.onStateParticipantMutex.Unlock()
	onMessage(m.onStateParticipantCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffStateParticipant(funId string, roomPattern ...string) {
	m.onStateParticipantMutex.Lock()
	defer m.onStateParticipantMutex.Unlock()
	offMessage(m.onStateParticipantCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeStateParticipant(msg *model.StateParticipantMessage) {
	funs := func() []OnStateParticipantFunc {
		m.onStateParticipantMutex.RLock()
		defer m.onStateParticipantMutex.RUnlock()
		return consumeMessage(m.onStateParticipantCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) OnCustom(funId string, fun OnCustomFunc, roomPattern ...string) {
	m.onCustomMutex.Lock()
	defer m.onCustomMutex.Unlock()
	onMessage(m.onCustomCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffCustom(funId string, roomPattern ...string) {
	m.onCustomMutex.Lock()
	defer m.onCustomMutex.Unlock()
	offMessage(m.onCustomCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeCustom(msg *model.CustomClusterMessage) {
	funs := func() []OnCustomFunc {
		m.onCustomMutex.RLock()
		defer m.onCustomMutex.RUnlock()
		return consumeMessage(m.onCustomCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) OnCustomAck(funId string, fun OnCustomAckFunc, roomPattern ...string) {
	m.onCustomAckMutex.Lock()
	defer m.onCustomAckMutex.Unlock()
	onMessage(m.onCustomAckCallbacks, funId, fun, roomPattern...)
}

func (m *Messager) OffCustomAck(funId string, roomPattern ...string) {
	m.onCustomAckMutex.Lock()
	defer m.onCustomAckMutex.Unlock()
	offMessage(m.onCustomAckCallbacks, funId, roomPattern...)
}

func (m *Messager) consumeCustomAck(msg *model.CustomAckMessage) {
	funs := func() []OnCustomAckFunc {
		m.onCustomAckMutex.RLock()
		defer m.onCustomAckMutex.RUnlock()
		return consumeMessage(m.onCustomAckCallbacks, msg, m.sugar, m.nodeName)
	}()
	for _, fun := range funs {
		go fun(msg)
	}
}

func (m *Messager) logEmitMsg(msg model.RoomMessage, msgType string) {
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

func (m *Messager) Emit(ctx context.Context, msg model.RoomMessage) error {
	target := msg.GetRouter()
	if target == nil || target.Room == "" {
		return errors.InvalidMessage("invalid room message, router is nil or router without room")
	}
	if target.NodeFrom != "" && target.NodeFrom != m.nodeName {
		return errors.InvalidMessage("invalid room message, nodeFrom %s different from current node %s", target.NodeFrom, m.nodeName)
	} else if target.NodeFrom == "" {
		target.NodeFrom = m.nodeName
	}
	var topic string
	key := msg.GetRouter().Room
	switch typedMsg := msg.(type) {
	case *model.StateMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_STATE)
		m.logEmitMsg(msg, "state")
		m.consumeState(typedMsg)
	case *model.WantMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_WANT)
		m.logEmitMsg(msg, "want")
		m.consumeWant(typedMsg)
	case *model.SelectMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_SELECT)
		m.logEmitMsg(msg, "select")
		m.consumeSelect(typedMsg)
	case *model.WantParticipantMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_WANT_PARTICIPANT)
		m.logEmitMsg(msg, "want-participant")
		m.consumeWantParticipant(typedMsg)
	case *model.StateParticipantMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_STATE_PARTICIPANT)
		m.logEmitMsg(msg, "state-participant")
		m.consumeStateParticipant(typedMsg)
	case *model.CustomClusterMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_CUSTOM)
		m.logEmitMsg(msg, "custom")
		m.consumeCustom(typedMsg)
	case *model.CustomAckMessage:
		topic = MakeKafkaTopic(m.Conf(), TOPIC_CUSTOM_ACK)
		m.logEmitMsg(msg, "custom-ack")
		m.consumeCustomAck(typedMsg)
	default:
		return errors.InvalidMessage("invalid room message, unknown message type %v", reflect.TypeOf(msg))
	}
	data, err := gproto.Marshal(msg)
	if err != nil {
		return errors.InvalidMessage("unable to marshal room message, %v", err)
	}
	return m.kafka.Produce(ctx, &kgo.Record{
		Key:   []byte(key),
		Topic: topic,
		Value: data,
	})
}
