package events

import "uuid"

const UserDeletedEvent = "event:user_deleted"

type UserDeletedEventPayload struct {
	UserId uuid.UUID
}

const UrlClickedEvent = "event:url_clicked"

type UrlClickedEventPayload struct {
	UrlId uuid.UUID
}
