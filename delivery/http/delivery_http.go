package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Neytrinoo/sync_player/domain"
	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"strconv"
)

type Handler struct {
	userSyncPlayerUseCase domain.UserSyncPlayerUseCase
	redisAddr             string
}

func NewHandler(usecase domain.UserSyncPlayerUseCase, redisAddr string) *Handler {
	return &Handler{
		userSyncPlayerUseCase: usecase,
		redisAddr:             redisAddr,
	}
}

func getUserId() uint { // заглушка. потом id user'а будет получаться из микросервиса авторизации
	return 2
}

func getRedisChannelName(userId uint) string {
	return strconv.Itoa(int(userId))
}

func (a *Handler) initRedisStructs(userId uint) (redisCon redis.Conn, redisPubSub *redis.PubSubConn, err error) {
	redisCon, err = redis.Dial("tcp", a.redisAddr)
	if err != nil {
		return
	}

	redisPubSub = &redis.PubSubConn{Conn: redisCon}
	err = redisPubSub.Subscribe(getRedisChannelName(userId))

	return
}

func (a *Handler) pushToRedisChannel(redisCon redis.Conn, channelName string, message string) {
	redisCon.Do("PUBLISH", channelName, message)
}

func (a *Handler) readRedisChannelLoop(redisChannel *redis.PubSubConn, wsCon *websocket.Conn) {
	defer fmt.Println("end redis channel loop")
	for {
		switch v := redisChannel.Receive().(type) {
		case redis.Message:
			if wsCon.WriteMessage(websocket.TextMessage, v.Data) != nil {
				return
			}
		}
	}
}

func (a *Handler) updateStateMessageProcessing(userId uint, message *domain.UserPlayerUpdateStateMessage) error {
	switch message.TypePushState {
	case domain.PlayNewTrack:
		return a.userSyncPlayerUseCase.NewTrackUpdateState(userId, message.Data.TrackId, message.Data.FromIs, message.Data.FromIsId, message.Data.TimeStateUpdate)
	case domain.OnPause:
		return a.userSyncPlayerUseCase.OnPauseUpdateState(userId, message.Data.TimeStateUpdate)
	case domain.OffPause:
		return a.userSyncPlayerUseCase.OffPauseUpdateState(userId, message.Data.TimeStateUpdate)
	case domain.ChangePosition:
		return a.userSyncPlayerUseCase.ChangePositionUpdateState(userId, message.Data.LastSecPosition, message.Data.TimeStateUpdate)
	}

	return errors.New("no such push state type")
}

func (a *Handler) PlayerStateLoop(c echo.Context) error {
	var upgrader = websocket.Upgrader{}
	userId := getUserId()

	wsCon, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer wsCon.Close()

	// первоначально мы либо получаем текущее состояние плеера и отсылаем пользователю, либо отсылаем сообщение
	// об отсутствии состояния. в таком случае мы ожидаем, что придет сообщение с обновлением состояния
	trackState, err := a.userSyncPlayerUseCase.GetTrackState(userId)
	var messageState []byte
	if err != nil {
		messageState, _ = json.Marshal(getNoTrackStateMessage())
	} else {
		messageState, _ = json.Marshal(getTrackStateMessage(trackState))
	}

	err = wsCon.WriteMessage(websocket.TextMessage, messageState)
	if err != nil {
		return err
	}

	redisCon, redisPubSub, err := a.initRedisStructs(userId)
	if err != nil {
		return err
	}
	defer redisCon.Close()
	defer redisPubSub.Close()

	// запускаем бесконечный цикл, в котором будут читаться сообщения из redis channel'а и отправляться клиенту
	go a.readRedisChannelLoop(redisPubSub, wsCon)

	var clientMessage domain.UserPlayerUpdateStateMessage
	redisChannelName := getRedisChannelName(userId)
	for {
		_, message, err := wsCon.ReadMessage()

		if err != nil {
			break
		}

		err = json.Unmarshal(message, &clientMessage)
		if err != nil {
			messageState, _ = json.Marshal(getInvalidTrackStateFormatMessage())
			if wsCon.WriteMessage(websocket.TextMessage, messageState) != nil {
				break
			}
		} else {
			// обновляем состояние плеера
			err = a.updateStateMessageProcessing(userId, &clientMessage)
			if err == domain.ErrGetUserPlayerState {
				messageState, _ = json.Marshal(getNoTrackStateMessage())
				if wsCon.WriteMessage(websocket.TextMessage, messageState) != nil {
					break
				}
			} else if err != nil {
				messageState, _ = json.Marshal(getInvalidTrackStateFormatMessage())
				if wsCon.WriteMessage(websocket.TextMessage, messageState) != nil {
					break
				}
			} else {
				// публикуем обновление состояния плеера в redis channel. его считают другие клиенты
				a.pushToRedisChannel(redisCon, redisChannelName, string(messageState))
			}
		}
	}

	return nil
}
