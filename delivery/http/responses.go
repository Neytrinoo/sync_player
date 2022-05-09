package http

import "github.com/Neytrinoo/sync_player/domain"

func getNoTrackStateMessage() *domain.UserPlayerUpdateStateMessage {
	return &domain.UserPlayerUpdateStateMessage{TypePushState: domain.NoTrackState}
}

func getTrackStateMessage(trackState *domain.UserPlayerState) *domain.UserPlayerUpdateStateMessage {
	return &domain.UserPlayerUpdateStateMessage{
		TypePushState: domain.PlayNewTrack,
		Data:          *trackState,
	}
}

func getInvalidTrackStateFormatMessage() *domain.UserPlayerUpdateStateMessage {
	return &domain.UserPlayerUpdateStateMessage{
		TypePushState: domain.InvalidTrackStateFormat,
	}
}
